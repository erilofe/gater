package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/circuitbreaker"
	"github.com/pietroagazzi/gater/internal/loadbalancer"
	"github.com/sony/gobreaker"
)

// Proxy forwards incoming requests to targets using load balancing.
// It distributes requests across multiple target instances using round-robin strategy.
func Proxy(targets []string, circuitBreakerName string, circuitBreakerSettings circuitbreaker.Settings) gin.HandlerFunc {
	// Validation: handle empty targets
	if len(targets) == 0 {
		return func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No healthy targets available"})
		}
	}

	// Create load balancer once per route
	loadBalancer := loadbalancer.NewRoundRobin(targets)

	// Create circuit breaker
	circuitBreaker := circuitbreaker.NewCircuitBreaker(circuitBreakerName, circuitBreakerSettings)

	// Build and cache reverse proxies once (per target) during initialization
	proxies := make(map[string]*httputil.ReverseProxy, len(targets))

	for _, target := range targets {
		remote, err := url.Parse(target)
		if err != nil {
			log.Printf("Failed to parse target URL %s: %v", target, err)
			continue
		}

		reverseProxy := httputil.NewSingleHostReverseProxy(remote)
		reverseProxy.Transport = circuitbreaker.NewCircuitBreakerTransport(circuitBreaker, http.DefaultTransport)

		// Custom error handler to manage circuit breaker open state
		reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
			if err == gobreaker.ErrOpenState {
				rw.WriteHeader(http.StatusServiceUnavailable)
				rw.Write([]byte("Service unavailable due to high failure rate"))
				return
			}

			log.Printf("Reverse proxy error for %s: %v", req.URL.String(), err)
			rw.WriteHeader(http.StatusBadGateway)
			rw.Write([]byte("Bad gateway"))
		}

		originalDirector := reverseProxy.Director
		reverseProxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = remote.Host
			req.Header.Add("X-Forwarded-Host", req.Host)
		}

		proxies[target] = reverseProxy
	}

	return func(c *gin.Context) {
		// Get next target from load balancer (per request)
		target := loadBalancer.Next()
		if target == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No targets available"})
			return
		}

		// Select previously built proxy for this request's target
		proxy := proxies[target]

		if proxy == nil {
			log.Printf("No reverse proxy available for target %s", target)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No healthy targets available"})
			return
		}

		// Serve the request
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
