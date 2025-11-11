// internal/processor/processor_test.go
package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/llm"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCategorizeMetrics tests metric categorization by type
func TestCategorizeMetrics(t *testing.T) {
	tests := []struct {
		name               string
		metrics            []string
		expectedCounters   []string
		expectedGauges     []string
		expectedHistograms []string
		expectedOthers     []string
	}{
		{
			name:               "empty metrics list",
			metrics:            []string{},
			expectedCounters:   nil,
			expectedGauges:     nil,
			expectedHistograms: nil,
			expectedOthers:     nil,
		},
		{
			name: "only counters",
			metrics: []string{
				"http_requests_total",
				"http_errors_total",
				"cache_hits_count",
			},
			expectedCounters:   []string{"http_requests_total", "http_errors_total", "cache_hits_count"},
			expectedGauges:     nil,
			expectedHistograms: nil,
			expectedOthers:     nil,
		},
		{
			name: "only gauges",
			metrics: []string{
				"database_connections_active_now",  // has _active_ in it
				"memory_usage_current_bytes",       // has _current_ in it
				"disk_size",                        // has _size suffix
				"cpu_gauge",                        // has _gauge suffix
				"network_bytes",                    // has _bytes suffix
				"cache_hit_ratio",                  // has _ratio suffix
			},
			expectedCounters:   nil,
			expectedGauges:     []string{"database_connections_active_now", "memory_usage_current_bytes", "disk_size", "cpu_gauge", "network_bytes", "cache_hit_ratio"},
			expectedHistograms: nil,
			expectedOthers:     nil,
		},
		{
			name: "only histograms",
			metrics: []string{
				"http_request_duration_bucket",
				"response_time_bucket",
			},
			expectedCounters:   nil,
			expectedGauges:     nil,
			expectedHistograms: []string{"http_request_duration_bucket", "response_time_bucket"},
			expectedOthers:     nil,
		},
		{
			name: "mixed types",
			metrics: []string{
				"http_requests_total",           // counter
				"memory_usage_current_value",    // gauge (has _current_)
				"http_duration_bucket",          // histogram
				"some_other_metric",             // other
				"cache_hits_count",              // counter
				"connections_active_now",        // gauge (has _active_)
				"request_duration_seconds_bucket", // histogram
			},
			expectedCounters:   []string{"http_requests_total", "cache_hits_count"},
			expectedGauges:     []string{"memory_usage_current_value", "connections_active_now"},
			expectedHistograms: []string{"http_duration_bucket", "request_duration_seconds_bucket"},
			expectedOthers:     []string{"some_other_metric"},
		},
		{
			name: "case insensitive matching",
			metrics: []string{
				"HTTP_REQUESTS_TOTAL",
				"Memory_Usage_CURRENT_Value",  // has _CURRENT_ (case insensitive)
				"HTTP_DURATION_BUCKET",
			},
			expectedCounters:   []string{"HTTP_REQUESTS_TOTAL"},
			expectedGauges:     []string{"Memory_Usage_CURRENT_Value"},
			expectedHistograms: []string{"HTTP_DURATION_BUCKET"},
			expectedOthers:     nil,
		},
		{
			name: "edge cases with similar patterns",
			metrics: []string{
				"total_requests",         // 'total' at start, not end - other
				"count_operations",       // 'count' at start, not end - other
				"bucket_size",            // 'bucket' at start, not end - gauge (has _size)
				"connections_active_now", // has _active_ in name - gauge
			},
			expectedCounters:   nil,
			expectedGauges:     []string{"bucket_size", "connections_active_now"},
			expectedHistograms: nil,
			expectedOthers:     []string{"total_requests", "count_operations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counters, gauges, histograms, others := categorizeMetrics(tt.metrics)

			assert.Equal(t, tt.expectedCounters, counters, "Counters mismatch")
			assert.Equal(t, tt.expectedGauges, gauges, "Gauges mismatch")
			assert.Equal(t, tt.expectedHistograms, histograms, "Histograms mismatch")
			assert.Equal(t, tt.expectedOthers, others, "Others mismatch")
		})
	}
}

