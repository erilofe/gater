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

		// Modify the request to direct it to the target URL
		proxy.Director = func(req *http.Request) {
			// Copy original request headers
			req.Header = c.Request.Header
			req.Header.Add("X-Forwarded-Host", req.Host)

			// Set the request URL to the target URL
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = remote.Path
			req.URL.RawQuery = remote.RawQuery
		}

		// Serve the request using the reverse proxy
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}
