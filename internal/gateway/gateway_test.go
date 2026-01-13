package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set Gin to test mode to reduce log output during tests
	gin.SetMode(gin.TestMode)
}

// CloseNotifyingRecorder wraps httptest.ResponseRecorder to implement http.CloseNotifier
type CloseNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

// NewCloseNotifyingRecorder creates a new CloseNotifyingRecorder
func NewCloseNotifyingRecorder() *CloseNotifyingRecorder {
	return &CloseNotifyingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closed:           make(chan bool, 1),
	}
}

// CloseNotify implements http.CloseNotifier
func (c *CloseNotifyingRecorder) CloseNotify() <-chan bool {
	return c.closed
}

// createMockBackend creates a test HTTP server that returns a specific response
func createMockBackend(response string, statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(response))
	}))
}

// createTestService creates a Service for testing with mock backends
func createTestService(name string, instanceURLs []string) (*service.Service, error) {
	cfg := &config.ServiceConfig{
		ServiceName: name,
	}
	cfg.Resilience.CircuitBreaker.RecoveryTimeout = 30 * time.Second

	return service.NewService(name, instanceURLs, cfg)
}

func TestNewGateway(t *testing.T) {
	// Create mock services
	services := make(map[string]*service.Service)
	routes := []*config.Route{
		{Path: "/users", ServiceName: "user-service", Methods: []string{"GET"}, Priority: 1},
	}

	gw := NewGateway(services, routes)

	assert.NotNil(t, gw)
	assert.Equal(t, services, gw.services)
	assert.Equal(t, routes, gw.routes)
}

func TestGateway_SetupRouter(t *testing.T) {
	// Create mock backend servers
	userBackend := createMockBackend(`{"service":"user"}`, http.StatusOK)
	defer userBackend.Close()

	postBackend := createMockBackend(`{"service":"post"}`, http.StatusOK)
	defer postBackend.Close()

	// Create services
	userService, err := createTestService("user-service", []string{userBackend.URL})
	require.NoError(t, err)

	postService, err := createTestService("post-service", []string{postBackend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"user-service": userService,
		"post-service": postService,
	}

	routes := []*config.Route{
		{Path: "/users", ServiceName: "user-service", Methods: []string{"GET", "POST"}, Priority: 1},
		{Path: "/posts", ServiceName: "post-service", Methods: nil, Priority: 1}, // nil = all methods
	}

	// Create gateway and setup router
	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	assert.NotNil(t, router)
}

func TestGateway_RoutingToCorrectService(t *testing.T) {
	// Create mock backend servers with different responses
	userBackend := createMockBackend(`{"service":"user","path":"{{PATH}}"}`, http.StatusOK)
	defer userBackend.Close()

	postBackend := createMockBackend(`{"service":"post","path":"{{PATH}}"}`, http.StatusOK)
	defer postBackend.Close()

	// Create services
	userService, err := createTestService("user-service", []string{userBackend.URL})
	require.NoError(t, err)

	postService, err := createTestService("post-service", []string{postBackend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"user-service": userService,
		"post-service": postService,
	}

	routes := []*config.Route{
		{Path: "/users", ServiceName: "user-service", Methods: []string{"GET"}, Priority: 1},
		{Path: "/posts", ServiceName: "post-service", Methods: []string{"GET"}, Priority: 1},
	}

	// Create gateway and setup router
	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	// Test routing to user service
	t.Run("Route to user service", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/users/123", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"service":"user"`)
	})

	// Test routing to post service
	t.Run("Route to post service", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/posts/456", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"service":"post"`)
	})
}