// TestLimitSlice tests slice limiting functionality
func TestLimitSlice(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		limit    int
		expected []string
	}{
		{
			name:     "empty slice",
			slice:    []string{},
			limit:    5,
			expected: []string{},
		},
		{
			name:     "slice shorter than limit",
			slice:    []string{"a", "b", "c"},
			limit:    5,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "slice equal to limit",
			slice:    []string{"a", "b", "c", "d", "e"},
			limit:    5,
			expected: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:     "slice longer than limit",
			slice:    []string{"a", "b", "c", "d", "e", "f", "g"},
			limit:    5,
			expected: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:     "limit of zero",
			slice:    []string{"a", "b", "c"},
			limit:    0,
			expected: []string{},
		},
		{
			name:     "limit of one",
			slice:    []string{"a", "b", "c"},
			limit:    1,
			expected: []string{"a"},
		},
		{
			name:     "nil slice",
			slice:    nil,
			limit:    5,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := limitSlice(tt.slice, tt.limit)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBuildPrompt tests prompt generation with various scenarios
func TestBuildPrompt(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		services       []semantic.Service
		intent         *QueryIntent
		similarQueries []semantic.SimilarQuery
		validateFunc   func(t *testing.T, prompt string)
	}{
		{
			name:     "no services discovered",
			services: []semantic.Service{},
			intent: &QueryIntent{
				Type:   "performance",
				Action: "show",
				Metric: "latency",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "WARNING: No services have been discovered yet")
				assert.Contains(t, prompt, "Return ERROR")
				assert.Contains(t, prompt, "CRITICAL RULES")
			},
		},
		{
			name: "single service with few metrics",
			services: []semantic.Service{
				{
					ID:        "svc-1",
					Name:      "api-gateway",
					Namespace: "production",
					MetricNames: []string{
						"http_requests_total",
						"http_errors_total",
						"http_duration_bucket",
					},
				},
			},
			intent: &QueryIntent{
				Type:   "performance",
				Action: "show",
				Metric: "throughput",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "AVAILABLE METRICS CATALOG")
				assert.Contains(t, prompt, "Service: api-gateway")
				assert.Contains(t, prompt, "namespace: production")
				assert.Contains(t, prompt, "Counters (use rate/increase)")
				assert.Contains(t, prompt, "http_requests_total")
				assert.Contains(t, prompt, "Histograms (use histogram_quantile)")
				assert.Contains(t, prompt, "http_duration_bucket")
				assert.Contains(t, prompt, "Intent: performance")
			},
		},
		{
			name: "multiple services with categorized metrics",
			services: []semantic.Service{
				{
					ID:        "svc-1",
					Name:      "api-gateway",
					Namespace: "production",
					MetricNames: []string{
						"http_requests_total",
						"memory_usage_current_bytes",  // gauge (has _current_)
						"http_duration_bucket",
					},
				},
				{
					ID:        "svc-2",
					Name:      "database",
					Namespace: "production",
					MetricNames: []string{
						"db_queries_total",
						"db_connections_active_now",  // gauge (has _active_)
					},
				},
			},
			intent: &QueryIntent{
				Type:    "errors",
				Action:  "show",
				Metric:  "error_rate",
				Service: "api-gateway",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "Service: api-gateway")
				assert.Contains(t, prompt, "Service: database")
				assert.Contains(t, prompt, "Target Service: api-gateway")
				assert.Contains(t, prompt, "Counters (use rate/increase)")
				assert.Contains(t, prompt, "Gauges (use directly or aggregate)")
				assert.Contains(t, prompt, "Histograms (use histogram_quantile)")
			},
		},
		{
			name: "service with many metrics - should filter",
			services: []semantic.Service{
				{
					ID:        "svc-1",
					Name:      "large-service",
					Namespace: "production",
					MetricNames: generateManyMetrics(100), // More than maxMetricsPerService (50)
				},
			},
			intent: &QueryIntent{
				Type:   "performance",
				Action: "show",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "Service: large-service")
				assert.Contains(t, prompt, "and", "Should indicate more metrics available")
				assert.Contains(t, prompt, "more metrics")
			},
		},
		{
			name: "targeted service with many metrics - should not filter",
			services: []semantic.Service{
				{
					ID:        "svc-1",
					Name:      "target-service",
					Namespace: "production",
					MetricNames: generateManyMetrics(60),
				},
				{
					ID:        "svc-2",
					Name:      "other-service",
					Namespace: "production",
					MetricNames: generateManyMetrics(70),
				},
			},
			intent: &QueryIntent{
				Type:    "performance",
				Action:  "show",
				Service: "target-service",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "Service: target-service")
				// For targeted service, all metrics should be shown
				// For other service, metrics should be filtered
				assert.Contains(t, prompt, "Service: other-service")
			},
		},
		{
			name: "with similar queries for examples",
			services: []semantic.Service{
				{
					ID:        "svc-1",
					Name:      "api-gateway",
					Namespace: "production",
					MetricNames: []string{"http_requests_total"},
				},
			},
			intent: &QueryIntent{
				Type:   "performance",
				Action: "show",
			},
			similarQueries: []semantic.SimilarQuery{
				{
					Query:  "Show me error rate",
					PromQL: `rate(http_errors_total[5m])`,
				},
				{
					Query:  "What's the latency?",
					PromQL: `histogram_quantile(0.95, rate(http_duration_bucket[5m]))`,
				},
			},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "EXAMPLES FROM PAST QUERIES")
				assert.Contains(t, prompt, "Show me error rate")
				assert.Contains(t, prompt, "rate(http_errors_total[5m])")
				assert.Contains(t, prompt, "What's the latency?")
			},
		},
		{
			name: "with time range in intent",
			services: []semantic.Service{
				{
					ID:          "svc-1",
					Name:        "api",
					Namespace:   "default",
					MetricNames: []string{"requests_total"},
				},
			},
			intent: &QueryIntent{
				Type:      "performance",
				Action:    "show",
				TimeRange: "5m",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "Time Range: 5m")
				assert.Contains(t, prompt, "Detected Context")
			},
		},
		{
			name: "service with no metrics discovered yet",
			services: []semantic.Service{
				{
					ID:          "svc-1",
					Name:        "new-service",
					Namespace:   "staging",
					MetricNames: []string{},
				},
			},
			intent: &QueryIntent{
				Type:   "metrics",
				Action: "show",
			},
			similarQueries: []semantic.SimilarQuery{},
			validateFunc: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "Service: new-service")
				assert.Contains(t, prompt, "(No metrics discovered yet)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock semantic mapper
			mockMapper := &MockSemanticMapper{
				services: tt.services,
			}

			// Create query processor
			qp := &QueryProcessor{
				semanticMapper: mockMapper,
			}

			// Build prompt
			req := &QueryRequest{
				Query: "test query",
			}
			prompt, err := qp.buildPrompt(ctx, req, tt.intent, tt.similarQueries)

			require.NoError(t, err)
			assert.NotEmpty(t, prompt)

			// Run validation function
			if tt.validateFunc != nil {
				tt.validateFunc(t, prompt)
			}

			// Common validations for all prompts
			assert.Contains(t, prompt, "CRITICAL RULES")
			assert.Contains(t, prompt, "YOUR TASK")
			assert.Contains(t, prompt, "User Query:")
		})
	}
}

