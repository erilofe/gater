package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"

	"github.com/pietroagazzi/gater/internal/config"
)

type StaticProvider struct {
	Services map[string]*ServiceUrls
}

type ServiceUrls struct {
	Urls []config.EndpointConfig
}

func NewStaticProvider(serviceConfigs map[string]*config.ServiceConfig) (*StaticProvider, error) {
	services := make(map[string]*ServiceUrls)

	log.Print("[WARNING] Static provider is enabled, this configuration is not recommended for productive environments ")

	for _, service := range serviceConfigs {

		if service.LoadBalancer == nil {
			return nil, fmt.Errorf("all services must specify a load balancer configuration when using the static provider, service %s", service.ServiceName)
		}

		services[service.ServiceName] = new(ServiceUrls)
		services[service.ServiceName].Urls = make([]config.EndpointConfig, 0, len(service.LoadBalancer.Endpoints))

		for _, endpoint := range service.LoadBalancer.Endpoints {

			// validate each address
			if err := validateAddress(endpoint.Address); err != nil {
				return nil, err
			}

			// It's IPV6
			if strings.Contains(endpoint.Address, ":") {
				endpoint.Address = `[` + endpoint.Address + `]`
			}

			services[service.ServiceName].Urls = append(services[service.ServiceName].Urls, endpoint)
		}
	}

	return &StaticProvider{Services: services}, nil
}

// This regex allows single-label hosts (user,post) and FQDNs (google.com)
// It strictly follows RFC 1123 for allowed characters.
var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`)

func validateAddress(addr string) error {
	if strings.Contains(addr, "://") {
		return fmt.Errorf("address should not include a protocol")
	}

	if net.ParseIP(addr) == nil {
		if _, _, err := net.SplitHostPort(addr); err == nil {
			return fmt.Errorf("must not specify a port, use the property `Port` if you need to do so")
		}
	} else {
		// Its a valid IP and has no port
		return nil
	}

	// 4. Check if it's a valid Domain
	if domainRegex.MatchString(addr) {
		return nil
	}

	return fmt.Errorf("invalid address %s ", addr)
}

func (p *StaticProvider) Name() string {
	return "static"
}

// StaticProvider stores values in memory, it doesn't require a custom Close
func (p *StaticProvider) Close() error {
	return nil
}

func (p *StaticProvider) ResolveService(_ context.Context, serviceName string) ([]string, error) {
	if p.Services[serviceName] == nil || len(p.Services[serviceName].Urls) == 0 {
		return nil, fmt.Errorf("no urls found for service %s and provider static ", serviceName)
	}

	urls := make([]string, 0, len(p.Services[serviceName].Urls))
	for _, entry := range p.Services[serviceName].Urls {
		urls = append(urls, fmt.Sprintf("http://%s:%d", entry.Address, *entry.Port))
	}

	return urls, nil
}
