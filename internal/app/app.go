package app

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/circuitbreaker"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/discovery"
	"github.com/pietroagazzi/gater/internal/proxy"
)

// SetupRouter configures the Gin engine with the discovered routes.
func SetupRouter(cfg *config.Config, routes []discovery.ServiceRoute) *gin.Engine {
	router := gin.Default()

	// Helper to convert internal config to circuitbreaker settings
	cbSettings := circuitbreaker.Settings{
		MaxRequests: cfg.CircuitBreaker.MaxRequests,
		Interval:    cfg.CircuitBreaker.Interval,
		Timeout:     cfg.CircuitBreaker.Timeout,
	}

	for _, route := range routes {
		log.Printf("Configuring route: %s -> %s (%s)", route.Prefix, route.ServiceName, route.TargetURL)

		// Create unique CB name per service
		cbName := route.ServiceName + "-cb"

		// Register the route.
		// Strips the prefix before proxying
		// For example, `/api/v1/users` with prefix `/api/v1` becomes `/users`
		router.Any(route.Prefix+"/*path", proxy.Proxy(route.TargetURL, cbName, cbSettings))
	}

	// Health check for the gateway itself
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "routes": len(routes)})
	})

	return router
}

// Run initializes and starts the Gater application.
func Run() {
	log.Println("Running Gater...")

	cfg := config.LoadConfig()

	// Initialize Consul Discovery
	discoveryClient, err := discovery.NewClient(cfg.ConsulAddress)
	if err != nil {
		log.Fatalf("Failed to initialize Consul client: %v", err)
	}

	// Dynamic Discovery
	log.Println("Scanning Consul for services with 'gater.prefix' tag...")
	routes, err := discoveryClient.DiscoverRoutes()
	if err != nil {
		log.Fatalf("Failed to discover routes: %v", err)
	}

	if len(routes) == 0 {
		log.Println("Warning: No routes discovered from Consul.")
		// We might want to fallback to static config if desired, but
		// the goal is dynamic discovery.
	}

	router := SetupRouter(cfg, routes)

	// Start the server on the configured port
	router.Run(":" + cfg.Port)
}
