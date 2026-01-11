package discovery

import "context"

// Provider is the interface that all service discovery implementations must satisfy.
type Provider interface {
	// ResolveService resolves the target URL of a specific service by name.
	// Returns the service URL (e.g., "http://host:port") for healthy instances.
	ResolveService(ctx context.Context, serviceName string) (string, error)

	// Name returns a human-readable name for this provider (e.g., "consul", "static").
	Name() string

	// Close performs cleanup of any resources held by the provider.
	Close() error
}
