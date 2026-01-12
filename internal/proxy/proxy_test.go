package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pietroagazzi/gater/internal/circuitbreaker"
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

var defaultSettings = circuitbreaker.Settings{
	MaxRequests: 1,
	Interval:    10 * time.Second,
	Timeout:     30 * time.Second,
}

func TestProxy(t *testing.T) {
	// 1. Setup a Mock Target Server
	// This server represents the "User Service" or "Post Service"
	mockResponse := `{"message": "success"}`
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that the proxy forwarded the correct path
		if r.URL.Path != "/some/path" {
			t.Errorf("Expected path /some/path, got %s", r.URL.Path)
		}
		// Verify headers if needed
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer targetServer.Close()

	// 2. Setup Gin Router with the Proxy
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/*path", Proxy([]string{targetServer.URL}, "test-cb", defaultSettings))

	// 3. Create a Request to the Proxy
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/some/path", nil)

	// 4. Perform the Request
	router.ServeHTTP(w, req)

	// 5. Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, mockResponse, w.Body.String())
}

func TestProxy_CircuitBreaker_Integration(t *testing.T) {
	// 1. Setup a Mock Target Server that always fails
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer targetServer.Close()

	// 2. Setup Gin Router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/*path", Proxy([]string{targetServer.URL}, "test-cb-integration", defaultSettings))

	// 3. First Request: Should fail with 502 (Bad Gateway) because the server returns 500
	// This failure triggers the Circuit Breaker to open because failure ratio 1/1 = 100% > 60%
	w1 := NewCloseNotifyingRecorder()
	req1, _ := http.NewRequest("GET", "/fail", nil)
	router.ServeHTTP(w1, req1)

	// Note: 502 is returned by our proxy ErrorHandler when not OpenState
	assert.Equal(t, http.StatusBadGateway, w1.Code)

	// 4. Second Request: Should fail with 503 (Service Unavailable) immediately
	// because the Circuit Breaker is now OPEN.
	w2 := NewCloseNotifyingRecorder()
	req2, _ := http.NewRequest("GET", "/fail", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Contains(t, w2.Body.String(), "Service unavailable")
}

func TestProxy_CircuitBreaker_ServiceUnreachable(t *testing.T) {
	// 1. Setup a Mock Target Server and immediately close it to simulate "Service Down"
	// We get a valid URL, but nothing will be listening on it.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	targetURL := targetServer.URL
	targetServer.Close() // Simulate service down (Connection Refused)

	// 2. Setup Gin Router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/*path", Proxy([]string{targetURL}, "test-cb-unreachable", defaultSettings))

	// 3. First Request: Should fail with 502 (Bad Gateway)
	// The underlying transport returns an error (dial tcp ...: connect: connection refused)
	// The Circuit Breaker counts this as a failure.
	w1 := NewCloseNotifyingRecorder()
	req1, _ := http.NewRequest("GET", "/fail", nil)
	router.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusBadGateway, w1.Code)
	assert.Contains(t, w1.Body.String(), "Bad gateway")

	// 4. Second Request: Should fail with 503 (Service Unavailable)
	// Because failure ratio > 60% (1/1 failures), the Circuit Breaker trips to OPEN.
	w2 := NewCloseNotifyingRecorder()
	req2, _ := http.NewRequest("GET", "/fail", nil)
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Contains(t, w2.Body.String(), "Service unavailable")
}

func TestProxy_LoadBalancing(t *testing.T) {
	// Track which server handled each request
	server1Calls := 0
	server2Calls := 0

	// Create two mock servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server1Calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"server": "1"}`))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server2Calls++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"server": "2"}`))
	}))
	defer server2.Close()

	// Setup Gin Router with load balancing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Any("/*path", Proxy([]string{server1.URL, server2.URL}, "test-lb", defaultSettings))

	// Make 10 requests
	for i := 0; i < 10; i++ {
		w := NewCloseNotifyingRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Verify distribution (should be equal - 5 calls to each server)
	assert.Equal(t, 5, server1Calls, "Expected 5 calls to server1")
	assert.Equal(t, 5, server2Calls, "Expected 5 calls to server2")
}
