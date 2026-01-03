package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Proxy forwards incoming requests to the target URL specified in the request URI.
func Proxy(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		remote, err := url.Parse(target)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target URL"})
			return
		}

		// Create the reverse proxy
		proxy := httputil.NewSingleHostReverseProxy(remote)

		// ... Add circuit breaker or other middleware here

		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = remote.Host
			req.Header.Add("X-Forwarded-Host", req.Host)
		}

		// Serve the request using the reverse proxy
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
