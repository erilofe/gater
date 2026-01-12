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

// ResolveService resolves the target URL of a service via Consul service discovery.
// It queries the Health API for healthy instances and returns the URL of the first available instance.
func (p *ConsulProvider) ResolveService(ctx context.Context, serviceName string) (string, error) {
	return p.getServiceURL(ctx, serviceName)
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
		return "", fmt.Errorf("no healthy instances found for service %q", serviceName)
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
