package server

import (
	"net/http"
	"net/http/httptest"
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
	router.Any("/*path", Proxy(targetServer.URL))

	// 3. Create a Request to the Proxy
	w := NewCloseNotifyingRecorder()
	req, _ := http.NewRequest("GET", "/some/path", nil)

	// 4. Perform the Request
	router.ServeHTTP(w, req)

	// 5. Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, mockResponse, w.Body.String())
}
