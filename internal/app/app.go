package app

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/circuitbreaker"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/proxy"
)

// SetupRouter configures the Gin engine with the necessary routes and middlewares.
func SetupRouter(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// Helper to convert internal config to circuitbreaker settings
	cbSettings := circuitbreaker.Settings{
		MaxRequests: cfg.CircuitBreaker.MaxRequests,
		Interval:    cfg.CircuitBreaker.Interval,
		Timeout:     cfg.CircuitBreaker.Timeout,
	}

	// Set up a catch-all route to handle all incoming requests
	// and forward them to the Proxy function
	router.Any("/users/*path", proxy.Proxy(cfg.UserServiceURL, "UserServiceCircuitBreaker", cbSettings))
	router.Any("/posts/*path", proxy.Proxy(cfg.PostServiceURL, "PostServiceCircuitBreaker", cbSettings))

	return router
}

// Run initializes and starts the Gater application.
func Run() {
	log.Println("Running Gater...")

	cfg := config.LoadConfig()

	router := SetupRouter(cfg)

	// Start the server on the configured port
	router.Run(":" + cfg.Port)
}
