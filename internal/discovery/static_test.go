package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pietroagazzi/gater/internal/config"
)

func TestNewStaticProvider(t *testing.T) {

	localhostEndpoint := config.EndpointConfig{Address: "localhost", Port: new(int)}
	*localhostEndpoint.Port = 8080

	ipv4Endpoint := config.EndpointConfig{Address: "127.0.0.1", Port: new(int)}
	*ipv4Endpoint.Port = 8081

	t.Run("Valid Config", func(t *testing.T) {
		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
				LoadBalancer: &config.LoadBalancerConfig{
					Endpoints: []config.EndpointConfig{
						localhostEndpoint,
						ipv4Endpoint,
					},
				},
			},
		}
		provider, err := NewStaticProvider(services)
		assert.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Len(t, provider.Services["service1"].Urls, 2)
	})

	t.Run("Missing LoadBalancer", func(t *testing.T) {
		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
			},
		}
		_, err := NewStaticProvider(services)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all services must specify a load balancer configuration")
	})

	t.Run("Invalid Address - With Protocol", func(t *testing.T) {
		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
				LoadBalancer: &config.LoadBalancerConfig{
					Endpoints: []config.EndpointConfig{
						{Address: "http://localhost"},
					},
				},
			},
		}
		_, err := NewStaticProvider(services)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "address should not include a protocol")
	})

	t.Run("Invalid Address - With Port", func(t *testing.T) {
		localhostEndpoint := config.EndpointConfig{Address: "localhost:8080", Port: new(int)}
		*localhostEndpoint.Port = 8080

		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
				LoadBalancer: &config.LoadBalancerConfig{
					Endpoints: []config.EndpointConfig{
						localhostEndpoint,
					},
				},
			},
		}
		_, err := NewStaticProvider(services)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not specify a port, use the property `Port` if you need to do so")
	})
}

func TestStaticProvider_Name(t *testing.T) {
	provider := &StaticProvider{}
	assert.Equal(t, "static", provider.Name())
}

func TestStaticProvider_ResolveService(t *testing.T) {

	localhostEndpoint := config.EndpointConfig{Address: "localhost", Port: new(int)}
	*localhostEndpoint.Port = 8080

	ipv6Endpoint := config.EndpointConfig{Address: "[::1]", Port: new(int)}
	*ipv6Endpoint.Port = 8081

	provider := &StaticProvider{
		Services: map[string]*ServiceUrls{
			"service1": {
				Urls: []config.EndpointConfig{
					localhostEndpoint,
					ipv6Endpoint,
				},
			},
			"service2": {
				Urls: []config.EndpointConfig{},
			},
		},
	}

	t.Run("Resolve Existing Service", func(t *testing.T) {
		urls, err := provider.ResolveService(context.Background(), "service1")
		assert.NoError(t, err)
		assert.Len(t, urls, 2)
		assert.Contains(t, urls, "http://localhost:8080")
		assert.Contains(t, urls, "http://[::1]:8081")
	})

	t.Run("Resolve Non-Existent Service", func(t *testing.T) {
		_, err := provider.ResolveService(context.Background(), "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no urls found for service")
	})

	t.Run("Resolve Service with No URLs", func(t *testing.T) {
		_, err := provider.ResolveService(context.Background(), "service2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no urls found for service")
	})
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		wantErr     bool
		errContains string
	}{
		{"Valid IPv4", "192.168.1.1", false, ""},
		{"Valid IPv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false, ""},
		{"Valid Domain", "example.com", false, ""},
		{"Valid Localhost", "localhost", false, ""},
		{"Invalid - With Protocol", "https://example.com", true, "address should not include a protocol"},
		{"Invalid - With Port", "example.com:8080", true, "must not specify a port, use the property `Port` if you need to do so"},
		{"Invalid - Gibberish", "not-a-valid..address", true, "invalid address"},
		{"Invalid - Path", "example.com/path", true, "invalid address"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddress(tt.addr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
