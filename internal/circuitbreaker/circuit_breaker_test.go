package circuitbreaker

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

// MockRoundTripper is a helper to mock http.RoundTripper
type MockRoundTripper struct {
	Response *http.Response
	Err      error
}

func (m *MockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

var defaultSettings = Settings{
	MaxRequests: 1,
	Interval:    10 * time.Second,
	Timeout:     30 * time.Second,
}

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test-cb", defaultSettings)
	assert.NotNil(t, cb)
	assert.Equal(t, "test-cb", cb.Name())
}

func TestTransport_RoundTrip_Success(t *testing.T) {
	// Setup
	cb := NewCircuitBreaker("cb-success", defaultSettings)
	mockRT := &MockRoundTripper{
		Response: &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
		},
		Err: nil,
	}
	transport := NewTransport(cb, mockRT)

	// Execute
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := transport.RoundTrip(req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, uint32(1), cb.Counts().Requests)
	assert.Equal(t, uint32(1), cb.Counts().TotalSuccesses)
	assert.Equal(t, uint32(0), cb.Counts().TotalFailures)
}

func TestTransport_RoundTrip_5xxError(t *testing.T) {
	// Setup
	cb := NewCircuitBreaker("cb-500", defaultSettings)
	mockRT := &MockRoundTripper{
		Response: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
		},
		Err: nil,
	}
	transport := NewTransport(cb, mockRT)

	// Execute
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := transport.RoundTrip(req)

	// Assert
	// The transport implementation returns nil, http.ErrHandlerTimeout (or similar) wrapped in the CB error
	// when a 5xx is encountered because the Execute func returns an error.
	assert.Error(t, err)

	// Because 1 failure / 1 request = 100% > 60%, the breaker trips immediately.
	// Gobreaker resets counts on state change, so we check state instead of counts.
	assert.Equal(t, gobreaker.StateOpen, cb.State())
}

func TestTransport_RoundTrip_NetworkError(t *testing.T) {
	// Setup
	cb := NewCircuitBreaker("cb-net-err", defaultSettings)
	expectedErr := errors.New("network error")
	mockRT := &MockRoundTripper{
		Response: nil,
		Err:      expectedErr,
	}
	transport := NewTransport(cb, mockRT)

	// Execute
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := transport.RoundTrip(req)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)

	// Because 1 failure / 1 request = 100% > 60%, the breaker trips immediately.
	assert.Equal(t, gobreaker.StateOpen, cb.State())
}

func TestCircuitBreaker_TripsOpen(t *testing.T) {
	// Setup
	cb := NewCircuitBreaker("cb-tripping", defaultSettings)
	// Returns 500 to trigger failures
	mockRT := &MockRoundTripper{
		Response: &http.Response{
			StatusCode: http.StatusInternalServerError,
		},
		Err: nil,
	}
	transport := NewTransport(cb, mockRT)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	// Execute: Trip the breaker (need > 3 consecutive failures)
	for range 4 {
		_, err := transport.RoundTrip(req)
		assert.Error(t, err)
	}

	// Verify state is OPEN
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Execute: Next request should fail immediately without calling RoundTrip
	// We change the mock to return success to prove it's not called
	mockRT.Response = &http.Response{StatusCode: http.StatusOK}
	_, err := transport.RoundTrip(req)

	// Assert
	assert.Error(t, err)
	assert.IsType(t, gobreaker.ErrOpenState, err)
}
