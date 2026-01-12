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
	lb := loadbalancer.NewRoundRobin(targets)

	// Create circuit breaker
	cb := circuitbreaker.NewCircuitBreaker(circuitBreakerName, circuitBreakerSettings)

	return func(c *gin.Context) {
		// Get next target from load balancer (per request)
		target := lb.Next()
		if target == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No targets available"})
			return
		}

		// Parse target URL
		remote, err := url.Parse(target)
		if err != nil {
			log.Printf("Failed to parse target URL %s: %v", target, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL"})
			return
		}

		// Create reverse proxy for this request
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.Transport = circuitbreaker.NewCircuitBreakerTransport(cb, http.DefaultTransport)

		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
			if err == gobreaker.ErrOpenState {
				rw.WriteHeader(http.StatusServiceUnavailable)
				rw.Write([]byte("Service unavailable due to high failure rate"))
				return
			}

			log.Printf("Reverse proxy error for %s -> %s: %v", req.URL.String(), target, err)
			rw.WriteHeader(http.StatusBadGateway)
			rw.Write([]byte("Bad gateway"))
		}

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = remote.Host
			req.Header.Add("X-Forwarded-Host", req.Host)
		}

		// Serve the request
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
