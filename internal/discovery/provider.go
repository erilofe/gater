package discovery

import "context"

// Provider is the interface that all service discovery implementations must satisfy.
type Provider interface {
	// DiscoverRoutes returns all available service routes.
	// Implementations should handle their own error recovery and logging.
	DiscoverRoutes(ctx context.Context) ([]ServiceRoute, error)

	// Name returns a human-readable name for this provider (e.g., "consul", "static").
	Name() string

	// Close performs cleanup of any resources held by the provider.
	Close() error
}
