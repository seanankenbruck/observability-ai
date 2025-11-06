// internal/processor/intent_test.go
package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewIntentClassifier tests creation of intent classifier
func TestNewIntentClassifier(t *testing.T) {
	ic := NewIntentClassifier()

	require.NotNil(t, ic)
	assert.NotEmpty(t, ic.patterns)
	assert.Contains(t, ic.patterns, "error_rate")
	assert.Contains(t, ic.patterns, "latency")
	assert.Contains(t, ic.patterns, "throughput")
	assert.Contains(t, ic.patterns, "availability")
	assert.Contains(t, ic.patterns, "comparison")
	assert.Contains(t, ic.patterns, "service_name")
	assert.Contains(t, ic.patterns, "time_range")
}

// TestClassifyIntent tests query intent classification
func TestClassifyIntent(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedType   string
		expectedAction string
		expectedMetric string
	}{
		// Error rate queries
		{
			name:           "error rate with percentage",
			query:          "What is the error rate for api-gateway?",
			expectedType:   "errors",
			expectedAction: "show",
			expectedMetric: "error_rate",
		},
		{
			name:           "failure rate",
			query:          "What's the fail rate of payment-service?",
			expectedType:   "errors",
			expectedAction: "show",
			expectedMetric: "error_rate",
		},
		{
			name:           "5xx errors",
			query:          "How many 5xx errors rate?",
			expectedType:   "errors",
			expectedAction: "show",
			expectedMetric: "error_rate",
		},

		// Latency queries
		{
			name:           "latency keyword",
			query:          "What is the latency of api-gateway?",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "latency",
		},
		{
			name:           "response time",
			query:          "Show me response time for user-service",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "latency",
		},
		{
			name:           "duration",
			query:          "How long is the duration?",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "latency",
		},

		// Throughput queries
		{
			name:           "throughput keyword",
			query:          "What is the throughput of api-gateway?",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "throughput",
		},
		{
			name:           "requests per second",
			query:          "Show me requests per second",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "throughput",
		},
		{
			name:           "qps abbreviation",
			query:          "Current QPS for payment service",
			expectedType:   "performance",
			expectedAction: "show",
			expectedMetric: "throughput",
		},

		// Comparison queries
		{
			name:           "compare services",
			query:          "Compare api-gateway and user-service",
			expectedType:   "comparison",
			expectedAction: "compare",
			expectedMetric: "",
		},
		{
			name:           "versus",
			query:          "api-gateway vs user-service",
			expectedType:   "comparison",
			expectedAction: "compare",
			expectedMetric: "",
		},

		// Default
		{
			name:           "unrecognized query",
			query:          "Tell me about the weather",
			expectedType:   "metrics",
			expectedAction: "show",
			expectedMetric: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewIntentClassifier()
			intent, err := ic.ClassifyIntent(tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, intent.Type,
				"Expected type %s for query: %s", tt.expectedType, tt.query)
			assert.Equal(t, tt.expectedAction, intent.Action,
				"Expected action %s for query: %s", tt.expectedAction, tt.query)
			if tt.expectedMetric != "" {
				assert.Equal(t, tt.expectedMetric, intent.Metric,
					"Expected metric %s for query: %s", tt.expectedMetric, tt.query)
			}
		})
	}
}

// TestExtractServiceName tests service name extraction
func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		expectedService string
	}{
		{
			name:            "service keyword with name",
			query:           "error rate for service api-gateway",
			expectedService: "api-gateway",
		},
		{
			name:            "app keyword with name",
			query:           "latency for app user-service",
			expectedService: "user-service",
		},
		{
			name:            "application keyword with name",
			query:           "throughput of application payment",
			expectedService: "payment",
		},
		{
			name:            "service with underscore",
			query:           "service user_service latency",
			expectedService: "user_service",
		},
		{
			name:            "no service keyword",
			query:           "What is the overall error rate?",
			expectedService: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewIntentClassifier()
			intent, err := ic.ClassifyIntent(tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedService, intent.Service,
				"Expected service %s for query: %s", tt.expectedService, tt.query)
		})
	}
}

// TestExtractTimeRange tests time range extraction
func TestExtractTimeRange(t *testing.T) {
	tests := []struct {
		name              string
		query             string
		expectedTimeRange string
	}{
		{
			name:              "last 5 minutes",
			query:             "error rate in the last 5 minutes",
			expectedTimeRange: "5minute",
		},
		{
			name:              "last 1 hour",
			query:             "Show me errors from the last 1 hour",
			expectedTimeRange: "1hour",
		},
		{
			name:              "last 24 hours",
			query:             "latency over the last 24 hours",
			expectedTimeRange: "24hour",
		},
		{
			name:              "last 1 day",
			query:             "errors in the last 1 day",
			expectedTimeRange: "1day",
		},
		{
			name:              "last 1 week",
			query:             "availability for the last 1 week",
			expectedTimeRange: "1week",
		},
		{
			name:              "past 30 minutes",
			query:             "What happened in the past 30 minutes?",
			expectedTimeRange: "30minute",
		},
		{
			name:              "past 2 hours",
			query:             "errors in the past 2 hours",
			expectedTimeRange: "2hour",
		},
		{
			name:              "past 7 days",
			query:             "throughput over the past 7 days",
			expectedTimeRange: "7day",
		},
		{
			name:              "no time range",
			query:             "What is the current error rate?",
			expectedTimeRange: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewIntentClassifier()
			intent, err := ic.ClassifyIntent(tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTimeRange, intent.TimeRange,
				"Expected time range %s for query: %s", tt.expectedTimeRange, tt.query)
		})
	}
}

