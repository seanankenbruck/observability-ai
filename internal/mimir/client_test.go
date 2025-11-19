// internal/mimir/client_test.go
package mimir

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client creation
func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		auth     AuthConfig
		timeout  time.Duration
	}{
		{
			name:     "basic auth client",
			endpoint: "http://localhost:9009",
			auth: AuthConfig{
				Type:     "basic",
				Username: "admin",
				Password: "password",
				TenantID: "tenant1",
			},
			timeout: 30 * time.Second,
		},
		{
			name:     "bearer auth client",
			endpoint: "http://localhost:9009",
			auth: AuthConfig{
				Type:        "bearer",
				BearerToken: "test-token",
				TenantID:    "tenant1",
			},
			timeout: 30 * time.Second,
		},
		{
			name:     "no auth client",
			endpoint: "http://localhost:9009",
			auth: AuthConfig{
				Type: "none",
			},
			timeout: 30 * time.Second,
		},
		{
			name:     "endpoint with trailing slash",
			endpoint: "http://localhost:9009/",
			auth: AuthConfig{
				Type: "none",
			},
			timeout: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.endpoint, tt.auth, tt.timeout)
			require.NotNil(t, client)
			assert.NotNil(t, client.httpClient)
			assert.Equal(t, tt.timeout, client.httpClient.Timeout)
			// Ensure trailing slash is removed (the endpoint should be trimmed)
			if tt.endpoint != "" {
				assert.Equal(t, "http://localhost:9009", client.endpoint)
			}
		})
	}
}

// TestClientQuery tests instant query functionality
func TestClientQuery(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		timestamp      time.Time
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful query",
			query:          "up",
			timestamp:      time.Time{},
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result": []map[string]interface{}{
						{
							"metric": map[string]string{
								"__name__": "up",
								"job":      "prometheus",
							},
							"value": []interface{}{float64(1234567890), "1"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:           "query with timestamp",
			query:          "up",
			timestamp:      time.Unix(1234567890, 0),
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result":     []interface{}{},
				},
			},
			wantErr: false,
		},
		{
			name:           "query with error response",
			query:          "invalid{query",
			timestamp:      time.Time{},
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status":    "error",
				"errorType": "bad_data",
				"error":     "parse error",
			},
			wantErr:     true,
			errContains: "query error",
		},
		{
			name:           "server error",
			query:          "up",
			timestamp:      time.Time{},
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			wantErr:        true,
			errContains:    "query failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{"/prometheus/api/v1/query", "/api/v1/query"}
				assert.Contains(t, validPaths, r.URL.Path)
				assert.Equal(t, tt.query, r.URL.Query().Get("query"))

				if !tt.timestamp.IsZero() {
					assert.Equal(t, fmt.Sprintf("%d", tt.timestamp.Unix()), r.URL.Query().Get("time"))
				}

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			resp, err := client.Query(ctx, tt.query, tt.timestamp)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "success", resp.Status)
			}
		})
	}
}

// TestClientQueryRange tests range query functionality
func TestClientQueryRange(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		start          time.Time
		end            time.Time
		step           time.Duration
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful range query",
			query:          "rate(http_requests_total[5m])",
			start:          time.Unix(1234567800, 0),
			end:            time.Unix(1234567890, 0),
			step:           15 * time.Second,
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "matrix",
					"result": []map[string]interface{}{
						{
							"metric": map[string]string{
								"__name__": "http_requests_total",
								"job":      "api",
							},
							"values": [][]interface{}{
								{float64(1234567800), "100"},
								{float64(1234567815), "150"},
								{float64(1234567830), "200"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:           "query range with error",
			query:          "invalid{query",
			start:          time.Unix(1234567800, 0),
			end:            time.Unix(1234567890, 0),
			step:           15 * time.Second,
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status":    "error",
				"errorType": "bad_data",
				"error":     "parse error",
			},
			wantErr:     true,
			errContains: "query_range error",
		},
		{
			name:           "server error",
			query:          "up",
			start:          time.Unix(1234567800, 0),
			end:            time.Unix(1234567890, 0),
			step:           15 * time.Second,
			responseStatus: http.StatusBadRequest,
			responseBody:   "Bad Request",
			wantErr:        true,
			errContains:    "query_range failed with status 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{"/prometheus/api/v1/query_range", "/api/v1/query_range"}
				assert.Contains(t, validPaths, r.URL.Path)
				assert.Equal(t, tt.query, r.URL.Query().Get("query"))
				assert.Equal(t, fmt.Sprintf("%d", tt.start.Unix()), r.URL.Query().Get("start"))
				assert.Equal(t, fmt.Sprintf("%d", tt.end.Unix()), r.URL.Query().Get("end"))
				assert.Equal(t, fmt.Sprintf("%d", int(tt.step.Seconds())), r.URL.Query().Get("step"))

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			resp, err := client.QueryRange(ctx, tt.query, tt.start, tt.end, tt.step)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "success", resp.Status)
			}
		})
	}
}

