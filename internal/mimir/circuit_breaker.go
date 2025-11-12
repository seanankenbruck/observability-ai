package mimir

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig defines circuit breaker configuration for Mimir
type CircuitBreakerConfig struct {
	MaxRequests   uint32        // Max requests allowed in half-open state
	Interval      time.Duration // Window for counting failures
	Timeout       time.Duration // Duration circuit stays open before trying recovery
	ReadyToTrip   func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from gobreaker.State, to gobreaker.State)
}

// DefaultCircuitBreakerConfig provides sensible defaults for Mimir
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	MaxRequests: 1,
	Interval:    10 * time.Second, // Count failures over 10 seconds
	Timeout:     30 * time.Second, // Try recovery after 30 seconds
	ReadyToTrip: func(counts gobreaker.Counts) bool {
		// Open circuit if we see 5 failures in the interval
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= 3 && (counts.ConsecutiveFailures >= 5 || failureRatio >= 0.6)
	},
	OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
		// Log state changes for monitoring
		fmt.Printf("Circuit breaker '%s' changed from %s to %s\n", name, from, to)
	},
}

// CircuitBreakerClient wraps a Mimir client with circuit breaker protection
type CircuitBreakerClient struct {
	client  *Client
	breaker *gobreaker.CircuitBreaker
}

// NewCircuitBreakerClient creates a new circuit breaker wrapped Mimir client
func NewCircuitBreakerClient(client *Client, name string, config CircuitBreakerConfig) *CircuitBreakerClient {
	settings := gobreaker.Settings{
		Name:          name,
		MaxRequests:   config.MaxRequests,
		Interval:      config.Interval,
		Timeout:       config.Timeout,
		ReadyToTrip:   config.ReadyToTrip,
		OnStateChange: config.OnStateChange,
	}

	return &CircuitBreakerClient{
		client:  client,
		breaker: gobreaker.NewCircuitBreaker(settings),
	}
}

// Query wraps the client's Query with circuit breaker protection
func (cb *CircuitBreakerClient) Query(ctx context.Context, query string, timestamp time.Time) (*QueryResponse, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.Query(ctx, query, timestamp)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.(*QueryResponse), nil
}

// QueryRange wraps the client's QueryRange with circuit breaker protection
func (cb *CircuitBreakerClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResponse, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.QueryRange(ctx, query, start, end, step)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.(*QueryResponse), nil
}

// GetMetricNames wraps the client's GetMetricNames with circuit breaker protection
func (cb *CircuitBreakerClient) GetMetricNames(ctx context.Context) ([]string, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.GetMetricNames(ctx)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.([]string), nil
}

// GetLabelValues wraps the client's GetLabelValues with circuit breaker protection
func (cb *CircuitBreakerClient) GetLabelValues(ctx context.Context, labelName string, metricMatchers ...string) ([]string, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.GetLabelValues(ctx, labelName, metricMatchers...)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.([]string), nil
}

// GetMetricMetadata wraps the client's GetMetricMetadata with circuit breaker protection
func (cb *CircuitBreakerClient) GetMetricMetadata(ctx context.Context, metricName string) (*MetricMetadata, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.GetMetricMetadata(ctx, metricName)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.(*MetricMetadata), nil
}

// TestConnection wraps the client's TestConnection with circuit breaker protection
func (cb *CircuitBreakerClient) TestConnection(ctx context.Context) error {
	_, err := cb.breaker.Execute(func() (interface{}, error) {
		return nil, cb.client.TestConnection(ctx)
	})

	return err
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreakerClient) State() gobreaker.State {
	return cb.breaker.State()
}

// Counts returns the current failure counts
func (cb *CircuitBreakerClient) Counts() gobreaker.Counts {
	return cb.breaker.Counts()
}
