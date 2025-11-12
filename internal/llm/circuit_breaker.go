package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerConfig defines circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxRequests  uint32        // Max requests allowed in half-open state
	Interval     time.Duration // Window for counting failures
	Timeout      time.Duration // Duration circuit stays open before trying recovery
	ReadyToTrip  func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from gobreaker.State, to gobreaker.State)
}

// DefaultCircuitBreakerConfig provides sensible defaults
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

// CircuitBreakerClient wraps an LLM client with circuit breaker protection
type CircuitBreakerClient struct {
	client  Client
	breaker *gobreaker.CircuitBreaker
}

// NewCircuitBreakerClient creates a new circuit breaker wrapped client
func NewCircuitBreakerClient(client Client, name string, config CircuitBreakerConfig) *CircuitBreakerClient {
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

// GenerateQuery wraps the client's GenerateQuery with circuit breaker protection
func (cb *CircuitBreakerClient) GenerateQuery(ctx context.Context, prompt string) (*Response, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.GenerateQuery(ctx, prompt)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.(*Response), nil
}

// GetEmbedding wraps the client's GetEmbedding with circuit breaker protection
func (cb *CircuitBreakerClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return cb.client.GetEmbedding(ctx, text)
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}

	return result.([]float32), nil
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreakerClient) State() gobreaker.State {
	return cb.breaker.State()
}

// Counts returns the current failure counts
func (cb *CircuitBreakerClient) Counts() gobreaker.Counts {
	return cb.breaker.Counts()
}
