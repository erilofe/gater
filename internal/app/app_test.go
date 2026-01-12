package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/config"
	"github.com/pietroagazzi/gater/internal/discovery"
	"github.com/stretchr/testify/assert"
)

type CloseNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func NewCloseNotifyingRecorder() *CloseNotifyingRecorder {
	return &CloseNotifyingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closed:           make(chan bool, 1),
	}
}

func (c *CloseNotifyingRecorder) CloseNotify() <-chan bool {
	return c.closed
}

// TestSetupRouter_Unit verifies that the router routes traffic correctly
// using manually injected routes (simulating discovery).
func TestSetupRouter_Unit(t *testing.T) {
	// Mock User Service
	mockUserSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"user"}`))
	}))
	defer mockUserSrv.Close()

	// Mock Post Service
	mockPostSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"post"}`))
	}))
	defer mockPostSrv.Close()

	// Setup Config
	cfg := &config.Config{
		CircuitBreaker: config.CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    10 * time.Second,
			Timeout:     30 * time.Second,
		},
	}

	// Simulated Discovered Routes
	routes := []discovery.ServiceRoute{
		{
			ServiceName: "user-service",
			Prefix:      "/users",
			TargetURLs:  []string{mockUserSrv.URL},
		},
		{
			ServiceName: "post-service",
			Prefix:      "/posts",
			TargetURLs:  []string{mockPostSrv.URL},
		},
	}

	// Setup Router
	gin.SetMode(gin.TestMode)
	router := SetupRouter(cfg, routes)

	// Test /users/ route
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/users/profile", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"service":"user"}`, w.Body.String())

	// Test /posts/ route
	w = NewCloseNotifyingRecorder()
	req, _ = http.NewRequest("GET", "/posts/latest", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"service":"post"}`, w.Body.String())
}

func TestSetupRouter_MethodFiltering(t *testing.T) {
	// Mock Service
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockSrv.Close()

	cfg := &config.Config{} // Default config

	routes := []discovery.ServiceRoute{
		{
			ServiceName: "readonly-service",
			Prefix:      "/readonly",
			TargetURLs:  []string{mockSrv.URL},
			Methods:     []string{"GET"},
		},
		{
			ServiceName: "mixed-service",
			Prefix:      "/mixed",
			TargetURLs:  []string{mockSrv.URL},
			Methods:     []string{"GET", "POST"},
		},
		{
			ServiceName: "all-service",
			Prefix:      "/all",
			TargetURLs:  []string{mockSrv.URL},
			Methods:     nil, // Should default to Any
		},
	}

	gin.SetMode(gin.TestMode)
	router := SetupRouter(cfg, routes)

	// Case 1: GET Allowed
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/readonly/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "GET should be allowed on /readonly")

	// Case 2: POST Denied (Gin defaults to 404 for unmatched method/path combo)
	w = NewCloseNotifyingRecorder()
	req, _ = http.NewRequest("POST", "/readonly/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "POST should be denied (404) on /readonly")

	// Case 3: Mixed Allowed
	w = NewCloseNotifyingRecorder()
	req, _ = http.NewRequest("POST", "/mixed/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "POST should be allowed on /mixed")

	// Case 4: Default All Allowed
	w = NewCloseNotifyingRecorder()
	req, _ = http.NewRequest("DELETE", "/all/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "DELETE should be allowed on /all (default)")
}

// TestSetupRouter_Integration runs only if the environment variables for services are set.
// It verifies that we can pass URLs from env vars into the router configuration manually.
func TestSetupRouter_Integration(t *testing.T) {
	userURL := os.Getenv("USER_SERVICE_URL")
	postURL := os.Getenv("POST_SERVICE_URL")

	if userURL == "" || postURL == "" {
		t.Skip("Skipping integration test: USER_SERVICE_URL or POST_SERVICE_URL not set")
	}

	cfg := &config.Config{
		CircuitBreaker: config.CircuitBreakerConfig{
			MaxRequests: 1,
			Interval:    10 * time.Second,
			Timeout:     30 * time.Second,
		},
	}

	routes := []discovery.ServiceRoute{
		{ServiceName: "user-service", Prefix: "/users", TargetURLs: []string{userURL}},
		{ServiceName: "post-service", Prefix: "/posts", TargetURLs: []string{postURL}},
	}

	gin.SetMode(gin.TestMode)
	router := SetupRouter(cfg, routes)

	// Test User Service
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/users/test", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"service":"user"`)
}