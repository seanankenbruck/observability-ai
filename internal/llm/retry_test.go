package llm

import (
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "rate limit error should be retryable",
			errMsg:   "rate limit exceeded: too many requests",
			expected: true,
		},
		{
			name:     "500 error should be retryable",
			errMsg:   "Claude API error 500: internal server error",
			expected: true,
		},
		{
			name:     "503 error should be retryable",
			errMsg:   "API error 503: service unavailable",
			expected: true,
		},
		{
			name:     "timeout should be retryable",
			errMsg:   "request timeout exceeded",
			expected: true,
		},
		{
			name:     "connection refused should be retryable",
			errMsg:   "connection refused by server",
			expected: true,
		},
		{
			name:     "auth error should not be retryable",
			errMsg:   "invalid API key: authentication failed",
			expected: false,
		},
		{
			name:     "401 error should not be retryable",
			errMsg:   "API error 401: unauthorized",
			expected: false,
		},
		{
			name:     "bad request should not be retryable",
			errMsg:   "bad request: invalid parameters",
			expected: false,
		},
		{
			name:     "400 error should not be retryable",
			errMsg:   "API error 400: bad request",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple error with the message
			err := &testError{msg: tt.errMsg}
			result := isRetryableError(err)
			if result != tt.expected {
				t.Errorf("isRetryableError() = %v, expected %v for error: %s", result, tt.expected, tt.errMsg)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second

	tests := []struct {
		name           string
		attempt        int
		expectedMin    time.Duration
		expectedMax    time.Duration
	}{
		{
			name:        "first retry (attempt 0)",
			attempt:     0,
			expectedMin: 50 * time.Millisecond,  // baseDelay * 2^0 * 0.5 (min jitter)
			expectedMax: 150 * time.Millisecond, // baseDelay * 2^0 * 1.5 (max jitter)
		},
		{
			name:        "second retry (attempt 1)",
			attempt:     1,
			expectedMin: 100 * time.Millisecond, // baseDelay * 2^1 * 0.5
			expectedMax: 300 * time.Millisecond, // baseDelay * 2^1 * 1.5
		},
		{
			name:        "third retry (attempt 2)",
			attempt:     2,
			expectedMin: 200 * time.Millisecond, // baseDelay * 2^2 * 0.5
			expectedMax: 600 * time.Millisecond, // baseDelay * 2^2 * 1.5
		},
		{
			name:        "large attempt should cap at maxDelay",
			attempt:     10,
			expectedMin: 2500 * time.Millisecond, // maxDelay * 0.5 (min jitter)
			expectedMax: 7500 * time.Millisecond, // maxDelay * 1.5 (max jitter, capped at maxDelay)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to account for jitter
			for i := 0; i < 10; i++ {
				delay := calculateBackoff(tt.attempt, baseDelay, maxDelay)

				// Check if delay is within expected range
				if delay < tt.expectedMin {
					t.Errorf("calculateBackoff() = %v, expected >= %v", delay, tt.expectedMin)
				}
				// For large attempts, the max with jitter can exceed maxDelay, but should be reasonable
				maxAllowed := tt.expectedMax
				if tt.attempt > 5 {
					maxAllowed = maxDelay * 2 // Allow 2x maxDelay with jitter
				}
				if delay > maxAllowed {
					t.Errorf("calculateBackoff() = %v, expected <= %v", delay, maxAllowed)
				}
			}
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	if DefaultRetryConfig.MaxRetries != 3 {
		t.Errorf("DefaultRetryConfig.MaxRetries = %d, expected 3", DefaultRetryConfig.MaxRetries)
	}
	if DefaultRetryConfig.BaseDelay != 100*time.Millisecond {
		t.Errorf("DefaultRetryConfig.BaseDelay = %v, expected 100ms", DefaultRetryConfig.BaseDelay)
	}
	if DefaultRetryConfig.MaxDelay != 5*time.Second {
		t.Errorf("DefaultRetryConfig.MaxDelay = %v, expected 5s", DefaultRetryConfig.MaxDelay)
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