// TestClientGetMetricNames tests metric names retrieval
func TestClientGetMetricNames(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		expectedNames  []string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful metric names retrieval",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": []string{
					"up",
					"http_requests_total",
					"cpu_usage_percent",
					"memory_usage_bytes",
				},
			},
			expectedNames: []string{"up", "http_requests_total", "cpu_usage_percent", "memory_usage_bytes"},
			wantErr:       false,
		},
		{
			name:           "empty metric names",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data":   []string{},
			},
			expectedNames: []string{},
			wantErr:       false,
		},
		{
			name:           "error response",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "error",
				"data":   []string{},
			},
			wantErr:     true,
			errContains: "get metric names failed",
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			wantErr:        true,
			errContains:    "get metric names failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{"/prometheus/api/v1/label/__name__/values", "/api/v1/label/__name__/values"}
				assert.Contains(t, validPaths, r.URL.Path)

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			names, err := client.GetMetricNames(ctx)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedNames, names)
			}
		})
	}
}

// TestClientGetLabelValues tests label values retrieval
func TestClientGetLabelValues(t *testing.T) {
	tests := []struct {
		name           string
		labelName      string
		metricMatchers []string
		responseStatus int
		responseBody   interface{}
		expectedValues []string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful label values retrieval",
			labelName:      "job",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data":   []string{"prometheus", "node-exporter", "api"},
			},
			expectedValues: []string{"prometheus", "node-exporter", "api"},
			wantErr:        false,
		},
		{
			name:           "label values with matcher",
			labelName:      "job",
			metricMatchers: []string{`{__name__="up"}`},
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data":   []string{"prometheus"},
			},
			expectedValues: []string{"prometheus"},
			wantErr:        false,
		},
		{
			name:           "empty label values",
			labelName:      "nonexistent",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data":   []string{},
			},
			expectedValues: []string{},
			wantErr:        false,
		},
		{
			name:           "server error",
			labelName:      "job",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			wantErr:        true,
			errContains:    "get label values failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{
					fmt.Sprintf("/prometheus/api/v1/label/%s/values", tt.labelName),
					fmt.Sprintf("/api/v1/label/%s/values", tt.labelName),
				}
				assert.Contains(t, validPaths, r.URL.Path)
				if len(tt.metricMatchers) > 0 {
					assert.Equal(t, tt.metricMatchers[0], r.URL.Query().Get("match[]"))
				}

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			values, err := client.GetLabelValues(ctx, tt.labelName, tt.metricMatchers...)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValues, values)
			}
		})
	}
}

// TestClientGetMetricMetadata tests metric metadata retrieval
func TestClientGetMetricMetadata(t *testing.T) {
	tests := []struct {
		name             string
		metricName       string
		responseStatus   int
		responseBody     interface{}
		expectedMetadata *MetricMetadata
		wantErr          bool
	}{
		{
			name:           "successful metadata retrieval",
			metricName:     "http_requests_total",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": map[string][]map[string]interface{}{
					"http_requests_total": {
						{
							"type": "counter",
							"help": "Total HTTP requests",
							"unit": "requests",
						},
					},
				},
			},
			expectedMetadata: &MetricMetadata{
				Type: "counter",
				Help: "Total HTTP requests",
				Unit: "requests",
			},
			wantErr: false,
		},
		{
			name:           "metadata not available - fallback to inference",
			metricName:     "custom_metric_total",
			responseStatus: http.StatusNotFound,
			responseBody:   "Not Found",
			expectedMetadata: &MetricMetadata{
				Type: "counter", // Should infer from _total suffix
				Help: "",
				Unit: "",
			},
			wantErr: false,
		},
		{
			name:           "histogram metric inference",
			metricName:     "request_duration_seconds",
			responseStatus: http.StatusNotFound,
			responseBody:   "Not Found",
			expectedMetadata: &MetricMetadata{
				Type: "histogram", // Should infer from _duration
				Help: "",
				Unit: "",
			},
			wantErr: false,
		},
		{
			name:           "gauge metric inference",
			metricName:     "current_connections",
			responseStatus: http.StatusNotFound,
			responseBody:   "Not Found",
			expectedMetadata: &MetricMetadata{
				Type: "gauge", // Default fallback
				Help: "",
				Unit: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{"/prometheus/api/v1/metadata", "/api/v1/metadata"}
				assert.Contains(t, validPaths, r.URL.Path)
				if tt.metricName != "" {
					assert.Equal(t, tt.metricName, r.URL.Query().Get("metric"))
				}

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			metadata, err := client.GetMetricMetadata(ctx, tt.metricName)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMetadata.Type, metadata.Type)
				assert.Equal(t, tt.expectedMetadata.Help, metadata.Help)
				assert.Equal(t, tt.expectedMetadata.Unit, metadata.Unit)
			}
		})
	}
}

