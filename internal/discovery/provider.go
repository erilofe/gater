package discovery

import "context"

// Provider is the interface that all service discovery implementations must satisfy.
type Provider interface {
	// ResolveService resolves the target URLs of all healthy instances of a service by name.
	// Returns a slice of service URLs (e.g., ["http://host1:port", "http://host2:port"]) for all healthy instances.
	// If no healthy instances are found, returns an error.
	ResolveService(ctx context.Context, serviceName string) ([]string, error)

	// Name returns a human-readable name for this provider (e.g., "consul", "static").
	Name() string

	// Close performs cleanup of any resources held by the provider.
	Close() error
}
