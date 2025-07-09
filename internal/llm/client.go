package llm

import (
	"context"
)

// Client interface for AI service integration
type Client interface {
	GenerateQuery(ctx context.Context, prompt string) (*Response, error)
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Response represents the response from the AI service
type Response struct {
	PromQL      string  `json:"promql"`
	Explanation string  `json:"explanation"`
	Confidence  float64 `json:"confidence"`
}

// Config holds configuration for LLM clients
type Config struct {
	APIKey    string
	Model     string
	BaseURL   string
	Timeout   int
	MaxTokens int
}
