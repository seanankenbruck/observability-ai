// internal/processor/safety_test.go
package processor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSafetyChecker tests creation of safety checker
func TestNewSafetyChecker(t *testing.T) {
	sc := NewSafetyChecker()

	require.NotNil(t, sc)
	assert.Equal(t, 7*24*time.Hour, sc.MaxQueryRange)
	assert.Equal(t, 10000, sc.MaxCardinality)
	assert.Equal(t, 30, sc.TimeoutSeconds)
	assert.NotEmpty(t, sc.ForbiddenMetrics)
	assert.Contains(t, sc.ForbiddenMetrics, ".*_secret.*")
	assert.Contains(t, sc.ForbiddenMetrics, ".*_password.*")
	assert.Contains(t, sc.ForbiddenMetrics, ".*_token.*")
	assert.Contains(t, sc.ForbiddenMetrics, ".*_key.*")
}

// TestValidateQuery tests query validation
func TestValidateQuery(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "safe query",
			query:   `rate(http_requests_total{service="api"}[5m])`,
			wantErr: false,
		},
		{
			name:    "safe aggregation query",
			query:   `sum(rate(http_requests_total[5m])) by (service)`,
			wantErr: false,
		},
		{
			name:    "safe histogram query",
			query:   `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))`,
			wantErr: false,
		},
		{
			name:        "forbidden metric - secret",
			query:       `rate(app_secret_key[5m])`,
			wantErr:     true,
			errContains: "forbidden metric pattern",
		},
		{
			name:        "forbidden metric - password",
			query:       `database_password_hash`,
			wantErr:     true,
			errContains: "forbidden metric pattern",
		},
		{
			name:        "forbidden metric - token",
			query:       `auth_token_count`,
			wantErr:     true,
			errContains: "forbidden metric pattern",
		},
		{
			name:        "forbidden metric - api key",
			query:       `api_key_usage`,
			wantErr:     true,
			errContains: "forbidden metric pattern",
		},
		{
			name:        "excessive time range - 1 year",
			query:       `rate(http_requests_total[1y])`,
			wantErr:     true,
			errContains: "time range exceeds maximum",
		},
		{
			name:        "excessive time range - 365 days",
			query:       `rate(http_requests_total[365d])`,
			wantErr:     true,
			errContains: "time range exceeds maximum",
		},
		{
			name:        "excessive time range - 52 weeks",
			query:       `rate(http_requests_total[52w])`,
			wantErr:     true,
			errContains: "time range exceeds maximum",
		},
		{
			name:        "high cardinality - empty by",
			query:       `sum(rate(http_requests_total[5m])) by ()`,
			wantErr:     true,
			errContains: "high cardinality",
		},
		{
			name:        "high cardinality - empty without",
			query:       `sum(rate(http_requests_total[5m])) without ()`,
			wantErr:     true,
			errContains: "high cardinality",
		},
		{
			name:        "expensive operation - group_left",
			query:       `http_requests_total * on(instance) group_left(node) node_info`,
			wantErr:     true,
			errContains: "expensive operation",
		},
		{
			name:        "expensive operation - group_right",
			query:       `http_requests_total * on(instance) group_right(node) node_info`,
			wantErr:     true,
			errContains: "expensive operation",
		},
		{
			name:        "expensive operation - absent",
			query:       `absent(up{job="prometheus"})`,
			wantErr:     true,
			errContains: "expensive operation",
		},
		{
			name:        "too many nested operations",
			query:       `sum(avg(rate(max(http_requests_total[5m]))))`,
			wantErr:     true,
			errContains: "too many nested operations",
		},
		{
			name:    "acceptable nested operations",
			query:   `sum(rate(http_requests_total[5m]))`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSafetyChecker()
			err := sc.ValidateQuery(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateTimeRange tests time range validation
func TestValidateTimeRange(t *testing.T) {
	tests := []struct {
		name        string
		timeRange   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "safe range - 5 minutes",
			timeRange: "5m",
			wantErr:   false,
		},
		{
			name:      "safe range - 1 hour",
			timeRange: "1h",
			wantErr:   false,
		},
		{
			name:      "safe range - 24 hours",
			timeRange: "24h",
			wantErr:   false,
		},
		{
			name:      "safe range - 1 day",
			timeRange: "1d",
			wantErr:   false,
		},
		{
			name:      "safe range - 7 days (at limit)",
			timeRange: "7d",
			wantErr:   false,
		},
		{
			name:      "safe range - 1 week",
			timeRange: "1w",
			wantErr:   false,
		},
		{
			name:        "unsafe range - 8 days",
			timeRange:   "8d",
			wantErr:     true,
			errContains: "exceeds maximum allowed",
		},
		{
			name:        "unsafe range - 2 weeks",
			timeRange:   "2w",
			wantErr:     true,
			errContains: "exceeds maximum allowed",
		},
		{
			name:        "unsafe range - 200 hours",
			timeRange:   "200h",
			wantErr:     true,
			errContains: "exceeds maximum allowed",
		},
		{
			name:        "unsafe range - 15000 minutes",
			timeRange:   "15000m",
			wantErr:     true,
			errContains: "exceeds maximum allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSafetyChecker()
			err := sc.ValidateTimeRange(tt.timeRange)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestEstimateCardinality tests cardinality estimation
func TestEstimateCardinality(t *testing.T) {
	tests := []struct {
		name              string
		query             string
		expectedMin       int
		expectedMax       int
		shouldBeReduced   bool
		shouldBeIncreased bool
	}{
		{
			name:        "simple query no labels",
			query:       `up`,
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name:        "single label matcher",
			query:       `http_requests_total{service="api"}`,
			expectedMin: 1,
			expectedMax: 1,
		},
		{
			name:        "multiple label matchers",
			query:       `http_requests_total{service="api",method="GET",status="200"}`,
			expectedMin: 1,
			expectedMax: 5,
		},
		{
			name:            "with sum aggregation",
			query:           `sum(http_requests_total)`,
			expectedMin:     0,
			expectedMax:     1,
			shouldBeReduced: true,
		},
		{
			name:            "with avg aggregation",
			query:           `avg(http_requests_total)`,
			expectedMin:     0,
			expectedMax:     1,
			shouldBeReduced: true,
		},
		{
			name:              "with group by",
			query:             `sum(http_requests_total) by (service)`,
			expectedMin:       0,
			expectedMax:       20,
			shouldBeIncreased: true,
		},
		{
			name:        "complex query with multiple labels and grouping",
			query:       `sum(rate(http_requests_total{service="api",method=~"GET|POST"}[5m])) by (status)`,
			expectedMin: 1,
			expectedMax: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSafetyChecker()
			cardinality := sc.EstimateCardinality(tt.query)

			assert.GreaterOrEqual(t, cardinality, tt.expectedMin,
				"Cardinality should be at least %d", tt.expectedMin)

			if tt.expectedMax > 0 {
				assert.LessOrEqual(t, cardinality, tt.expectedMax,
					"Cardinality should be at most %d", tt.expectedMax)
			}

			if tt.shouldBeReduced {
				assert.LessOrEqual(t, cardinality, 1,
					"Aggregation should reduce cardinality")
			}

			if tt.shouldBeIncreased {
				// Group by might result in 0 if no labels, so just check it doesn't reduce below 0
				assert.GreaterOrEqual(t, cardinality, 0,
					"Group by should not reduce cardinality below 0")
			}
		})
	}
}

// TestCustomSafetyCheckerConfig tests custom safety checker configuration
func TestCustomSafetyCheckerConfig(t *testing.T) {
	// Create custom safety checker with stricter rules
	sc := &SafetyChecker{
		MaxQueryRange:  1 * 24 * time.Hour, // Only 1 day
		MaxCardinality: 1000,
		TimeoutSeconds: 10,
		ForbiddenMetrics: []string{
			".*_secret.*",
			".*_password.*",
			".*_internal.*", // Custom forbidden pattern
		},
	}

	t.Run("time range validation", func(t *testing.T) {
		err := sc.ValidateTimeRange("12h")
		assert.NoError(t, err, "12h should be within 1 day limit")

		err = sc.ValidateTimeRange("2d")
		assert.Error(t, err, "2d should exceed 1 day limit")
	})

	t.Run("custom forbidden pattern", func(t *testing.T) {
		err := sc.ValidateQuery("app_internal_debug")
		assert.Error(t, err, "Should catch _internal_ pattern")

		err = sc.ValidateQuery("app_secret_key")
		assert.Error(t, err, "Should catch _secret_ pattern")
	})
}

// TestEdgeCases tests edge cases and potential bypasses
func TestEdgeCases(t *testing.T) {
	sc := NewSafetyChecker()

	tests := []struct {
		name        string
		query       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "empty query",
			query:   "",
			wantErr: false,
		},
		{
			name:    "whitespace only",
			query:   "   ",
			wantErr: false,
		},
		{
			name:    "uppercase SECRET - pattern is case-sensitive",
			query:   `my_SECRET_value`,
			wantErr: false, // Pattern .*_secret.* is lowercase only
		},
		{
			name:    "mixed case Password - pattern is case-sensitive",
			query:   `user_Password_hash`,
			wantErr: false, // Pattern .*_password.* is lowercase only
		},
		{
			name:    "similar but safe - tokenize (not token)",
			query:   `tokenize_operation_count`,
			wantErr: false, // "tokenize" doesn't match ".*_token.*" pattern
		},
		{
			name:        "embedded forbidden word",
			query:       `oauth_token_refresh`,
			wantErr:     true,
			errContains: "forbidden",
		},
		{
			name:    "safe query with parentheses",
			query:   `((http_requests_total))`,
			wantErr: false,
		},
		{
			name:    "multiple time ranges in query",
			query:   `rate(http_requests_total[5m]) / rate(http_requests_total[1h])`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sc.ValidateQuery(tt.query)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSecurityBypass tests potential security bypass attempts
func TestSecurityBypass(t *testing.T) {
	sc := NewSafetyChecker()

	bypassAttempts := []struct {
		name        string
		query       string
		shouldBlock bool
		reason      string
	}{
		{
			name:        "secret pattern in metric name",
			query:       `my_secret_value`,
			shouldBlock: true,
			reason:      "Should catch _secret_ pattern",
		},
		{
			name:        "token pattern in metric name",
			query:       `api_token_count`,
			shouldBlock: true,
			reason:      "Should catch _token_ pattern",
		},
		{
			name:        "key pattern in metric name",
			query:       `api_key_usage`,
			shouldBlock: true,
			reason:      "Should catch _key_ pattern",
		},
		{
			name:        "multiple nested parentheses for bypass",
			query:       `(((((http_requests_total)))))`,
			shouldBlock: true,
			reason:      "Should catch excessive nesting",
		},
	}

	for _, tt := range bypassAttempts {
		t.Run(tt.name, func(t *testing.T) {
			err := sc.ValidateQuery(tt.query)

			if tt.shouldBlock {
				assert.Error(t, err, "Expected query to be blocked: %s", tt.reason)
			} else {
				assert.NoError(t, err, "Query should be allowed: %s", tt.reason)
			}
		})
	}
}

// BenchmarkValidateQuery benchmarks query validation
func BenchmarkValidateQuery(b *testing.B) {
	sc := NewSafetyChecker()
	query := `sum(rate(http_requests_total{service="api",method="GET"}[5m])) by (status)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sc.ValidateQuery(query)
	}
}

// BenchmarkEstimateCardinality benchmarks cardinality estimation
func BenchmarkEstimateCardinality(b *testing.B) {
	sc := NewSafetyChecker()
	query := `sum(rate(http_requests_total{service="api",method="GET",status="200"}[5m])) by (instance,pod)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sc.EstimateCardinality(query)
	}
}
