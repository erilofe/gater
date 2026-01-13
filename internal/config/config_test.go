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
		expectedRoutes []*Route
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
			expectedRoutes: []*Route{
				{Path: "/api/v1/users", ServiceName: "user-service", Methods: []string{"GET", "POST"}, Priority: 0},
				{Path: "/api/v1/orders", ServiceName: "order-service", Methods: []string{"GET"}, Priority: 0},
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
			expectedRoutes: []*Route{
				{Path: "/users", ServiceName: "user-service", Methods: []string{"GET"}, Priority: 0},
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
	assert.Equal(t, "config/routes.yml", cfg.RoutesConfigPath)     // Default value check
	assert.Equal(t, "config/services.yml", cfg.ServicesConfigPath) // Default value check
}

func TestLoadServicesFromFile(t *testing.T) {
	tests := []struct {
		name             string
		yamlContent      string
		createFile       bool
		expectedError    bool
		errorContains    string
		expectedServices map[string]*ServiceConfig
	}{
		{
			name: "Success - Valid Services Configuration",
			yamlContent: `
services:
  user-service:
    service_name: user-service
    resilience:
      circuit_breaker:
        recovery_timeout: 30s
  post-service:
    service_name: post-service
    resilience:
      circuit_breaker:
        recovery_timeout: 20s
`,
			createFile:    true,
			expectedError: false,
			expectedServices: map[string]*ServiceConfig{
				"user-service": {
					ServiceName: "user-service",
				},
				"post-service": {
					ServiceName: "post-service",
				},
			},
		},
		{
			name: "Success - Service Name Inferred from Key",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 30s
`,
			createFile:    true,
			expectedError: false,
			expectedServices: map[string]*ServiceConfig{
				"user-service": {
					ServiceName: "user-service",
				},
			},
		},
		{
			name:          "Failure - File Not Found",
			createFile:    false,
			expectedError: true,
			errorContains: "failed to read services config file",
		},
		{
			name: "Failure - Malformed YAML",
			yamlContent: `
services:
  user-service:
    service_name: user-service
  - invalid_yaml_syntax
`,
			createFile:    true,
			expectedError: true,
			errorContains: "failed to parse services config",
		},
		{
			name: "Failure - Empty Services",
			yamlContent: `
services: {}
`,
			createFile:    true,
			expectedError: true,
			errorContains: "no services defined",
		},
		{
			name: "Failure - No Services Key",
			yamlContent: `
other_config:
  value: 123
`,
			createFile:    true,
			expectedError: true,
			errorContains: "no services defined",
		},
		{
			name: "Failure - Missing Recovery Timeout",
			yamlContent: `
services:
  user-service:
    service_name: user-service
    resilience:
      circuit_breaker: {}
`,
			createFile:    true,
			expectedError: true,
			errorContains: "recovery_timeout must be > 0",
		},
		{
			name: "Failure - Zero Recovery Timeout",
			yamlContent: `
services:
  user-service:
    service_name: user-service
    resilience:
      circuit_breaker:
        recovery_timeout: 0s
`,
			createFile:    true,
			expectedError: true,
			errorContains: "recovery_timeout must be > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.createFile {
				// Create a temporary file for the test
				tmpFile, err := os.CreateTemp("", "services_*.yml")
				require.NoError(t, err)
				defer os.Remove(tmpFile.Name()) // Clean up

				_, err = tmpFile.WriteString(tt.yamlContent)
				require.NoError(t, err)
				tmpFile.Close()
				path = tmpFile.Name()
			} else {
				// Use a non-existent path
				path = filepath.Join(os.TempDir(), "non_existent_services.yml")
			}

			services, err := LoadServicesFromFile(path)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, services)

				// Check that expected services are present
				for serviceName, expectedConfig := range tt.expectedServices {
					actualConfig, exists := services[serviceName]
					assert.True(t, exists, "Service %s should exist", serviceName)
					if exists {
						assert.Equal(t, expectedConfig.ServiceName, actualConfig.ServiceName)
					}
				}
			}
		})
	}
}

func TestLoadServicesFromFile_EmptyFilepath(t *testing.T) {
	services, err := LoadServicesFromFile("")

	assert.Error(t, err)
	assert.Nil(t, services)
	assert.Contains(t, err.Error(), "filepath is empty")
}

func TestLoadServicesFromFile_CircuitBreakerValues(t *testing.T) {
	yamlContent := `
services:
  test-service:
    service_name: test-service
    resilience:
      circuit_breaker:
        recovery_timeout: 45s
`

	tmpFile, err := os.CreateTemp("", "services_*.yml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	services, err := LoadServicesFromFile(tmpFile.Name())

	require.NoError(t, err)
	require.NotNil(t, services)
	require.Contains(t, services, "test-service")

	testSvc := services["test-service"]
	assert.Equal(t, "test-service", testSvc.ServiceName)
	assert.Equal(t, 45*time.Second, testSvc.Resilience.CircuitBreaker.RecoveryTimeout)
}

func TestLoadServicesFromFile_MultipleServices(t *testing.T) {
	yamlContent := `
services:
  service-1:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
  service-2:
    resilience:
      circuit_breaker:
        recovery_timeout: 20s
  service-3:
    resilience:
      circuit_breaker:
        recovery_timeout: 30s
`

	tmpFile, err := os.CreateTemp("", "services_*.yml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	tmpFile.Close()

	services, err := LoadServicesFromFile(tmpFile.Name())

	require.NoError(t, err)
	assert.Equal(t, 3, len(services))
	assert.Contains(t, services, "service-1")
	assert.Contains(t, services, "service-2")
	assert.Contains(t, services, "service-3")

	// Verify service names are set correctly
	assert.Equal(t, "service-1", services["service-1"].ServiceName)
	assert.Equal(t, "service-2", services["service-2"].ServiceName)
	assert.Equal(t, "service-3", services["service-3"].ServiceName)
}
