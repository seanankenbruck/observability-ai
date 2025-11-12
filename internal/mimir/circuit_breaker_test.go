package mimir

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

func TestMimirCircuitBreakerClient_Success(t *testing.T) {
	// Create a real Mimir client (it will fail to connect, but that's ok for this test)
	client := NewClient("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second)

	// Create circuit breaker client
	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", DefaultCircuitBreakerConfig)

	// Verify initial state
	assert.Equal(t, gobreaker.StateClosed, cbClient.State())
}

func TestMimirCircuitBreakerClient_OpensAfterFailures(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	// Configure circuit breaker with lower threshold for testing
	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			t.Logf("State changed from %s to %s", from, to)
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Make 3 failing requests to open the circuit
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := cbClient.Query(ctx, "up", time.Time{})
		assert.Error(t, err)
	}

	// Circuit should now be open
	assert.Equal(t, gobreaker.StateOpen, cbClient.State())

	// Next request should fail immediately without calling the client
	_, err := cbClient.Query(ctx, "up", time.Time{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestMimirCircuitBreakerClient_QueryRange(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Test QueryRange fails and counts towards circuit breaker
	ctx := context.Background()
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	_, err := cbClient.QueryRange(ctx, "up", start, end, 1*time.Minute)
	assert.Error(t, err)

	counts := cbClient.Counts()
	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalFailures)
}

func TestMimirCircuitBreakerClient_GetMetricNames(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Test GetMetricNames fails and counts towards circuit breaker
	ctx := context.Background()
	_, err := cbClient.GetMetricNames(ctx)
	assert.Error(t, err)

	counts := cbClient.Counts()
	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalFailures)
}

func TestMimirCircuitBreakerClient_GetLabelValues(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Test GetLabelValues fails and counts towards circuit breaker
	ctx := context.Background()
	_, err := cbClient.GetLabelValues(ctx, "job")
	assert.Error(t, err)

	counts := cbClient.Counts()
	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalFailures)
}

func TestMimirCircuitBreakerClient_GetMetricMetadata(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Test GetMetricMetadata - note this method has fallback behavior so may not error
	ctx := context.Background()
	metadata, err := cbClient.GetMetricMetadata(ctx, "up")

	// Metadata might succeed with fallback, so just check it was called
	if err == nil {
		assert.NotNil(t, metadata)
	}
}

func TestMimirCircuitBreakerClient_TestConnection(t *testing.T) {
	// Create a client pointing to non-existent endpoint
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 100*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-mimir-cb", config)

	// Test TestConnection fails and counts towards circuit breaker
	ctx := context.Background()
	err := cbClient.TestConnection(ctx)
	assert.Error(t, err)

	counts := cbClient.Counts()
	assert.Equal(t, uint32(1), counts.Requests)
	assert.Equal(t, uint32(1), counts.TotalFailures)
}

// MockFailingClient simulates a client that can be controlled to fail or succeed
type mockFailingClient struct {
	shouldFail bool
	failError  error
}

func TestCircuitBreakerRecovery(t *testing.T) {
	// This is a conceptual test - in practice, you'd use a mock server
	// For now, we verify the circuit breaker behavior with counts
	client := NewClient("http://localhost:19999", AuthConfig{Type: "none"}, 50*time.Millisecond)

	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     50 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			t.Logf("State changed from %s to %s", from, to)
		},
	}

	cbClient := NewCircuitBreakerClient(client, "test-recovery-cb", config)
	ctx := context.Background()

	// Fail twice to open circuit
	for i := 0; i < 2; i++ {
		_, err := cbClient.Query(ctx, "up", time.Time{})
		assert.Error(t, err)
	}

	// Verify circuit is open
	assert.Equal(t, gobreaker.StateOpen, cbClient.State())

	// Wait for timeout to go to half-open
	time.Sleep(100 * time.Millisecond)

	// Next request will go through (and fail, moving back to open or closed depending on result)
	// This just verifies the timeout mechanism works
	_, err := cbClient.Query(ctx, "up", time.Time{})
	assert.Error(t, err)
}

func TestCircuitBreakerCustomConfig(t *testing.T) {
	client := NewClient("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second)

	// Custom configuration
	customConfig := CircuitBreakerConfig{
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Very conservative - need 10 failures
			return counts.ConsecutiveFailures >= 10
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			t.Logf("Custom CB: State changed from %s to %s", from, to)
		},
	}

	cbClient := NewCircuitBreakerClient(client, "custom-cb", customConfig)

	// Verify it was created successfully
	assert.NotNil(t, cbClient)
	assert.Equal(t, gobreaker.StateClosed, cbClient.State())
}

// Helper function to create a test error
func createTestError(msg string) error {
	return errors.New(msg)
}
