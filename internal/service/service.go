package service

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sony/gobreaker"

	"github.com/pietroagazzi/gater/internal/circuitbreaker"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/loadbalancer"
)

// ValidatingTransport validates that the Director properly configured the request
type ValidatingTransport struct {
	underlying http.RoundTripper
}

// RoundTrip validates the request before forwarding to the underlying transport
func (t *ValidatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Validate that the Director configured the request properly
	if req.URL.Host == "" || req.URL.Scheme == "" {
		return nil, fmt.Errorf("request not properly configured by director")
	}
	return t.underlying.RoundTrip(req)
}

// Service represents a backend service with all its operational policies
type Service struct {
	Name           string
	Config         *config.ServiceConfig
	Instances      []*Instance
	Validator      *Validator
	loadBalancer   *loadbalancer.RoundRobin
	circuitBreaker *gobreaker.CircuitBreaker
	reverseProxy   *httputil.ReverseProxy
	transport      http.RoundTripper
}

// NewService creates a Service with all infrastructure components
func NewService(name string, instanceURLs []string, cfg *config.ServiceConfig) (*Service, error) {
	if name == "" {
		return nil, fmt.Errorf("service name is required")
	}

	if len(instanceURLs) == 0 {
		return nil, fmt.Errorf("service %s: no instances provided", name)
	}

	if cfg == nil {
		return nil, fmt.Errorf("service %s: configuration is required", name)
	}

	// 1. Parse instances
	instances := make([]*Instance, 0, len(instanceURLs))
	targetURLs := make([]string, 0, len(instanceURLs))

	for _, rawURL := range instanceURLs {
		instance, err := NewInstance(rawURL)
		if err != nil {
			log.Printf("WARNING: Failed to parse instance URL %s for service %s: %v", rawURL, name, err)
			continue
		}
		instances = append(instances, instance)
		targetURLs = append(targetURLs, rawURL)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("service %s: no valid instances after parsing", name)
	}

	// 2. Create load balancer
	lb := loadbalancer.NewRoundRobin(targetURLs)

	// 3. Create circuit breaker settings from ServiceConfig
	cbSettings := circuitbreaker.Settings{
		// Use a reasonable default for MaxRequests (requests allowed in half-open state)
		MaxRequests: 1,
		// Interval from config (default to 10 seconds if not set)
		Interval: 10 * time.Second,
		// Timeout from config (recovery_timeout)
		Timeout: cfg.Resilience.CircuitBreaker.RecoveryTimeout,
	}

	// 4. Create circuit breaker with service-specific name
	circuitBreaker := circuitbreaker.NewCircuitBreaker(name+"-cb", cbSettings)

	// 5. Create custom transport with circuit breaker
	circuitBreakerTransport := circuitbreaker.NewTransport(circuitBreaker, http.DefaultTransport)

	// 6. Wrap with validating transport to catch Director errors
	transport := &ValidatingTransport{underlying: circuitBreakerTransport}

	// 7. Create single reverse proxy with dynamic director
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Load balancer selects target for THIS request
			targetURL := lb.Next()
			if targetURL == "" {
				log.Printf("ERROR: No target available for service %s", name)
				// Leave request unconfigured - ValidatingTransport will catch this
				return
			}

			target, err := url.Parse(targetURL)
			if err != nil {
				log.Printf("ERROR: Failed to parse target URL %s: %v", targetURL, err)
				// Leave request unconfigured - ValidatingTransport will catch this
				return
			}

			// Update request to point to selected backend
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.Header.Add("Forwarded", FormatForHeader(req))

		},
		Transport: transport,
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			// Handle circuit breaker open state
			if err == gobreaker.ErrOpenState {
				log.Printf("Circuit breaker open for service %s: %s", name, req.URL.String())
				rw.WriteHeader(http.StatusServiceUnavailable)
				_, _ = rw.Write([]byte("Service unavailable due to high failure rate"))
				return
			}

			// Handle Director configuration errors
			if err != nil && err.Error() == "request not properly configured by director" {
				log.Printf("Director failed to configure request for service %s: %s", name, req.URL.String())
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = rw.Write([]byte("Internal server error: failed to configure request"))
				return
			}

			// Handle other errors
			log.Printf("Reverse proxy error for service %s: %s: %v", name, req.URL.String(), err)
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = rw.Write([]byte("Bad gateway"))
		},
	}

	// 8. Create Validator
	validator := NewValidator(cfg.Validator)

	return &Service{
		Name:           name,
		Config:         cfg,
		Instances:      instances,
		Validator:      validator,
		loadBalancer:   lb,
		circuitBreaker: circuitBreaker,
		reverseProxy:   proxy,
		transport:      transport,
	}, nil
}

// Generates a  RFC 7239 compliant Forwarded header from request parameters
func FormatForHeader(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)

	if err != nil {
		host = req.RemoteAddr
	}

	if strings.Contains(host, ":") {
		host = `"[` + host + `]"`
	}

	return "for=" + host + ";proto=" + req.URL.Scheme + ";by=gater"
}

// ServeHTTP handles an HTTP request by proxying to the service
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Validator.Handle(w, r) {
		s.reverseProxy.ServeHTTP(w, r)
	}
}

// GetInstances returns current service instances (for health/monitoring)
func (s *Service) GetInstances() []*Instance {
	return s.Instances
}