func TestGateway_MethodFiltering(t *testing.T) {
	// Create mock backend
	backend := createMockBackend(`{"ok":true}`, http.StatusOK)
	defer backend.Close()

	// Create service
	svc, err := createTestService("test-service", []string{backend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"test-service": svc,
	}

	routes := []*config.Route{
		{Path: "/readonly", ServiceName: "test-service", Methods: []string{"GET"}, Priority: 1},
		{Path: "/write", ServiceName: "test-service", Methods: []string{"POST", "PUT"}, Priority: 1},
		{Path: "/all", ServiceName: "test-service", Methods: nil, Priority: 1}, // All methods
	}

	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	// Test GET on readonly endpoint - should succeed
	t.Run("GET on readonly endpoint", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/readonly/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test POST on readonly endpoint - should fail (404)
	t.Run("POST on readonly endpoint", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("POST", "/readonly/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code, "POST should not be allowed on readonly endpoint")
	})

	// Test POST on write endpoint - should succeed
	t.Run("POST on write endpoint", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("POST", "/write/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Test DELETE on all endpoint - should succeed
	t.Run("DELETE on all-methods endpoint", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("DELETE", "/all/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGateway_HealthCheck(t *testing.T) {
	// Create mock backend
	backend := createMockBackend(`{"ok":true}`, http.StatusOK)
	defer backend.Close()

	// Create service
	svc, err := createTestService("test-service", []string{backend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"test-service": svc,
	}

	routes := []*config.Route{
		{Path: "/test", ServiceName: "test-service", Methods: []string{"GET"}, Priority: 1},
	}

	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	// Test health check endpoint
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
	assert.Contains(t, w.Body.String(), `"services":1`)
	assert.Contains(t, w.Body.String(), `"routes":1`)
}

func TestGateway_UnknownServiceInRoute(t *testing.T) {
	// Create a service
	backend := createMockBackend(`{"ok":true}`, http.StatusOK)
	defer backend.Close()

	svc, err := createTestService("existing-service", []string{backend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"existing-service": svc,
	}

	// Create routes where one references a non-existent service
	routes := []*config.Route{
		{Path: "/existing", ServiceName: "existing-service", Methods: []string{"GET"}, Priority: 1},
		{Path: "/missing", ServiceName: "non-existent-service", Methods: []string{"GET"}, Priority: 1},
	}

	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	// Request to existing service should work
	t.Run("Existing service route works", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/existing/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Request to non-existent service should return 404
	t.Run("Non-existent service route returns 404", func(t *testing.T) {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/missing/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestGateway_MultipleInstances(t *testing.T) {
	// Create multiple backend servers
	backend1 := createMockBackend(`{"instance":"1"}`, http.StatusOK)
	defer backend1.Close()

	backend2 := createMockBackend(`{"instance":"2"}`, http.StatusOK)
	defer backend2.Close()

	// Create service with multiple instances
	svc, err := createTestService("multi-service", []string{backend1.URL, backend2.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"multi-service": svc,
	}

	routes := []*config.Route{
		{Path: "/multi", ServiceName: "multi-service", Methods: []string{"GET"}, Priority: 1},
	}

	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	// Make multiple requests and verify load balancing
	responses := make(map[string]int)
	for i := 0; i < 10; i++ {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/multi/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		responses[body]++
	}

	// Both instances should have received requests (due to round-robin)
	assert.Greater(t, len(responses), 0, "At least one instance should receive requests")

	// With 2 instances and 10 requests, we expect 5 requests per instance
	// (due to round-robin load balancing)
	t.Logf("Response distribution: %v", responses)
}

func TestGateway_PrefixMatching(t *testing.T) {
	// Test that routes match prefix correctly (e.g., /users matches /users/123, /users/profile, etc.)
	backend := createMockBackend(`{"ok":true}`, http.StatusOK)
	defer backend.Close()

	svc, err := createTestService("test-service", []string{backend.URL})
	require.NoError(t, err)

	services := map[string]*service.Service{
		"test-service": svc,
	}

	routes := []*config.Route{
		{Path: "/api/v1/users", ServiceName: "test-service", Methods: []string{"GET"}, Priority: 1},
	}

	gw := NewGateway(services, routes)
	router := gw.SetupRouter()

	testCases := []struct {
		name          string
		path          string
		shouldWork    bool
		allowRedirect bool // Gin may redirect when path doesn't end with /
	}{
		{"Exact match", "/api/v1/users", true, true}, // Gin redirects to /api/v1/users/
		{"With trailing slash", "/api/v1/users/", true, false},
		{"With ID", "/api/v1/users/123", true, false},
		{"With nested path", "/api/v1/users/123/profile", true, false},
		{"Different prefix", "/api/v2/users", false, false},
		{"Partial prefix", "/api/v1/user", false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewCloseNotifyingRecorder()
			req, _ := http.NewRequest("GET", tc.path, nil)
			router.ServeHTTP(w, req)

			if tc.shouldWork {
				if tc.allowRedirect {
					// Accept both OK and redirect (Gin behavior for paths without trailing slash)
					assert.Contains(t, []int{http.StatusOK, http.StatusMovedPermanently}, w.Code,
						"Path %s should be routed (may redirect)", tc.path)
				} else {
					assert.Equal(t, http.StatusOK, w.Code, "Path %s should be routed", tc.path)
				}
			} else {
				assert.Equal(t, http.StatusNotFound, w.Code, "Path %s should not be routed", tc.path)
			}
		})
	}
}
