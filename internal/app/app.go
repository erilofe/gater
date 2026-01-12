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
		circuitBreakerName := route.ServiceName + "-cb"

		// Register the route.
		// Strips the prefix before proxying
		// For example, `/api/v1/users` with prefix `/api/v1` becomes `/users`
		router.Any(route.Prefix+"/*path", proxy.Proxy(route.TargetURL, circuitBreakerName, cbSettings))
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

	// Load Routes from YAML
	log.Printf("Loading routes from: %s", cfg.RoutesConfigPath)
	routeConfigs, err := config.LoadRoutesFromFile(cfg.RoutesConfigPath)
	if err != nil {
		log.Fatalf("Failed to load routes config: %v", err)
	}

	log.Printf("Loaded %d routes from config", len(routeConfigs))

	// Resolve each service using discovery provider
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var routes []discovery.ServiceRoute

	for _, routeCfg := range routeConfigs {
		log.Printf("Resolving service: %s for path: %s", routeCfg.ServiceName, routeCfg.Path)

		targetURL, err := provider.ResolveService(ctx, routeCfg.ServiceName)
		if err != nil {
			log.Printf("ERROR: Failed to resolve service %s: %v", routeCfg.ServiceName, err)
			// Skip route and continue
			continue
		}

		routes = append(routes, discovery.ServiceRoute{
			ServiceName: routeCfg.ServiceName,
			Prefix:      routeCfg.Path,
			TargetURL:   targetURL,
		})

		log.Printf("Resolved %s to %s", routeCfg.ServiceName, targetURL)
	}

	if len(routes) == 0 {
		log.Fatalf("No routes configured or all services failed resolution")
	}

	log.Printf("Successfully resolved %d routes", len(routes))

	router := SetupRouter(cfg, routes)

	// Start the server on the configured port
	router.Run(":" + cfg.Port)
}
