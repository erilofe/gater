package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/service"
)

// mockConsulProvider implements a mock discovery provider for testing
type mockConsulProvider struct {
	services map[string][]string
}

func (m *mockConsulProvider) ResolveService(_ context.Context, serviceName string) ([]string, error) {
	if urls, exists := m.services[serviceName]; exists {
		return urls, nil
	}
	return nil, fmt.Errorf("service %s not found", serviceName)
}

func (m *mockConsulProvider) Name() string {
	return "mock"
}

func (m *mockConsulProvider) Close() error {
	return nil
}

// createTempConfigFiles creates temporary routes.yml and services.yml files
func createTempConfigFiles(t *testing.T, routesContent, servicesContent string) (string, string) {
	t.Helper()
	// Create temp directory
	tempDir := t.TempDir()

	// Create routes.yml
	routesPath := filepath.Join(tempDir, "routes.yml")
	err := os.WriteFile(routesPath, []byte(routesContent), 0644)
	require.NoError(t, err)

	// Create services.yml
	servicesPath := filepath.Join(tempDir, "services.yml")
	err = os.WriteFile(servicesPath, []byte(servicesContent), 0644)
	require.NoError(t, err)

	return routesPath, servicesPath
}

func TestCollectUniqueServiceNames(t *testing.T) {
	tests := []struct {
		name     string
		routes   []*config.Route
		expected []string
	}{
		{
			name: "Single service",
			routes: []*config.Route{
				{Path: "/users", ServiceName: "user-service"},
			},
			expected: []string{"user-service"},
		},
		{
			name: "Multiple unique services",
			routes: []*config.Route{
				{Path: "/users", ServiceName: "user-service"},
				{Path: "/posts", ServiceName: "post-service"},
				{Path: "/comments", ServiceName: "comment-service"},
			},
			expected: []string{"user-service", "post-service", "comment-service"},
		},
		{
			name: "Duplicate service names",
			routes: []*config.Route{
				{Path: "/users", ServiceName: "user-service"},
				{Path: "/users/admin", ServiceName: "user-service"},
				{Path: "/posts", ServiceName: "post-service"},
			},
			expected: []string{"user-service", "post-service"},
		},
		{
			name:     "Empty routes",
			routes:   []*config.Route{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collectUniqueServiceNames(tt.routes)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestLoadConfiguration_Success(t *testing.T) {
	routesContent := `
routes:
  - path: /test
    service_name: test-service
    methods: [GET]
`

	servicesContent := `
services:
  test-service:
    service_name: test-service
    resilience:
      circuit_breaker:
        recovery_timeout: 30s
`

	routesPath, servicesPath := createTempConfigFiles(t, routesContent, servicesContent)

	cfg := &config.Config{
		RoutesConfigPath:   routesPath,
		ServicesConfigPath: servicesPath,
	}

	routes, servicesConfig, err := loadConfiguration(cfg)

	require.NoError(t, err)
	assert.Len(t, routes, 1)
	assert.Equal(t, "/test", routes[0].Path)
	assert.Len(t, servicesConfig, 1)
	assert.Contains(t, servicesConfig, "test-service")
}

func TestLoadConfiguration_InvalidRoutesFile(t *testing.T) {
	routesContent := `
routes:
  - invalid yaml syntax
    missing colons
`

	servicesContent := `
services:
  test-service:
    resilience:
      circuit_breaker:
        recovery_timeout: 30s
`

	routesPath, servicesPath := createTempConfigFiles(t, routesContent, servicesContent)

	cfg := &config.Config{
		RoutesConfigPath:   routesPath,
		ServicesConfigPath: servicesPath,
	}

	_, _, err := loadConfiguration(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load routes config")
}

func TestLoadConfiguration_InvalidServicesFile(t *testing.T) {
	routesContent := `
routes:
  - path: /test
    service_name: test-service
`

	servicesContent := `
services:
  test-service:
    resilience:
      circuit_breaker: {}
`

	routesPath, servicesPath := createTempConfigFiles(t, routesContent, servicesContent)

	cfg := &config.Config{
		RoutesConfigPath:   routesPath,
		ServicesConfigPath: servicesPath,
	}

	_, _, err := loadConfiguration(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load services config")
}

func TestBuildServices_Success(t *testing.T) {
	// Create mock backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer backend.Close()

	// Create mock provider
	mockProvider := &mockConsulProvider{
		services: map[string][]string{
			"test-service": {backend.URL},
		},
	}

	// Create routes and services config
	routes := []*config.Route{
		{Path: "/test", ServiceName: "test-service"},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"test-service": {
			ServiceName: "test-service",
		},
	}
	servicesConfig["test-service"].Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second
	servicesConfig["test-service"].Validator = &config.ValidatorConfig{
		AllowWebsocket: new(bool),
	}
	*servicesConfig["test-service"].Validator.AllowWebsocket = false

	ctx := context.Background()
	services, err := buildServices(ctx, routes, servicesConfig, mockProvider, "test.yml")

	require.NoError(t, err)
	assert.Len(t, services, 1)
	assert.Contains(t, services, "test-service")
	assert.Len(t, services["test-service"].GetInstances(), 1)
}

func TestBuildServices_ServiceNotInConfig(t *testing.T) {
	mockProvider := &mockConsulProvider{
		services: map[string][]string{
			"existing-service": {"http://localhost:8080"},
		},
	}

	routes := []*config.Route{
		{Path: "/test", ServiceName: "missing-service"},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"existing-service": {
			ServiceName: "existing-service",
		},
	}

	ctx := context.Background()
	_, err := buildServices(ctx, routes, servicesConfig, mockProvider, "test.yml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing-service")
	assert.Contains(t, err.Error(), "not found in")
}

func TestBuildServices_ServiceNotFoundInDiscovery(t *testing.T) {
	mockProvider := &mockConsulProvider{
		services: map[string][]string{},
	}

	routes := []*config.Route{
		{Path: "/test", ServiceName: "test-service"},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"test-service": {
			ServiceName: "test-service",
		},
	}
	servicesConfig["test-service"].Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second

	ctx := context.Background()
	_, err := buildServices(ctx, routes, servicesConfig, mockProvider, "test.yml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestValidateRoutes_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	mockProvider := &mockConsulProvider{
		services: map[string][]string{
			"test-service": {backend.URL},
		},
	}

	routes := []*config.Route{
		{Path: "/test", ServiceName: "test-service"},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"test-service": {
			ServiceName: "test-service",
		},
	}
	servicesConfig["test-service"].Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second
	servicesConfig["test-service"].Validator = &config.ValidatorConfig{
		AllowWebsocket: new(bool),
	}
	*servicesConfig["test-service"].Validator.AllowWebsocket = false

	ctx := context.Background()
	services, err := buildServices(ctx, routes, servicesConfig, mockProvider, "test.yml")
	require.NoError(t, err)

	err = validateRoutes(routes, services)
	assert.NoError(t, err)
}

func TestValidateRoutes_MissingService(t *testing.T) {
	routes := []*config.Route{
		{Path: "/test", ServiceName: "missing-service"},
	}

	services := map[string]*service.Service{}

	err := validateRoutes(routes, services)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing-service")
	assert.Contains(t, err.Error(), "unknown service")
}

func TestResolveAndCreateService_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	mockProvider := &mockConsulProvider{
		services: map[string][]string{
			"test-service": {backend.URL},
		},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"test-service": {
			ServiceName: "test-service",
		},
	}
	servicesConfig["test-service"].Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second
	servicesConfig["test-service"].Validator = &config.ValidatorConfig{
		AllowWebsocket: new(bool),
	}
	*servicesConfig["test-service"].Validator.AllowWebsocket = false

	ctx := context.Background()
	svc, err := resolveAndCreateService(ctx, "test-service", servicesConfig, mockProvider, "test.yml")

	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, "test-service", svc.Name)
	assert.Len(t, svc.GetInstances(), 1)
}

func TestResolveAndCreateService_NotInConfig(t *testing.T) {
	mockProvider := &mockConsulProvider{
		services: map[string][]string{},
	}

	servicesConfig := map[string]*config.ServiceConfig{}

	ctx := context.Background()
	_, err := resolveAndCreateService(ctx, "missing-service", servicesConfig, mockProvider, "test.yml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in")
}

func TestResolveAndCreateService_NoInstances(t *testing.T) {
	mockProvider := &mockConsulProvider{
		services: map[string][]string{
			"test-service": {}, // Empty instances
		},
	}

	servicesConfig := map[string]*config.ServiceConfig{
		"test-service": {
			ServiceName: "test-service",
		},
	}
	servicesConfig["test-service"].Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second

	ctx := context.Background()
	_, err := resolveAndCreateService(ctx, "test-service", servicesConfig, mockProvider, "test.yml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no instances found")
}
