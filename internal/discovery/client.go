package discovery

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/consul/api"
)

// Client wraps the Consul API client.
type Client struct {
	consul *api.Client
}

// ServiceRoute represents a dynamically discovered route.
type ServiceRoute struct {
	ServiceName string
	Prefix      string
	TargetURL   string
}

// NewClient creates a new Consul discovery client.
func NewClient(address string) (*Client, error) {
	config := api.DefaultConfig()
	config.Address = address

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &Client{consul: client}, nil
}

// GetServiceURL returns the URL (http://address:port) of a healthy instance of the service.
func (c *Client) GetServiceURL(serviceName string) (string, error) {
	// Query only passing (healthy) services
	entries, _, err := c.consul.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query service %s: %w", serviceName, err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("service %s not found or no healthy instances available", serviceName)
	}

	// TODO: Valuate load balancing here

	// Pick the first healthy instance
	entry := entries[0]

	address := entry.Service.Address
	port := entry.Service.Port

	// If Service.Address is empty, fallback to Node.Address
	if address == "" {
		address = entry.Node.Address
	}

	return fmt.Sprintf("http://%s:%d", address, port), nil
}

// DiscoverRoutes scans all services in Consul and looks for the 'gater.prefix' tag.
// It returns a list of routes to be configured.
func (c *Client) DiscoverRoutes() ([]ServiceRoute, error) {
	// Get all services and their tags from the Catalog
	services, _, err := c.consul.Catalog().Services(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var routes []ServiceRoute

	for serviceName, tags := range services {
		for _, tag := range tags {
			// Check for our convention "gater.prefix=/somepath"
			if after, ok := strings.CutPrefix(tag, "gater.prefix="); ok {
				prefix := after

				// Resolve the target URL for this service
				targetURL, err := c.GetServiceURL(serviceName)
				if err != nil {
					// Log the error but continue processing other services
					log.Printf("Warning: could not get URL for service %s: %v", serviceName, err)
					continue
				}

				routes = append(routes, ServiceRoute{
					ServiceName: serviceName,
					Prefix:      prefix,
					TargetURL:   targetURL,
				})

				// Only one gater.prefix tag per service is supported
				break
			}
		}
	}

	return routes, nil
}
