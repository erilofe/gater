package discovery

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/pietroagazzi/gater/internal/config"
)

type StaticProvider struct {
	Services map[string][]string
}

func NewStaticProvider(serviceConfigs map[string]*config.ServiceConfig) (*StaticProvider, error) {
	services := make(map[string][]string)

	log.Print("[WARNING] Static provider is enabled, this configuration is not recommended for productive environments")

	for _, service := range serviceConfigs {

		if service.LoadBalancer == nil {
			return nil, fmt.Errorf("all services must specify a load balancer configuration when using the static provider, service %s", service.ServiceName)
		}

		services[service.ServiceName] = make([]string, 0, len(service.LoadBalancer.Endpoints))

		for _, endpoint := range service.LoadBalancer.Endpoints {
			// validate each address
			addr, err := buildEndpointURL(endpoint.Address, *endpoint.Port)

			if err != nil {
				return nil, err
			}

			services[service.ServiceName] = append(services[service.ServiceName], addr)
		}
	}

	return &StaticProvider{Services: services}, nil
}

func buildEndpointURL(address string, port int) (string, error) {
	if strings.Contains(address, "://") {
		return "", fmt.Errorf("address should not include a protocol scheme")
	}

	if strings.Contains(address, "/") {
		return "", fmt.Errorf("address should not contain any path components")
	}

	// Wrap IPv6 addresses in brackets for URL formatting
	host := address
	if strings.Contains(address, ":") {
		host = "[" + address + "]"
	}

	rawURL := fmt.Sprintf("http://%s:%d", host, port)

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint URL %s: %w", rawURL, err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid endpoint URL %s: missing scheme or host", rawURL)
	}

	return rawURL, nil
}

func (p *StaticProvider) Name() string {
	return "static"
}

// StaticProvider stores values in memory, it doesn't require a custom Close
func (p *StaticProvider) Close() error {
	return nil
}

func (p *StaticProvider) ResolveService(_ context.Context, serviceName string) ([]string, error) {
	if len(p.Services[serviceName]) == 0 {
		return nil, fmt.Errorf("no urls found for service %s", serviceName)
	}

	return p.Services[serviceName], nil
}
