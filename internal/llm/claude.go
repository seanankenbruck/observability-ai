package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	ClaudeAPIBaseURL = "https://api.anthropic.com/v1"
	ClaudeVersion    = "2023-06-01"
	MaxTokens        = 1000
	Temperature      = 0.1 // Low temperature for consistent PromQL generation
)

// ClaudeClient implements the Client interface using Anthropic's Claude API
type ClaudeClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// Claude API request structures
type ClaudeRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature,omitempty"`
	Messages    []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Claude API response structures
type ClaudeResponse struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	Usage   Usage          `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Error response structure
type ClaudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type ClaudeErrorResponse struct {
	Error ClaudeError `json:"error"`
}

// NewClaudeClient creates a new Claude client
func NewClaudeClient(apiKey, model string) (*ClaudeClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if model == "" {
		model = "claude-3-5-sonnet-20241022" // Default to Claude 3.5 Sonnet (latest)
	}

	return &ClaudeClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: ClaudeAPIBaseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GenerateQuery sends a prompt to Claude and returns a PromQL query
func (c *ClaudeClient) GenerateQuery(ctx context.Context, prompt string) (*Response, error) {
	// Prepare the request
	request := ClaudeRequest{
		Model:       c.model,
		MaxTokens:   MaxTokens,
		Temperature: Temperature,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	// Send request to Claude
	response, err := c.sendClaudeRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Claude: %w", err)
	}

	// Extract PromQL from response
	promql, explanation, confidence := c.parseClaudeResponse(response)
	if promql == "" {
		return nil, fmt.Errorf("Claude did not return a valid PromQL query")
	}

	return &Response{
		PromQL:      promql,
		Explanation: explanation,
		Confidence:  confidence,
	}, nil
}

// GetEmbedding implements simple text-based similarity using basic string features
// Since Claude doesn't provide embeddings, we'll create a simple representation
func (c *ClaudeClient) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// This is a simple text-to-vector conversion for basic similarity matching
	// In production, you'd want to use a proper embedding model
	return c.createSimpleEmbedding(text), nil
}

// sendClaudeRequest handles the HTTP communication with Claude API
func (c *ClaudeClient) sendClaudeRequest(ctx context.Context, request ClaudeRequest) (*ClaudeResponse, error) {
	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", ClaudeVersion)

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp.StatusCode, body)
	}

	// Parse successful response
	var claudeResponse ClaudeResponse
	if err := json.Unmarshal(body, &claudeResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &claudeResponse, nil
}

// parseClaudeResponse extracts PromQL query, explanation, and confidence from Claude's response
func (c *ClaudeClient) parseClaudeResponse(response *ClaudeResponse) (promql, explanation string, confidence float64) {
	if len(response.Content) == 0 {
		return "", "", 0.0
	}

	text := response.Content[0].Text

	// Try to extract PromQL query from the response
	// Look for code blocks first (most reliable)
	codeBlockRegex := regexp.MustCompile("```(?:promql)?\n?(.*?)\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(text); len(matches) > 1 {
		extractedPromQL := strings.TrimSpace(matches[1])
		confidence := c.calculateConfidence(text, extractedPromQL)
		explanation := c.cleanExplanation(text, extractedPromQL)
		return extractedPromQL, explanation, confidence
	}

	// Look for lines that start with metric names or functions - but get the FULL query
	lines := strings.Split(text, "\n")
	var promqlLines []string
	var inPromQL bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Check if line looks like PromQL
		if c.looksLikePromQLLine(line) {
			inPromQL = true
			promqlLines = append(promqlLines, line)
		} else if inPromQL {
			// If we were in PromQL and this line doesn't look like PromQL, we're done
			break
		}
	}

	if len(promqlLines) > 0 {
		extractedPromQL := strings.Join(promqlLines, " ")
		confidence := c.calculateConfidence(text, extractedPromQL)
		explanation := c.cleanExplanation(text, extractedPromQL)
		return extractedPromQL, explanation, confidence
	}

	// Try to find multi-line PromQL expressions
	multiLineRegex := regexp.MustCompile(`(?s)\b(?:rate|sum|avg|histogram_quantile|increase|max|min)\s*\([^)]*\)[^.]*`)
	if matches := multiLineRegex.FindAllString(text, -1); len(matches) > 0 {
		// Take the longest match (most likely to be complete)
		longestMatch := ""
		for _, match := range matches {
			if len(match) > len(longestMatch) {
				longestMatch = match
			}
		}
		extractedPromQL := strings.TrimSpace(longestMatch)
		confidence := c.calculateConfidence(text, extractedPromQL)
		explanation := c.cleanExplanation(text, extractedPromQL)
		return extractedPromQL, explanation, confidence
	}

	// Last resort: try to find anything that contains metric patterns
	promqlRegex := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*\{[^}]*\}(?:\[[^\]]+\])?`)
	if matches := promqlRegex.FindString(text); matches != "" {
		// Try to expand to include surrounding context
		expandedRegex := regexp.MustCompile(`(?:rate|sum|avg|histogram_quantile|increase|max|min)\([^)]*` + regexp.QuoteMeta(matches) + `[^)]*\)|` + regexp.QuoteMeta(matches))
		if expandedMatch := expandedRegex.FindString(text); expandedMatch != "" {
			confidence := c.calculateConfidence(text, expandedMatch)
			explanation := c.cleanExplanation(text, expandedMatch)
			return expandedMatch, explanation, confidence
		}
	}

	// If we still haven't found anything, return the first substantial line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 10 && !strings.Contains(strings.ToLower(line), "here") && !strings.Contains(strings.ToLower(line), "query") {
			confidence := 0.3 // Low confidence for fallback
			explanation := c.cleanExplanation(text, line)
			return line, explanation, confidence
		}
	}

	// Final fallback - return the full text as PromQL with very low confidence
	confidence = 0.1 // Very low confidence for this fallback
	explanation = c.cleanExplanation(text, text)
	return strings.TrimSpace(text), explanation, confidence
}

// looksLikePromQLLine checks if a line looks like valid PromQL
func (c *ClaudeClient) looksLikePromQLLine(line string) bool {
	// Must contain either metric name patterns or PromQL functions
	hasMetricPattern := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*(\{[^}]*\})?`).MatchString(line)
	hasPromQLFunction := regexp.MustCompile(`(?:rate|sum|avg|histogram_quantile|increase|max|min|count)\s*\(`).MatchString(line)
	hasArithmetic := regexp.MustCompile(`[\+\-\*/]`).MatchString(line)

	// Should not contain common explanation words
	explanationWords := []string{"this query", "the query", "here is", "you can", "to get", "will show", "returns", "this will"}
	for _, word := range explanationWords {
		if strings.Contains(strings.ToLower(line), word) {
			return false
		}
	}

	// Should be reasonably short (most PromQL queries are under 300 chars per line)
	if len(line) > 400 {
		return false
	}

	return hasMetricPattern || hasPromQLFunction || (hasArithmetic && len(line) > 5)
}

