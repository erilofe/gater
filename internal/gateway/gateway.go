package gateway

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/service"
)

// Gateway orchestrates services and routes
type Gateway struct {
	services map[string]*service.Service
	routes   []*config.Route
}

// NewGateway creates a new gateway with services and routes
func NewGateway(services map[string]*service.Service, routes []*config.Route) *Gateway {
	return &Gateway{
		services: services,
		routes:   routes,
	}
}

// SetupRouter configures Gin with all routes
func (g *Gateway) SetupRouter() *gin.Engine {
	router := gin.Default()

	// Register routes
	for _, route := range g.routes {
		svc, exists := g.services[route.ServiceName]
		if !exists {
			log.Printf("WARNING: Route %s references unknown service %s", route.Path, route.ServiceName)
			continue
		}

		// Create handler that delegates to service
		handler := g.createServiceHandler(svc)

		// Register with Gin
		// Add /*path wildcard to capture remaining path segments
		routePath := route.Path + "/*path"

		if len(route.Methods) == 0 {
			// No methods specified = accept all methods
			router.Any(routePath, handler)
			log.Printf("Registered route: ANY %s -> %s (%d instances)", route.Path, route.ServiceName, len(svc.GetInstances()))
		} else {
			// Register specific methods
			for _, method := range route.Methods {
				router.Handle(method, routePath, handler)
			}
			log.Printf("Registered route: %v %s -> %s (%d instances)", route.Methods, route.Path, route.ServiceName, len(svc.GetInstances()))
		}
	}

	// Health check
	router.GET("/health", g.healthCheck)

	return router
}

// createServiceHandler wraps service.ServeHTTP for Gin
func (g *Gateway) createServiceHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc.ServeHTTP(c.Writer, c.Request)
	}
}

// healthCheck returns gateway health status
func (g *Gateway) healthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":   "ok",
		"services": len(g.services),
		"routes":   len(g.routes),
	})
}

// GetService returns a service by name (for monitoring/management)
func (g *Gateway) GetService(name string) (*service.Service, bool) {
	svc, exists := g.services[name]
	return svc, exists
}

// GetServices returns all services
func (g *Gateway) GetServices() map[string]*service.Service {
	return g.services
}

// GetRoutes returns all routes
func (g *Gateway) GetRoutes() []*config.Route {
	return g.routes
}
