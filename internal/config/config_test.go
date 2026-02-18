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
			name: "Success - Methods Normalized (case/whitespace/dedup)",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
    methods: [get, " POST ", GET, ""]
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []*Route{
				{Path: "/api/v1/users", ServiceName: "user-service", Methods: []string{"GET", "POST"}, Priority: 0},
			},
		},
		{
			name: "Success - Methods Empty Means All Methods",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []*Route{
				{Path: "/api/v1/users", ServiceName: "user-service", Methods: []string{}, Priority: 0},
			},
		},
		{
			name: "Success - Priority Parsed",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
    methods: [GET]
    priority: 10
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []*Route{
				{Path: "/api/v1/users", ServiceName: "user-service", Methods: []string{"GET"}, Priority: 10},
			},
		},
		{
			name:          "Failure - Empty Filepath",
			createFile:    true,
			yamlContent:   "",
			expectedError: true,
			errorContains: "routes config filepath is empty",
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
			name: "Failure - No Routes Defined",
			yamlContent: `
routes: []
`,
			createFile:    true,
			expectedError: true,
			errorContains: "no routes defined",
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
			name: "Failure - Methods All Empty After Normalization",
			yamlContent: `
routes:
  - path: /api/v1/users
    service_name: user-service
    methods: ["", "   "]
`,
			createFile:    true,
			expectedError: true,
			errorContains: "methods are empty after normalization",
		},
		{
			name: "Success - Path Is Root Kept",
			yamlContent: `
routes:
  - path: /
    service_name: root-service
    methods: [GET]
`,
			createFile:    true,
			expectedError: false,
			expectedRoutes: []*Route{
				{Path: "/", ServiceName: "root-service", Methods: []string{"GET"}, Priority: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.name == "Failure - Empty Filepath" {
				path = ""
			} else if tt.createFile {
				// Create a temporary file for the test
				tmpFile, err := os.CreateTemp(t.TempDir(), "routes_*.yml")
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
	t.Setenv("PORT", "9090")
	t.Setenv("DISCOVERY_PROVIDER", "consul")

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
				tmpFile, err := os.CreateTemp(t.TempDir(), "services_*.yml")
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

	tmpFile, err := os.CreateTemp(t.TempDir(), "services_*.yml")
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

	tmpFile, err := os.CreateTemp(t.TempDir(), "services_*.yml")
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

func TestLoadServicesFromFile_LoadBalancer(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectedError bool
		errorContains string
		check         func(t *testing.T, services map[string]*ServiceConfig)
	}{
		{
			name: "Failure - Load Balancer with no endpoints",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints: []
`,
			expectedError: true,
			errorContains: "must have at least one endpoint",
		},
		{
			name: "Failure - Load Balancer endpoint with no address",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: ""
          port: 8080
`,
			expectedError: true,
			errorContains: "endpoint address cannot be empty",
		},
		{
			name: "Success - Load Balancer endpoint with no port defaults to 80",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: "localhost"
`,
			expectedError: false,
			check: func(t *testing.T, services map[string]*ServiceConfig) {
				t.Helper()
				require.Contains(t, services, "user-service")
				svc := services["user-service"]
				require.NotNil(t, svc.LoadBalancer)
				require.Len(t, svc.LoadBalancer.Endpoints, 1)
				assert.Equal(t, "localhost", svc.LoadBalancer.Endpoints[0].Address)
				require.NotNil(t, svc.LoadBalancer.Endpoints[0].Port)
				assert.Equal(t, 80, *svc.LoadBalancer.Endpoints[0].Port)
			},
		},
		{
			name: "Failure - Load Balancer endpoint with invalid port (zero)",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: "localhost"
          port: 0
`,
			expectedError: true,
			errorContains: "should specify valid port ranges",
		},
		{
			name: "Failure - Load Balancer endpoint with invalid port (negative)",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: "localhost"
          port: -1
`,
			expectedError: true,
			errorContains: "should specify valid port ranges",
		},
		{
			name: "Failure - Load Balancer endpoint with invalid port (too high)",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: "localhost"
          port: 65536
`,
			expectedError: true,
			errorContains: "should specify valid port ranges",
		},
		{
			name: "Success - Valid Load Balancer configuration",
			yamlContent: `
services:
  user-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 10s
    load_balancer:
      endpoints:
        - address: "user-service-1"
          port: 8080
        - address: "user-service-2"
          port: 8081
`,
			expectedError: false,
			check: func(t *testing.T, services map[string]*ServiceConfig) {
				t.Helper()
				require.Contains(t, services, "user-service")
				svc := services["user-service"]
				require.NotNil(t, svc.LoadBalancer)
				require.Len(t, svc.LoadBalancer.Endpoints, 2)
				assert.Equal(t, "user-service-1", svc.LoadBalancer.Endpoints[0].Address)
				require.NotNil(t, svc.LoadBalancer.Endpoints[0].Port)
				assert.Equal(t, 8080, *svc.LoadBalancer.Endpoints[0].Port)
				assert.Equal(t, "user-service-2", svc.LoadBalancer.Endpoints[1].Address)
				require.NotNil(t, svc.LoadBalancer.Endpoints[1].Port)
				assert.Equal(t, 8081, *svc.LoadBalancer.Endpoints[1].Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "services_*.yml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.yamlContent)
			require.NoError(t, err)
			tmpFile.Close()

			services, err := LoadServicesFromFile(tmpFile.Name())

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, services)
				if tt.check != nil {
					tt.check(t, services)
				}
			}
		})
	}
}
