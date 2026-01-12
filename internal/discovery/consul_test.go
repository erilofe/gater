package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConsulServer creates a mock HTTP server that simulates basic Consul endpoints.
func mockConsulServer(t *testing.T, services map[string][]api.ServiceEntry) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/agent/self":
			// Health check for NewConsulProvider
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"Config": {"NodeName": "mock-node"}}`))

		default:
			// Check if it's a health service query
			// Pattern: /v1/health/service/{serviceName}
			prefix := "/v1/health/service/"
			if len(r.URL.Path) > len(prefix) && r.URL.Path[:len(prefix)] == prefix {
				serviceName := r.URL.Path[len(prefix):]
				
				entries, exists := services[serviceName]
				if !exists {
					// Return empty list if service not found, similar to Consul
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("[]"))
					return
				}

				if err := json.NewEncoder(w).Encode(entries); err != nil {
					t.Errorf("Failed to encode response: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}
			
			http.NotFound(w, r)
		}
	}))
}

func TestConsulProvider_ResolveService(t *testing.T) {
	// Define mock data
	mockServices := map[string][]api.ServiceEntry{
		"user-service": {
			{
				Service: &api.AgentService{
					Service: "user-service",
					Address: "10.0.0.1",
					Port:    8081,
				},
			},
		},
		"multi-service": {
			{
				Service: &api.AgentService{
					Service: "multi-service",
					Address: "10.0.0.2",
					Port:    9090,
				},
			},
			{
				Service: &api.AgentService{
					Service: "multi-service",
					Address: "10.0.0.3",
					Port:    9091,
				},
			},
		},
		"empty-address-service": {
			{
				Node: &api.Node{
					Address: "192.168.1.100",
				},
				Service: &api.AgentService{
					Service: "empty-address-service",
					Address: "", // Should fallback to Node.Address
					Port:    8082,
				},
			},
		},
	}

	server := mockConsulServer(t, mockServices)
	defer server.Close()

	// Initialize Provider pointing to mock server
	// httptest.Server URL starts with http://, we need to strip it for some clients 
	// but api.Config.Address expects "host:port" or "http://host:port". 
	// The standard lib consul client handles http:// prefix correctly in Address since some versions,
	// but let's just pass the URL as is.
	provider, err := NewConsulProvider(server.URL)
	require.NoError(t, err)
	defer provider.Close()

	tests := []struct {
		name          string
		serviceName   string
		expectedURL   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "Resolve User Service",
			serviceName: "user-service",
			expectedURL: "http://10.0.0.1:8081",
		},
		{
			name:        "Resolve Service Fallback to Node Address",
			serviceName: "empty-address-service",
			expectedURL: "http://192.168.1.100:8082",
		},
		{
			name:          "Service Not Found",
			serviceName:   "non-existent",
			expectError:   true,
			errorContains: "no healthy instances found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := provider.ResolveService(context.Background(), tt.serviceName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, url)
			}
		})
	}
}

func TestNewConsulProvider_ConnectionFailure(t *testing.T) {
	// Point to a closed port/invalid address
	_, err := NewConsulProvider("http://localhost:12345")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul unreachable")
}