// calculateConfidence estimates how confident we are in the response
func (c *ClaudeClient) calculateConfidence(fullText, promql string) float64 {
	confidence := 0.5 // Base confidence

	// Higher confidence if we found a PromQL query
	if promql != "" {
		confidence += 0.3
	}

	// Higher confidence if the response mentions PromQL concepts
	promqlKeywords := []string{"rate(", "sum(", "avg(", "histogram_quantile(", "by (", "without ("}
	for _, keyword := range promqlKeywords {
		if strings.Contains(strings.ToLower(fullText), strings.ToLower(keyword)) {
			confidence += 0.05
		}
	}

	// Lower confidence if the response seems uncertain
	uncertaintyPhrases := []string{"not sure", "might be", "could be", "I think", "perhaps"}
	for _, phrase := range uncertaintyPhrases {
		if strings.Contains(strings.ToLower(fullText), phrase) {
			confidence -= 0.1
		}
	}

	// Ensure confidence is between 0 and 1
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// cleanExplanation removes the PromQL query from the explanation to avoid duplication
func (c *ClaudeClient) cleanExplanation(fullText, promql string) string {
	explanation := fullText

	// Remove the extracted PromQL query from explanation
	if promql != "" {
		explanation = strings.ReplaceAll(explanation, promql, "")
	}

	// Remove code block markers
	explanation = regexp.MustCompile("```(?:promql)?\n?.*?\n?```").ReplaceAllString(explanation, "")

	// Clean up extra whitespace
	explanation = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(explanation, "\n")
	explanation = strings.TrimSpace(explanation)

	// If explanation is empty or too short, provide a default
	if len(explanation) < 10 {
		explanation = "PromQL query generated based on the natural language request."
	}

	return explanation
}

// handleAPIError processes Claude API errors
func (c *ClaudeClient) handleAPIError(statusCode int, body []byte) error {
	var errorResponse ClaudeErrorResponse
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return fmt.Errorf("API error %d: %s", statusCode, string(body))
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key: %s", errorResponse.Error.Message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded: %s", errorResponse.Error.Message)
	case http.StatusBadRequest:
		return fmt.Errorf("bad request: %s", errorResponse.Error.Message)
	case http.StatusInternalServerError:
		return fmt.Errorf("Claude API internal error: %s", errorResponse.Error.Message)
	default:
		return fmt.Errorf("Claude API error %d: %s", statusCode, errorResponse.Error.Message)
	}
}

