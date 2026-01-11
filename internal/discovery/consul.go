package discovery

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/consul/api"
)

// ConsulProvider implements the Provider interface using Consul for service discovery.
type ConsulProvider struct {
	client *api.Client
}

// NewConsulProvider creates a new Consul provider.
// It performs a connectivity check and returns an error if Consul is unreachable (fail-fast).
func NewConsulProvider(address string) (*ConsulProvider, error) {
	config := api.DefaultConfig()
	config.Address = address

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	// Test connectivity (fail-fast)
	_, err = client.Agent().Self()
	if err != nil {
		return nil, fmt.Errorf("consul unreachable at %s: %w", address, err)
	}

	return &ConsulProvider{client: client}, nil
}

// DiscoverRoutes scans all services in Consul and looks for the 'gater.prefix' tag.
// It returns a list of routes to be configured.
func (p *ConsulProvider) DiscoverRoutes(ctx context.Context) ([]ServiceRoute, error) {
	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)

	// Get all services and their tags from the Catalog
	services, _, err := p.client.Catalog().Services(opts)
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
				targetURL, err := p.getServiceURL(ctx, serviceName)
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

// Name returns the provider name for logging purposes.
func (p *ConsulProvider) Name() string {
	return "consul"
}

// Close performs cleanup. Consul client doesn't require explicit cleanup.
func (p *ConsulProvider) Close() error {
	return nil
}

// getServiceURL returns the URL (http://address:port) of a healthy instance of the service.
func (p *ConsulProvider) getServiceURL(ctx context.Context, serviceName string) (string, error) {
	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)

	// Query only passing (healthy) services
	entries, _, err := p.client.Health().Service(serviceName, "", true, opts)
	if err != nil {
		return "", fmt.Errorf("failed to query service: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no healthy instances found")
	}

	// TODO: Evaluate load balancing here
	// Pick the first healthy instance
	service := entries[0].Service
	address := service.Address

	// If Service.Address is empty, fallback to Node.Address
	if address == "" {
		address = entries[0].Node.Address
	}

	return fmt.Sprintf("http://%s:%d", address, service.Port), nil
}
