package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/discovery"
	"github.com/pietroagazzi/gater/internal/gateway"
	"github.com/pietroagazzi/gater/internal/service"
)

// collectUniqueServiceNames extracts unique service names from routes
func collectUniqueServiceNames(routes []*config.Route) []string {
	serviceMap := make(map[string]bool)
	for _, route := range routes {
		serviceMap[route.ServiceName] = true
	}

	services := make([]string, 0, len(serviceMap))
	for serviceName := range serviceMap {
		services = append(services, serviceName)
	}
	return services
}

// loadConfiguration loads and validates all configuration files
func loadConfiguration(cfg *config.Config) ([]*config.Route, map[string]*config.ServiceConfig, error) {
	// Load routes from YAML
	log.Printf("Loading routes from: %s", cfg.RoutesConfigPath)
	routes, err := config.LoadRoutesFromFile(cfg.RoutesConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load routes config: %w", err)
	}
	log.Printf("Loaded %d routes from config", len(routes))

	// Load services configuration from YAML
	log.Printf("Loading services config from: %s", cfg.ServicesConfigPath)
	servicesConfig, err := config.LoadServicesFromFile(cfg.ServicesConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load services config: %w", err)
	}
	log.Printf("Loaded configuration for %d services", len(servicesConfig))

	return routes, servicesConfig, nil
}

// resolveAndCreateService resolves a single service and creates its entity
func resolveAndCreateService(
	ctx context.Context,
	serviceName string,
	servicesConfig map[string]*config.ServiceConfig,
	provider discovery.Provider,
	servicesConfigPath string,
) (*service.Service, error) {
	log.Printf("Resolving service: %s", serviceName)

	// Check if service config exists
	svcConfig, exists := servicesConfig[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s referenced in routes but not found in %s", serviceName, servicesConfigPath)
	}

	// Resolve service instances from discovery provider
	instanceURLs, err := provider.ResolveService(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve service %s from discovery: %w", serviceName, err)
	}

	if len(instanceURLs) == 0 {
		return nil, fmt.Errorf("no instances found for service %s", serviceName)
	}

	log.Printf("Resolved %s to %d instance(s): %v", serviceName, len(instanceURLs), instanceURLs)

	// Create Service entity
	svc, err := service.NewService(serviceName, instanceURLs, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create service %s: %w", serviceName, err)
	}

	log.Printf("Created service entity for %s with %d instances", serviceName, len(svc.GetInstances()))
	return svc, nil
}

// buildServices resolves and creates all service entities
func buildServices(
	ctx context.Context,
	routes []*config.Route,
	servicesConfig map[string]*config.ServiceConfig,
	provider discovery.Provider,
	servicesConfigPath string,
) (map[string]*service.Service, error) {
	serviceNames := collectUniqueServiceNames(routes)
	log.Printf("Unique services referenced in routes: %v", serviceNames)

	services := make(map[string]*service.Service)

	for _, serviceName := range serviceNames {
		svc, err := resolveAndCreateService(ctx, serviceName, servicesConfig, provider, servicesConfigPath)
		if err != nil {
			return nil, err
		}
		services[serviceName] = svc
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no services configured")
	}

	return services, nil
}

// validateRoutes ensures all routes reference valid services
func validateRoutes(routes []*config.Route, services map[string]*service.Service) error {
	for _, route := range routes {
		if _, exists := services[route.ServiceName]; !exists {
			return fmt.Errorf("route %s references unknown service %s", route.Path, route.ServiceName)
		}
	}
	return nil
}

// initializeGateway creates and configures the gateway
func initializeGateway(services map[string]*service.Service, routes []*config.Route) *gateway.Gateway {
	log.Println("Creating gateway...")
	gw := gateway.NewGateway(services, routes)

	log.Println("Setting up router...")
	return gw
}

// RunWithContext initializes and starts the Gater application with a cancellable context.
// This function is designed for testing and allows graceful shutdown via context cancellation.
func RunWithContext(ctx context.Context, cfg *config.Config) (*gateway.Gateway, error) {
	log.Println("Running Gater...")

	// Use provided config or load default
	if cfg == nil {
		cfg = config.LoadConfig()
	}
	log.Printf("Configuration loaded: Port=%s, Consul=%s", cfg.Port, cfg.ConsulAddress)

	// Initialize Service Discovery Provider
	provider, err := discovery.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize discovery provider: %w", err)
	}
	defer provider.Close()
	log.Printf("Using service discovery provider: %s", provider.Name())

	// Load all configuration files
	routes, servicesConfig, err := loadConfiguration(cfg)
	if err != nil {
		return nil, fmt.Errorf("configuration loading failed: %w", err)
	}

	// Resolve and create service entities
	resolveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	services, err := buildServices(resolveCtx, routes, servicesConfig, provider, cfg.ServicesConfigPath)
	if err != nil {
		return nil, fmt.Errorf("service resolution failed: %w", err)
	}

	// Validate routes reference valid services
	if err := validateRoutes(routes, services); err != nil {
		return nil, fmt.Errorf("route validation failed: %w", err)
	}

	// Create and configure gateway
	gw := initializeGateway(services, routes)
	router := gw.SetupRouter()

	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting gateway server on %s", serverAddr)
	log.Printf("Gateway initialized with %d services and %d routes", len(services), len(routes))

	srv := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return nil, fmt.Errorf("server shutdown failed: %w", err)
		}

		return gw, nil

	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return gw, nil
		}
		return nil, fmt.Errorf("server failed: %w", err)
	}
}

// Run initializes and starts the Gater application.
// This is the main entry point for production use.
func Run() {
	gw, err := RunWithContext(context.Background(), nil)
	if err != nil {
		log.Fatalf("Application failed: %v", err)
	}
	// Never reached, context.Background() never cancels
	_ = gw
}
