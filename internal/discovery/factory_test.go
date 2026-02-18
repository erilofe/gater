package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pietroagazzi/gater/internal/config"
)

func TestDetermineProviderType(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		expectedType  ProviderType
		expectedError bool
	}{
		{
			name: "Explicit DiscoveryProvider",
			cfg: &config.Config{
				DiscoveryProvider: "consul",
			},
			expectedType:  ProviderTypeConsul,
			expectedError: false,
		},
		{
			name: "Static Provider",
			cfg: &config.Config{
				DiscoveryProvider: "static",
			},
			expectedType:  ProviderTypeStatic,
			expectedError: false,
		},
		{
			name: "No Configuration (even if ConsulAddress is set)",
			cfg: &config.Config{
				DiscoveryProvider: "",
				ConsulAddress:     "localhost:8500",
			},
			expectedType:  "",
			expectedError: true,
		},
		{
			name: "Unknown Provider",
			cfg: &config.Config{
				DiscoveryProvider: "unknown",
			},
			expectedType:  ProviderType("unknown"),
			expectedError: false, // determineProviderType doesn't validate if it's 'known', only that it's set. Validation happens in NewProvider.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pType, err := determineProviderType(tt.cfg)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, pType)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	// Start a mock Consul server (reusing helper from consul_test.go)
	server := mockConsulServer(t, nil)
	defer server.Close()

	t.Run("Create Consul Provider", func(t *testing.T) {
		cfg := &config.Config{
			DiscoveryProvider: "consul",
			ConsulAddress:     server.URL,
		}

		p, err := NewProvider(cfg, nil)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "consul", p.Name())
	})

	t.Run("Create Static Provider", func(t *testing.T) {
		cfg := &config.Config{
			DiscoveryProvider: "static",
		}
		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
				LoadBalancer: &config.LoadBalancerConfig{
					Endpoints: []config.EndpointConfig{
						{Address: "localhost"},
					},
				},
			},
		}

		p, err := NewProvider(cfg, services)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "static", p.Name())
	})

	t.Run("Create Static Provider - Missing LoadBalancer", func(t *testing.T) {
		cfg := &config.Config{
			DiscoveryProvider: "static",
		}
		services := map[string]*config.ServiceConfig{
			"service1": {
				ServiceName: "service1",
			},
		}

		p, err := NewProvider(cfg, services)
		assert.Error(t, err)
		assert.Nil(t, p)
		assert.Contains(t, err.Error(), "all services must specify a load balancer configuration")
	})

	t.Run("Unknown Provider", func(t *testing.T) {
		cfg := &config.Config{
			DiscoveryProvider: "alien_tech",
		}

		p, err := NewProvider(cfg, nil)
		assert.Error(t, err)
		assert.Nil(t, p)
		assert.Contains(t, err.Error(), "unknown provider type")
	})

	t.Run("Missing Configuration", func(t *testing.T) {
		cfg := &config.Config{
			DiscoveryProvider: "",
		}

		p, err := NewProvider(cfg, nil)
		assert.Error(t, err)
		assert.Nil(t, p)
	})
}
