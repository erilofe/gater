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
			TargetURL:   mockUserSrv.URL,
		},
		{
			ServiceName: "post-service",
			Prefix:      "/posts",
			TargetURL:   mockPostSrv.URL,
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
		{ServiceName: "user-service", Prefix: "/users", TargetURL: userURL},
		{ServiceName: "post-service", Prefix: "/posts", TargetURL: postURL},
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