// TestComparisonPattern tests comparison detection
func TestComparisonPattern(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedAction string
		expectedType   string
	}{
		{
			name:           "compare keyword",
			query:          "compare api-gateway and user-service",
			expectedAction: "compare",
			expectedType:   "comparison",
		},
		{
			name:           "versus keyword",
			query:          "api-gateway versus user-service",
			expectedAction: "compare",
			expectedType:   "comparison",
		},
		{
			name:           "vs abbreviation",
			query:          "api-gateway vs user-service",
			expectedAction: "compare",
			expectedType:   "comparison",
		},
		{
			name:           "against keyword",
			query:          "api-gateway against user-service",
			expectedAction: "compare",
			expectedType:   "comparison",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := NewIntentClassifier()
			intent, err := ic.ClassifyIntent(tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedAction, intent.Action,
				"Expected action %s for query: %s", tt.expectedAction, tt.query)
			assert.Equal(t, tt.expectedType, intent.Type,
				"Expected type %s for query: %s", tt.expectedType, tt.query)
		})
	}
}

// TestComplexQueries tests complex real-world queries
func TestComplexQueries(t *testing.T) {
	ic := NewIntentClassifier()

	tests := []struct {
		name           string
		query          string
		expectedType   string
		expectedMetric string
		expectedAction string
	}{
		{
			name:           "complex error query",
			query:          "Show me the error rate for service api-gateway over the last 24 hours",
			expectedType:   "errors",
			expectedMetric: "error_rate",
			expectedAction: "show",
		},
		{
			name:           "complex latency query",
			query:          "What's the latency for app user-service in the past 2 hours?",
			expectedType:   "performance",
			expectedMetric: "latency",
			expectedAction: "show",
		},
		{
			name:           "complex compare query",
			query:          "Compare api-gateway and user-service",
			expectedType:   "comparison",
			expectedAction: "compare",
		},
		{
			name:           "throughput query",
			query:          "Show requests for service payment",
			expectedType:   "performance",
			expectedMetric: "throughput",
			expectedAction: "show",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent, err := ic.ClassifyIntent(tt.query)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, intent.Type)
			if tt.expectedMetric != "" {
				assert.Equal(t, tt.expectedMetric, intent.Metric)
			}
			assert.Equal(t, tt.expectedAction, intent.Action)
		})
	}
}

// TestIntentEdgeCases tests edge cases and unusual inputs
func TestIntentEdgeCases(t *testing.T) {
	ic := NewIntentClassifier()

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "empty query",
			query: "",
		},
		{
			name:  "whitespace only",
			query: "   ",
		},
		{
			name:  "very long query",
			query: "Show me the error rate for service api-gateway in the last 5 minutes and also the latency and throughput",
		},
		{
			name:  "special characters",
			query: "error rate!@#$%^&*()",
		},
		{
			name:  "mixed case",
			query: "SHOW ME THE ERROR RATE",
		},
		{
			name:  "numbers only",
			query: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			intent, err := ic.ClassifyIntent(tt.query)
			require.NoError(t, err)
			assert.NotNil(t, intent)
			assert.NotEmpty(t, intent.Type)
			assert.NotEmpty(t, intent.Action)
		})
	}
}

// TestCaseInsensitivity tests that classification is case-insensitive
func TestCaseInsensitivity(t *testing.T) {
	ic := NewIntentClassifier()

	queries := []string{
		"show me the error rate",
		"SHOW ME THE ERROR RATE",
		"Show Me The Error Rate",
		"sHoW mE tHe ErRoR rAtE",
	}

	var firstIntent *QueryIntent
	for i, query := range queries {
		intent, err := ic.ClassifyIntent(query)
		require.NoError(t, err)

		if i == 0 {
			firstIntent = intent
		} else {
			assert.Equal(t, firstIntent.Type, intent.Type,
				"Type should be case-insensitive")
			assert.Equal(t, firstIntent.Action, intent.Action,
				"Action should be case-insensitive")
		}
	}
}

// TestMultipleMatches tests behavior when query matches multiple patterns
func TestMultipleMatches(t *testing.T) {
	ic := NewIntentClassifier()

	tests := []struct {
		name         string
		query        string
		expectedType string
		reason       string
	}{
		{
			name:         "error keyword should match first",
			query:        "Show me error rate",
			expectedType: "errors",
			reason:       "Error pattern should match",
		},
		{
			name:         "latency should match",
			query:        "Show me latency",
			expectedType: "performance",
			reason:       "Latency pattern should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent, err := ic.ClassifyIntent(tt.query)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, intent.Type, tt.reason)
		})
	}
}

// TestAggregationAssignment tests that aggregation is set correctly
func TestAggregationAssignment(t *testing.T) {
	ic := NewIntentClassifier()

	tests := []struct {
		name                string
		query               string
		expectedAggregation string
	}{
		{
			name:                "error rate uses rate",
			query:               "error rate",
			expectedAggregation: "rate",
		},
		{
			name:                "latency uses avg",
			query:               "latency",
			expectedAggregation: "avg",
		},
		{
			name:                "throughput uses rate",
			query:               "requests per second",
			expectedAggregation: "rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intent, err := ic.ClassifyIntent(tt.query)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAggregation, intent.Aggregation)
		})
	}
}

// BenchmarkClassifyIntent benchmarks intent classification
func BenchmarkClassifyIntent(b *testing.B) {
	ic := NewIntentClassifier()
	query := "What is the error rate for service api-gateway over the last 24 hours?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ic.ClassifyIntent(query)
	}
}

// BenchmarkClassifyComplexQuery benchmarks complex query classification
func BenchmarkClassifyComplexQuery(b *testing.B) {
	ic := NewIntentClassifier()
	query := "Compare api-gateway and user-service over the past 7 days"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ic.ClassifyIntent(query)
	}
}
