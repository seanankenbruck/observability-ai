package llm

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// RetryConfig defines retry behavior for Claude API calls
type RetryConfig struct {
	MaxRetries int           // Maximum number of retry attempts
	BaseDelay  time.Duration // Initial delay between retries
	MaxDelay   time.Duration // Maximum delay between retries
}

// DefaultRetryConfig provides sensible defaults for retry behavior
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	BaseDelay:  100 * time.Millisecond,
	MaxDelay:   5 * time.Second,
}

// sendClaudeRequestWithRetry wraps sendClaudeRequest with retry logic
func (c *ClaudeClient) sendClaudeRequestWithRetry(ctx context.Context, request ClaudeRequest) (*ClaudeResponse, error) {
	config := DefaultRetryConfig
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Try to send the request
		response, err := c.sendClaudeRequest(ctx, request)

		// Success - return immediately
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if we should retry this error
		if !isRetryableError(err) {
			// Non-retryable error (auth, bad request, etc.) - fail immediately
			return nil, err
		}

		// Last attempt - don't wait, just return the error
		if attempt == config.MaxRetries {
			break
		}

		// Calculate backoff delay with exponential backoff and jitter
		delay := calculateBackoff(attempt, config.BaseDelay, config.MaxDelay)

		// Wait before retrying, but respect context cancellation
		select {
		case <-time.After(delay):
			// Continue to next retry
			continue
		case <-ctx.Done():
			// Context cancelled or timed out
			return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Retry rate limit errors (429)
	if strings.Contains(errMsg, "rate limit exceeded") {
		return true
	}

	// Retry server errors (500, 502, 503, 504)
	if strings.Contains(errMsg, "internal error") ||
		strings.Contains(errMsg, "API error 500") ||
		strings.Contains(errMsg, "API error 502") ||
		strings.Contains(errMsg, "API error 503") ||
		strings.Contains(errMsg, "API error 504") {
		return true
	}

	// Retry timeout errors
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return true
	}

	// Retry connection errors
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "EOF") {
		return true
	}

	// Don't retry auth errors (401, 403)
	if strings.Contains(errMsg, "invalid API key") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "API error 401") ||
		strings.Contains(errMsg, "API error 403") {
		return false
	}

	// Don't retry bad requests (400)
	if strings.Contains(errMsg, "bad request") ||
		strings.Contains(errMsg, "API error 400") {
		return false
	}

	// Default: don't retry unknown errors
	return false
}

// calculateBackoff calculates the delay before the next retry attempt
// Uses exponential backoff with jitter to avoid thundering herd
func calculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay

	// Cap at maxDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter (random factor between 0.5 and 1.5)
	jitter := 0.5 + rand.Float64()
	delay = time.Duration(float64(delay) * jitter)

	return delay
}

// isHTTPStatusRetryable checks if an HTTP status code should be retried
func isHTTPStatusRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests: // 429
		return true
	case http.StatusInternalServerError: // 500
		return true
	case http.StatusBadGateway: // 502
		return true
	case http.StatusServiceUnavailable: // 503
		return true
	case http.StatusGatewayTimeout: // 504
		return true
	default:
		return false
	}
}