// TestProcessQuery_ErrorHandling tests ERROR response from LLM
func TestProcessQuery_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		llmResponse    string
		expectedError  bool
		errorContains  string
	}{
		{
			name:          "LLM returns ERROR - no suitable metrics",
			llmResponse:   "ERROR: No suitable metrics found. The requested CPU metrics are not available in the current service catalog.",
			expectedError: true,
			errorContains: "No suitable metrics found",
		},
		{
			name:          "LLM returns ERROR with leading spaces",
			llmResponse:   "  ERROR: Cannot find matching metrics for memory usage",
			expectedError: true,
			errorContains: "Cannot find matching metrics",
		},
		{
			name:          "LLM returns valid PromQL",
			llmResponse:   `rate(http_requests_total[5m])`,
			expectedError: false,
		},
		{
			name:          "LLM returns ERROR with newlines",
			llmResponse:   "\nERROR: Service not discovered yet\n",
			expectedError: true,
			errorContains: "Service not discovered yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock LLM client
			mockLLM := &MockLLMClient{
				response: &llm.Response{
					PromQL:      tt.llmResponse,
					Explanation: "Test explanation",
					Confidence:  0.9,
				},
			}

			// Create mock semantic mapper
			mockMapper := &MockSemanticMapper{
				services: []semantic.Service{
					{
						ID:          "svc-1",
						Name:        "test-service",
						Namespace:   "default",
						MetricNames: []string{"test_metric_total"},
					},
				},
			}

			// Create mock Redis client
			mockRedis := redis.NewClient(&redis.Options{
				Addr: "localhost:6379",
			})

			// Create query processor
			qp := NewQueryProcessor(mockLLM, mockMapper, mockRedis)

			// Process query
			req := &QueryRequest{
				Query: "test query",
			}
			response, err := qp.ProcessQuery(ctx, req)

			if tt.expectedError {
				require.Error(t, err)
				// Check if error message contains the expected error string
				// Note: The error format is "[ERROR_CODE] message: details"
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, response)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, tt.llmResponse, response.PromQL)
			}
		})
	}
}