// createSimpleEmbedding creates a basic text representation for similarity matching
// This is a placeholder until we integrate a proper embedding model
func (c *ClaudeClient) createSimpleEmbedding(text string) []float32 {
	// Simple approach: create features based on text characteristics
	// This won't be as good as real embeddings, but provides basic similarity matching

	const embeddingDim = 384 // Smaller dimension for simplicity
	embedding := make([]float32, embeddingDim)

	text = strings.ToLower(text)

	// Feature 1-50: Character frequencies
	charCounts := make(map[rune]int)
	for _, char := range text {
		charCounts[char]++
	}

	chars := "abcdefghijklmnopqrstuvwxyz0123456789 "
	for i, char := range chars {
		if i < 50 {
			if count, exists := charCounts[char]; exists {
				embedding[i] = float32(count) / float32(len(text))
			}
		}
	}

	// Feature 51-100: Common PromQL/observability keywords
	keywords := []string{
		"error", "rate", "latency", "response", "time", "service", "request",
		"http", "database", "queue", "metric", "alert", "monitor", "trace",
		"log", "status", "throughput", "performance", "availability", "uptime",
		"cpu", "memory", "disk", "network", "container", "pod", "node",
		"slow", "fast", "high", "low", "increase", "decrease", "spike",
		"average", "sum", "count", "max", "min", "percentile", "histogram",
		"gauge", "counter", "summary", "bucket", "label", "tag", "dimension",
		"dashboard", "graph", "chart", "visualization", "query", "search",
		"filter", "group", "aggregate", "compare",
	}

	for i, keyword := range keywords {
		if i+50 < embeddingDim {
			if strings.Contains(text, keyword) {
				embedding[i+50] = 1.0
			}
		}
	}

	// Feature 151-200: Text length and structure features
	if 150 < embeddingDim {
		embedding[150] = float32(len(text)) / 1000.0                            // Normalized text length
		embedding[151] = float32(strings.Count(text, " ")) / float32(len(text)) // Word density
		embedding[152] = float32(strings.Count(text, "?"))                      // Question marks
		embedding[153] = float32(strings.Count(text, "{"))                      // Curly braces (PromQL)
		embedding[154] = float32(strings.Count(text, "["))                      // Square brackets (PromQL)
	}

	// Normalize the embedding vector
	var magnitude float32
	for _, val := range embedding {
		magnitude += val * val
	}
	if magnitude > 0 {
		magnitude = float32(1.0 / (magnitude + 0.001)) // Add small epsilon to avoid division by zero
		for i := range embedding {
			embedding[i] *= magnitude
		}
	}

	return embedding
}
