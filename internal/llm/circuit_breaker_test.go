package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the Client interface
type MockClient struct {
	mock.Mock
}

func (m *MockClient) GenerateQuery(ctx context.Context, prompt string) (*Response, error) {
	args := m.Called(ctx, prompt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Response), args.Error(1)
}

func (m *MockClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float32), args.Error(1)
}

func TestCircuitBreakerClient_Success(t *testing.T) {
	// Create mock client
	mockClient := new(MockClient)
	expectedResponse := &Response{
		PromQL:      "up",
		Explanation: "Test query",
		Confidence:  0.9,
	}
	mockClient.On("GenerateQuery", mock.Anything, "test prompt").Return(expectedResponse, nil)

	// Create circuit breaker client
	cbClient := NewCircuitBreakerClient(mockClient, "test-cb", DefaultCircuitBreakerConfig)

	// Execute request
	response, err := cbClient.GenerateQuery(context.Background(), "test prompt")

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, response)
	assert.Equal(t, gobreaker.StateClosed, cbClient.State())
	mockClient.AssertExpectations(t)
}

func TestCircuitBreakerClient_OpensAfterFailures(t *testing.T) {
	// Create mock client that fails
	mockClient := new(MockClient)
	mockClient.On("GenerateQuery", mock.Anything, "test prompt").Return(nil, errors.New("service unavailable"))

	// Configure circuit breaker with lower threshold for testing
	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     100 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip after 3 consecutive failures
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			t.Logf("State changed from %s to %s", from, to)
		},
	}

	cbClient := NewCircuitBreakerClient(mockClient, "test-cb", config)

	// Make 3 failing requests to open the circuit
	for i := 0; i < 3; i++ {
		_, err := cbClient.GenerateQuery(context.Background(), "test prompt")
		assert.Error(t, err)
	}

	// Circuit should now be open
	assert.Equal(t, gobreaker.StateOpen, cbClient.State())

	// Next request should fail immediately without calling the client
	_, err := cbClient.GenerateQuery(context.Background(), "test prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreakerClient_HalfOpenRecovery(t *testing.T) {
	// Create mock client
	mockClient := new(MockClient)

	// First 3 calls fail
	mockClient.On("GenerateQuery", mock.Anything, "test prompt").Return(nil, errors.New("service unavailable")).Times(3)
	// Then succeed on the 4th call
	mockClient.On("GenerateQuery", mock.Anything, "test prompt").Return(&Response{PromQL: "up", Explanation: "success", Confidence: 0.9}, nil).Once()

	// Configure circuit breaker with short timeout
	config := CircuitBreakerConfig{
		MaxRequests: 1,
		Interval:    1 * time.Second,
		Timeout:     50 * time.Millisecond,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			t.Logf("State changed from %s to %s", from, to)
		},
	}

	cbClient := NewCircuitBreakerClient(mockClient, "test-cb", config)

	// Make 3 failing requests to open the circuit
	for i := 0; i < 3; i++ {
		_, err := cbClient.GenerateQuery(context.Background(), "test prompt")
		assert.Error(t, err)
	}

	// Circuit should be open
	assert.Equal(t, gobreaker.StateOpen, cbClient.State())

	// Wait for timeout to transition to half-open
	time.Sleep(100 * time.Millisecond)

	// Next request should succeed and close the circuit
	response, err := cbClient.GenerateQuery(context.Background(), "test prompt")
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "up", response.PromQL)

	// Circuit should be closed again
	assert.Equal(t, gobreaker.StateClosed, cbClient.State())
}

func TestCircuitBreakerClient_GetEmbedding(t *testing.T) {
	// Create mock client
	mockClient := new(MockClient)
	expectedEmbedding := []float32{0.1, 0.2, 0.3}
	mockClient.On("GetEmbedding", mock.Anything, "test text").Return(expectedEmbedding, nil)

	// Create circuit breaker client
	cbClient := NewCircuitBreakerClient(mockClient, "test-cb", DefaultCircuitBreakerConfig)

	// Execute request
	embedding, err := cbClient.GetEmbedding(context.Background(), "test text")

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedEmbedding, embedding)
	mockClient.AssertExpectations(t)
}

func TestCircuitBreakerCounts(t *testing.T) {
	// Create mock client
	mockClient := new(MockClient)
	mockClient.On("GenerateQuery", mock.Anything, "test prompt").Return(&Response{PromQL: "up"}, nil)

	cbClient := NewCircuitBreakerClient(mockClient, "test-cb", DefaultCircuitBreakerConfig)

	// Make several successful requests
	for i := 0; i < 5; i++ {
		_, err := cbClient.GenerateQuery(context.Background(), "test prompt")
		assert.NoError(t, err)
	}

	// Check counts
	counts := cbClient.Counts()
	assert.Equal(t, uint32(5), counts.Requests)
	assert.Equal(t, uint32(0), counts.TotalFailures)
	assert.Equal(t, uint32(0), counts.ConsecutiveFailures)
}
