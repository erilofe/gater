package discovery

import (
	"testing"

	"github.com/pietroagazzi/gater/internal/config"
	"github.com/stretchr/testify/assert"
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
