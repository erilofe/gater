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
			name: "Success - Trailing Slash Normalized",
			yamlContent: `
routes:
  - path: /users/
    service_name: user-service
    methods: [GET]
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []RouteConfig{
				{Path: "/users", ServiceName: "user-service", Methods: []string{"GET"}},
			},
		},
		{
			name: "Success - Methods Normalized (Trim + Uppercase)",
			yamlContent: `
routes:
  - path: /users
    service_name: user-service
    methods: [" get ", "post"]
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []RouteConfig{
				{Path: "/users", ServiceName: "user-service", Methods: []string{"GET", "POST"}},
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
			name: "Failure - Path Missing Leading Slash",
			yamlContent: `
routes:
  - path: users
    service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "path must start with '/'",
		},
		{
			name: "Failure - Path Contains Whitespace",
			yamlContent: `
routes:
  - path: /api v1/users
    service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "path must not contain whitespace",
		},
		{
			name: "Failure - Path Contains Query Or Fragment",
			yamlContent: `
routes:
  - path: /users?debug=true
    service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "path must not contain '?' or '#'",
		},
		{
			name: "Failure - Path Contains Double Slash",
			yamlContent: `
routes:
  - path: /users//admin
    service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "consecutive slashes",
		},
		{
			name: "Failure - Path Contains Param Or Wildcard Tokens",
			yamlContent: `
routes:
  - path: /users/:id
    service_name: user-service
`,
			createFile:    true,
			expectedError: true,
			errorContains: "use a static prefix only",
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
		{
			name: "Failure - Invalid Method",
			yamlContent: `
routes:
  - path: /users
    service_name: user-service
    methods: [FETCH]
`,
			createFile:    true,
			expectedError: true,
			errorContains: "invalid method",
		},
		{
			name: "Failure - Empty Method Value",
			yamlContent: `
routes:
  - path: /users
    service_name: user-service
    methods: ["   "]
`,
			createFile:    true,
			expectedError: true,
			errorContains: "methods must not contain empty values",
		},
		{
			name: "Failure - Duplicate Method (After Normalization)",
			yamlContent: `
routes:
  - path: /users
    service_name: user-service
    methods: ["get", " GET "]
`,
			createFile:    true,
			expectedError: true,
			errorContains: "duplicate method",
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