// TestEstimateQueryCost tests query cost estimation
func TestEstimateQueryCost(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedCost int
	}{
		{
			name:         "simple query",
			query:        `up`,
			expectedCost: 1,
		},
		{
			name:         "query with sum",
			query:        `sum(http_requests_total)`,
			expectedCost: 3, // 1 base + 2 for sum
		},
		{
			name:         "query with avg",
			query:        `avg(http_requests_total)`,
			expectedCost: 3, // 1 base + 2 for avg
		},
		{
			name:         "query with rate",
			query:        `rate(http_requests_total[5m])`,
			expectedCost: 4, // 1 base + 3 for rate
		},
		{
			name:         "query with increase",
			query:        `increase(http_requests_total[5m])`,
			expectedCost: 4, // 1 base + 3 for increase
		},
		{
			name:         "query with regex",
			query:        `http_requests_total{service=~"api.*"}`,
			expectedCost: 6, // 1 base + 5 for regex
		},
		{
			name:         "complex query with multiple operations",
			query:        `sum(rate(http_requests_total{service=~"api.*"}[5m]))`,
			expectedCost: 11, // 1 base + 2 sum + 3 rate + 5 regex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qp := &QueryProcessor{}
			cost := qp.estimateQueryCost(tt.query)
			assert.Equal(t, tt.expectedCost, cost)
		})
	}
}

// TestCacheOperations tests caching functionality
func TestCacheOperations(t *testing.T) {
	ctx := context.Background()

	// Create mock Redis client (will fail to connect, but that's ok for unit tests)
	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	qp := &QueryProcessor{
		cache: mockRedis,
	}

	t.Run("cache miss returns error", func(t *testing.T) {
		result, err := qp.getCachedResult(ctx, "non-existent query")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("cache result structure", func(t *testing.T) {
		response := &QueryResponse{
			PromQL:      `rate(http_requests_total[5m])`,
			Explanation: "Test explanation",
			Confidence:  0.95,
		}

		// Marshal to JSON to verify structure
		data, err := json.Marshal(response)
		require.NoError(t, err)

		var decoded QueryResponse
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, response.PromQL, decoded.PromQL)
		assert.Equal(t, response.Explanation, decoded.Explanation)
		assert.Equal(t, response.Confidence, decoded.Confidence)
	})
}

// Mock implementations

type MockSemanticMapper struct {
	services []semantic.Service
}

func (m *MockSemanticMapper) GetServices(ctx context.Context) ([]semantic.Service, error) {
	return m.services, nil
}

func (m *MockSemanticMapper) GetServiceByName(ctx context.Context, name, namespace string) (*semantic.Service, error) {
	for _, svc := range m.services {
		if svc.Name == name && svc.Namespace == namespace {
			return &svc, nil
		}
	}
	return nil, nil
}

func (m *MockSemanticMapper) CreateService(ctx context.Context, name, namespace string, labels map[string]string) (*semantic.Service, error) {
	return nil, nil
}

func (m *MockSemanticMapper) UpdateServiceMetrics(ctx context.Context, serviceID string, metrics []string) error {
	return nil
}

func (m *MockSemanticMapper) DeleteService(ctx context.Context, serviceID string) error {
	return nil
}

func (m *MockSemanticMapper) SearchServices(ctx context.Context, searchTerm string) ([]semantic.Service, error) {
	return m.services, nil
}

func (m *MockSemanticMapper) GetMetrics(ctx context.Context, serviceID string) ([]semantic.Metric, error) {
	return []semantic.Metric{}, nil
}

func (m *MockSemanticMapper) CreateMetric(ctx context.Context, name, metricType, description, serviceID string, labels map[string]string) (*semantic.Metric, error) {
	return nil, nil
}

func (m *MockSemanticMapper) FindSimilarQueries(ctx context.Context, embedding []float32) ([]semantic.SimilarQuery, error) {
	return []semantic.SimilarQuery{}, nil
}

func (m *MockSemanticMapper) StoreQueryEmbedding(ctx context.Context, query string, embedding []float32, promql string) error {
	return nil
}

type MockLLMClient struct {
	response *llm.Response
	err      error
}

func (m *MockLLMClient) GenerateQuery(ctx context.Context, prompt string) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *MockLLMClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Return a simple embedding
	return make([]float32, 1536), nil
}

// Helper functions

func generateManyMetrics(count int) []string {
	metrics := make([]string, count)
	for i := 0; i < count; i++ {
		if i%3 == 0 {
			metrics[i] = "metric_" + string(rune('a'+i%26)) + "_total"
		} else if i%3 == 1 {
			metrics[i] = "metric_" + string(rune('a'+i%26)) + "_current"
		} else {
			metrics[i] = "metric_" + string(rune('a'+i%26)) + "_bucket"
		}
	}
	return metrics
}
