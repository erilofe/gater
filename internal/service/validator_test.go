package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pietroagazzi/gater/internal/config"
)

func createValidator(alloweWebsocket bool) *Validator {
	return NewValidator(&config.ValidatorConfig{
		AllowWebsocket: &alloweWebsocket,
	})
}

func mockWebsocketRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "test_websocket_key")
	req.Header.Set("Sec-WebSocket-Version", "13")
	return req
}

func TestValidator_Handle(t *testing.T) {
	// Validators
	var (
		allowWebsocketValidator    = createValidator(true)
		disallowWebsocketValidator = createValidator(false)
	)

	// Test cases
	testCases := []struct {
		name               string
		validator          *Validator
		req                *http.Request
		expectedStatusCode int
		expectedAllowed    bool
	}{
		{
			name:               "Websocket allowed and request is websocket",
			validator:          allowWebsocketValidator,
			req:                mockWebsocketRequest(),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
		{
			name:               "Websocket not allowed and request is websocket",
			validator:          disallowWebsocketValidator,
			req:                mockWebsocketRequest(),
			expectedStatusCode: http.StatusForbidden,
			expectedAllowed:    false,
		},
		{
			name:               "Websocket not allowed and request is plain HTTP",
			validator:          allowWebsocketValidator,
			req:                httptest.NewRequest(http.MethodGet, "/", nil),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
		{
			name:               "Websocket allowed and request is plain HTTP",
			validator:          allowWebsocketValidator,
			req:                httptest.NewRequest(http.MethodGet, "/", nil),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
		{
			name:      "Websocket not allowed, partial websocket headers (missing key)",
			validator: disallowWebsocketValidator,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Upgrade", "websocket")
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Sec-WebSocket-Version", "13")
				return req
			}(),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
		{
			name:      "Websocket not allowed, partial websocket headers (missing version)",
			validator: disallowWebsocketValidator,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Upgrade", "websocket")
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
				return req
			}(),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
		{
			name:      "Websocket not allowed, partial websocket headers (missing upgrade)",
			validator: disallowWebsocketValidator,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Connection", "Upgrade")
				req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
				req.Header.Set("Sec-WebSocket-Version", "13")
				return req
			}(),
			expectedStatusCode: http.StatusOK,
			expectedAllowed:    true,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			allowed := tc.validator.Handle(w, tc.req)

			// Validate
			assert.Equal(t, tc.expectedAllowed, allowed, "Validator handle result mismatch")

			if !tc.expectedAllowed {
				assert.Equal(t, tc.expectedStatusCode, w.Code, "HTTP status code mismatch")
				assert.Contains(t, w.Body.String(), "Websocket connections are not allowed.", "Response body mismatch")
				assert.Equal(t, "close", w.Header().Get("Connection"), "Connection header mismatch")
			} else {
				// If allowed, the recorder should not have been written to by the validator
				assert.Equal(t, http.StatusOK, w.Code, "HTTP status code should not be modified for allowed requests")
			}
		})
	}
}