// TestClientTestConnection tests connection testing
func TestClientTestConnection(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   interface{}
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful connection",
			responseStatus: http.StatusOK,
			responseBody: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result":     []interface{}{},
				},
			},
			wantErr: false,
		},
		{
			name:           "connection failure",
			responseStatus: http.StatusServiceUnavailable,
			responseBody:   "Service Unavailable",
			wantErr:        true,
			errContains:    "connection test failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Accept both Mimir and Prometheus paths for testing
				validPaths := []string{"/prometheus/api/v1/query", "/api/v1/query"}
				assert.Contains(t, validPaths, r.URL.Path)
				assert.Equal(t, "up", r.URL.Query().Get("query"))

				w.WriteHeader(tt.responseStatus)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			ctx := context.Background()

			err := client.TestConnection(ctx)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestClientAuthentication tests various authentication mechanisms
func TestClientAuthentication(t *testing.T) {
	tests := []struct {
		name              string
		auth              AuthConfig
		expectedAuthType  string
		expectedUsername  string
		expectedPassword  string
		expectedBearer    string
		expectedTenantID  string
	}{
		{
			name: "basic authentication",
			auth: AuthConfig{
				Type:     "basic",
				Username: "admin",
				Password: "secret",
				TenantID: "tenant1",
			},
			expectedAuthType: "basic",
			expectedUsername: "admin",
			expectedPassword: "secret",
			expectedTenantID: "tenant1",
		},
		{
			name: "bearer token authentication",
			auth: AuthConfig{
				Type:        "bearer",
				BearerToken: "test-token-12345",
				TenantID:    "tenant2",
			},
			expectedAuthType: "bearer",
			expectedBearer:   "test-token-12345",
			expectedTenantID: "tenant2",
		},
		{
			name: "no authentication",
			auth: AuthConfig{
				Type: "none",
			},
			expectedAuthType: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authentication headers
				if tt.expectedAuthType == "basic" {
					username, password, ok := r.BasicAuth()
					assert.True(t, ok, "Basic auth should be present")
					assert.Equal(t, tt.expectedUsername, username)
					assert.Equal(t, tt.expectedPassword, password)
				} else if tt.expectedAuthType == "bearer" {
					authHeader := r.Header.Get("Authorization")
					assert.Contains(t, authHeader, "Bearer "+tt.expectedBearer)
				}

				// Verify tenant ID header
				if tt.expectedTenantID != "" {
					assert.Equal(t, tt.expectedTenantID, r.Header.Get("X-Scope-OrgID"))
				}

				// Return successful response
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"data": map[string]interface{}{
						"resultType": "vector",
						"result":     []interface{}{},
					},
				})
			}))
			defer server.Close()

			client := NewClient(server.URL, tt.auth, 5*time.Second)
			ctx := context.Background()

			_, err := client.Query(ctx, "up", time.Time{})
			require.NoError(t, err)
		})
	}
}

// TestInferMetricType tests metric type inference
func TestInferMetricType(t *testing.T) {
	tests := []struct {
		metricName   string
		expectedType string
	}{
		{"http_requests_total", "counter"},
		{"api_calls_count", "counter"},
		{"request_duration_seconds_bucket", "histogram"},
		{"request_duration_histogram", "histogram"},
		{"http_request_duration_seconds", "histogram"},
		{"response_time_milliseconds", "histogram"},
		{"api_latency_seconds", "histogram"},
		{"request_summary", "summary"},
		{"cpu_usage_percent", "gauge"},
		{"memory_bytes", "gauge"},
		{"current_connections", "gauge"},
	}

	for _, tt := range tests {
		t.Run(tt.metricName, func(t *testing.T) {
			result := inferMetricType(tt.metricName)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

// TestClientTimeout tests client timeout behavior
func TestClientTimeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   map[string]interface{}{},
		})
	}))
	defer server.Close()

	// Create client with 1 second timeout
	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 1*time.Second, BackendTypeMimir)
	ctx := context.Background()

	_, err := client.Query(ctx, "up", time.Time{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Client.Timeout")
}

// TestClientContextCancellation tests context cancellation handling
func TestClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"data":   map[string]interface{}{},
		})
	}))
	defer server.Close()

	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 10*time.Second, BackendTypeMimir)

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Query(ctx, "up", time.Time{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
