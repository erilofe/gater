package app

import (
	"context"
	"log"
	"time"

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

	// Initialize Service Discovery Provider
	provider, err := discovery.NewProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize discovery provider: %v", err)
	}
	defer provider.Close()

	log.Printf("Using service discovery provider: %s", provider.Name())

	// Discover Routes with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Discovering services...")
	routes, err := provider.DiscoverRoutes(ctx)
	if err != nil {
		log.Fatalf("Failed to discover routes from %s: %v", provider.Name(), err)
	}

	log.Printf("Discovered %d routes", len(routes))

	if len(routes) == 0 {
		log.Printf("Warning: No routes discovered from %s provider.", provider.Name())
		// The application will continue but won't proxy any requests
	}

	router := SetupRouter(cfg, routes)

	// Start the server on the configured port
	router.Run(":" + cfg.Port)
}
