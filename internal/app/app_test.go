package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
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
// using local mock servers. This runs in isolation.
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

	// Setup Router
	gin.SetMode(gin.TestMode)
	router := SetupRouter(mockUserSrv.URL, mockPostSrv.URL)

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
// This is intended to run inside the Docker test container against the other containers.
func TestSetupRouter_Integration(t *testing.T) {
	userURL := os.Getenv("USER_SERVICE_URL")
	postURL := os.Getenv("POST_SERVICE_URL")

	if userURL == "" || postURL == "" {
		t.Skip("Skipping integration test: USER_SERVICE_URL or POST_SERVICE_URL not set")
	}

	gin.SetMode(gin.TestMode)
	router := SetupRouter(userURL, postURL)

	// Test User Service (Expects the http-echo response)
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/users/test", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	// The http-echo service returns exactly what we configured in docker-compose.test.yml
	assert.Contains(t, w.Body.String(), `"service":"user"`)

	// Test Post Service
	w = NewCloseNotifyingRecorder()
	req, _ = http.NewRequest("GET", "/posts/test", nil)
	router.ServeHTTP(w, req)
	
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"service":"post"`)
}
