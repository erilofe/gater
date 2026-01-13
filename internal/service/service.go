package service

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/pietroagazzi/gater/internal/circuitbreaker"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/loadbalancer"
	"github.com/sony/gobreaker"
)

// Service represents a backend service with all its operational policies
type Service struct {
	Name           string
	Config         *config.ServiceConfig
	Instances      []*ServiceInstance
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
	instances := make([]*ServiceInstance, 0, len(instanceURLs))
	targetURLs := make([]string, 0, len(instanceURLs))

	for _, rawURL := range instanceURLs {
		instance, err := NewServiceInstance(rawURL)
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
	cb := circuitbreaker.NewCircuitBreaker(name+"-cb", cbSettings)

	// 5. Create custom transport with circuit breaker
	transport := circuitbreaker.NewCircuitBreakerTransport(cb, http.DefaultTransport)

	// 6. Create single reverse proxy with dynamic director
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Load balancer selects target for THIS request
			targetURL := lb.Next()
			if targetURL == "" {
				log.Printf("ERROR: No target available for service %s", name)
				return
			}

			target, err := url.Parse(targetURL)
			if err != nil {
				log.Printf("ERROR: Failed to parse target URL %s: %v", targetURL, err)
				return
			}

			// Update request to point to selected backend
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.Header.Add("X-Forwarded-Host", req.Host)
		},
		Transport: transport,
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			// Handle circuit breaker open state
			if err == gobreaker.ErrOpenState {
				log.Printf("Circuit breaker open for service %s: %s", name, req.URL.String())
				rw.WriteHeader(http.StatusServiceUnavailable)
				rw.Write([]byte("Service unavailable due to high failure rate"))
				return
			}

			// Handle other errors
			log.Printf("Reverse proxy error for service %s: %s: %v", name, req.URL.String(), err)
			rw.WriteHeader(http.StatusBadGateway)
			rw.Write([]byte("Bad gateway"))
		},
	}

	return &Service{
		Name:           name,
		Config:         cfg,
		Instances:      instances,
		loadBalancer:   lb,
		circuitBreaker: cb,
		reverseProxy:   proxy,
		transport:      transport,
	}, nil
}

// ServeHTTP handles an HTTP request by proxying to the service
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reverseProxy.ServeHTTP(w, r)
}

// GetInstances returns current service instances (for health/monitoring)
func (s *Service) GetInstances() []*ServiceInstance {
	return s.Instances
}

// GetName returns the service name
func (s *Service) GetName() string {
	return s.Name
}
