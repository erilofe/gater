package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/pkg/circuit_breaker"
	"github.com/sony/gobreaker"
)

// Proxy forwards incoming requests to the target URL specified in the request URI.
func Proxy(target string, cbName string) gin.HandlerFunc {
	remote, err := url.Parse(target)

	if err != nil {
		return func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target URL"})
		}
	}

	// Create the reverse proxy once
	proxy := httputil.NewSingleHostReverseProxy(remote)

	cb := circuit_breaker.NewCircuitBreaker(cbName)

	proxy.Transport = circuit_breaker.NewCircuitBreakerTransport(cb, http.DefaultTransport)

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		if err == gobreaker.ErrOpenState {
			rw.WriteHeader(http.StatusServiceUnavailable) // 503 Service Unavailable
			rw.Write([]byte("Service unavailable due to high failure rate"))
			return
		}

		log.Printf("Reverse proxy error for %s: %v", req.URL.String(), err)
		rw.WriteHeader(http.StatusBadGateway) // 502 Bad Gateway
		rw.Write([]byte("Bad gateway"))
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = remote.Host
		req.Header.Add("X-Forwarded-Host", req.Host)
	}

	return func(c *gin.Context) {
		// Serve the request using the reverse proxy
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
