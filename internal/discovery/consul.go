package discovery

import (
	"context"
	"fmt"

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

// Name returns the provider name for logging purposes.
func (p *ConsulProvider) Name() string {
	return "consul"
}

// Close performs cleanup. Consul client doesn't require explicit cleanup.
func (p *ConsulProvider) Close() error {
	return nil
}

// ResolveService resolves the target URLs of all healthy instances of a service via Consul service discovery.
// It queries the Health API for healthy instances and returns URLs for all available instances.
func (p *ConsulProvider) ResolveService(ctx context.Context, serviceName string) ([]string, error) {
	return p.getServiceURLs(ctx, serviceName)
}

// getServiceURLs returns the URLs (http://address:port) of all healthy instances of the service.
func (p *ConsulProvider) getServiceURLs(ctx context.Context, serviceName string) ([]string, error) {
	opts := &api.QueryOptions{}
	opts = opts.WithContext(ctx)

	// Query only passing (healthy) services
	entries, _, err := p.client.Health().Service(serviceName, "", true, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query service: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no healthy instances found for service %q", serviceName)
	}

	// Collect all healthy instances
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		service := entry.Service
		address := service.Address

		// If Service.Address is empty, fallback to Node.Address
		if address == "" {
			address = entry.Node.Address
		}

		urls = append(urls, fmt.Sprintf("http://%s:%d", address, service.Port))
	}

	return urls, nil
}
