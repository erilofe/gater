package service

import (
	"fmt"
	"net/url"
)

// ServiceInstance represents a single backend instance of a service
type ServiceInstance struct {
	URL    string // Full URL (e.g., "http://10.0.1.5:8080")
	Host   string // Host:Port (e.g., "10.0.1.5:8080")
	Scheme string // http or https
}

// NewServiceInstance parses a URL string and creates a ServiceInstance
func NewServiceInstance(rawURL string) (*ServiceInstance, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty URL provided")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %s: %w", rawURL, err)
	}

	if parsed.Scheme == "" {
		return nil, fmt.Errorf("URL missing scheme: %s", rawURL)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("URL missing host: %s", rawURL)
	}

	return &ServiceInstance{
		URL:    rawURL,
		Host:   parsed.Host,
		Scheme: parsed.Scheme,
	}, nil
}
