package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRoutesFromFile(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		createFile     bool
		expectedError  bool
		errorContains  string
		expectedRoutes []RouteConfig
	}{
		{
			name: "Success - Valid Routes",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
    methods: [GET, POST]
  - path: /api/v1/orders
    service_name: order-service
    methods: [GET]
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []RouteConfig{
				{Path: "/api/v1/users", ServiceName: "user-service", Methods: []string{"GET", "POST"}},
				{Path: "/api/v1/orders", ServiceName: "order-service", Methods: []string{"GET"}},
			},
		},
		{
			name:          "Failure - File Not Found",
			createFile:    false,
			expectedError: true,
			errorContains: "failed to read routes file",
		},
		{
			name: "Failure - Malformed YAML",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
  - invalid_yaml_indentation
`,
			createFile:    true,
			expectedError: true,
			errorContains: "failed to parse routes YAML",
		},
		{
			name: "Failure - Missing Path",
			yamlContent: `
routes:
  - service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "path and service_name are required",
		},
		{
			name: "Failure - Missing Service Name",
			yamlContent: `
routes:
  - path: /api/v1/users
`,
			createFile:    true,
			expectedError: true,
			errorContains: "path and service_name are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.createFile {
				// Create a temporary file for the test
				tmpFile, err := os.CreateTemp("", "routes_*.yml")
				require.NoError(t, err)
				defer os.Remove(tmpFile.Name()) // Clean up

				_, err = tmpFile.WriteString(tt.yamlContent)
				require.NoError(t, err)
				tmpFile.Close()
				path = tmpFile.Name()
			} else {
				// Use a non-existent path
				path = filepath.Join(os.TempDir(), "non_existent_file.yml")
			}

			routes, err := LoadRoutesFromFile(path)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRoutes, routes)
			}
		})
	}
}

// TestLoadConfig verifies env var loading (basic smoke test)
func TestLoadConfig(t *testing.T) {
	// Set some env vars
	os.Setenv("PORT", "9090")
	os.Setenv("DISCOVERY_PROVIDER", "consul")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("DISCOVERY_PROVIDER")

	cfg := LoadConfig()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "consul", cfg.DiscoveryProvider)
	assert.Equal(t, time.Second*30, cfg.CircuitBreaker.Timeout) // Default value check
}
