package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/seanankenbruck/observability-ai/internal/llm"
)

func main() {
	fmt.Println("=== Claude Client Test ===")

	// Get API key from environment
	apiKey := os.Getenv("CLAUDE_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set CLAUDE_API_KEY environment variable")
	}

	// Initialize Claude client
	fmt.Println("Initializing Claude client...")
	client, err := llm.NewClaudeClient(apiKey, "claude-3-haiku-20240307")
	if err != nil {
		log.Fatalf("Failed to create Claude client: %v", err)
	}
	fmt.Println("✓ Claude client created successfully")

	ctx := context.Background()

	// Test 1: Simple PromQL generation
	fmt.Println("\n1. Testing basic PromQL generation...")
	testBasicQuery(ctx, client)

	// Test 2: Error rate query
	fmt.Println("\n2. Testing error rate query...")
	testErrorRateQuery(ctx, client)

	// Test 3: Latency query
	fmt.Println("\n3. Testing latency query...")
	testLatencyQuery(ctx, client)

	// Test 4: Throughput query
	fmt.Println("\n4. Testing throughput query...")
	testThroughputQuery(ctx, client)

	// Test 5: Complex query with context
	fmt.Println("\n5. Testing complex query with context...")
	testComplexQuery(ctx, client)

	// Test 6: Embedding generation
	fmt.Println("\n6. Testing embedding generation...")
	testEmbeddings(ctx, client)

	fmt.Println("\n🎉 All Claude client tests completed!")
}

func testBasicQuery(ctx context.Context, client *llm.ClaudeClient) {
	prompt := "Convert this to PromQL: show me HTTP requests for my-service"

	response, err := client.GenerateQuery(ctx, prompt)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("  Query: %s\n", prompt)
	fmt.Printf("  PromQL: %s\n", response.PromQL)
	fmt.Printf("  Confidence: %.2f\n", response.Confidence)
	fmt.Printf("  Explanation: %s\n", truncateString(response.Explanation, 100))
	fmt.Println("  ✓ Basic query generation successful")
}

func testErrorRateQuery(ctx context.Context, client *llm.ClaudeClient) {
	prompt := `You are a PromQL expert. Convert this natural language query to PromQL.

Context: We have a service called "user-service" that exposes HTTP metrics.
Available metrics: http_requests_total{service, method, status}

Query: show me the error rate for user-service in the last 5 minutes

Return only valid PromQL query focusing on accuracy.`

	response, err := client.GenerateQuery(ctx, prompt)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("  PromQL: %s\n", response.PromQL)
	fmt.Printf("  Confidence: %.2f\n", response.Confidence)

	// Check if response looks like valid PromQL
	if containsPromQLPatterns(response.PromQL) {
		fmt.Println("  ✓ Error rate query looks valid")
	} else {
		fmt.Println("  ⚠️  Generated query might not be valid PromQL")
	}
}

func testLatencyQuery(ctx context.Context, client *llm.ClaudeClient) {
	prompt := `Convert to PromQL: What's the 95th percentile latency for payment-service?

Available metrics:
- http_request_duration_seconds_bucket{service="payment-service", le}
- http_request_duration_seconds_sum{service="payment-service"}
- http_request_duration_seconds_count{service="payment-service"}`

	response, err := client.GenerateQuery(ctx, prompt)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("  PromQL: %s\n", response.PromQL)
	fmt.Printf("  Confidence: %.2f\n", response.Confidence)

	if containsHistogramQuantile(response.PromQL) {
		fmt.Println("  ✓ Latency query includes histogram_quantile")
	} else {
		fmt.Println("  ⚠️  Expected histogram_quantile for percentile query")
	}
}

func testThroughputQuery(ctx context.Context, client *llm.ClaudeClient) {
	prompt := "Convert to PromQL: requests per second for notification-service over the last 10 minutes"

	response, err := client.GenerateQuery(ctx, prompt)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("  PromQL: %s\n", response.PromQL)
	fmt.Printf("  Confidence: %.2f\n", response.Confidence)

	if containsRate(response.PromQL) {
		fmt.Println("  ✓ Throughput query includes rate() function")
	} else {
		fmt.Println("  ⚠️  Expected rate() function for throughput query")
	}
}

func testComplexQuery(ctx context.Context, client *llm.ClaudeClient) {
	prompt := `You are a PromQL expert. Convert this natural language query to PromQL.

Service Context:
- Name: api-gateway
- Namespace: production
- Available metrics: http_requests_total, http_request_duration_seconds_bucket

Similar queries for reference:
- Query: show error rate for user-service
  PromQL: rate(http_requests_total{service="user-service",status=~"5.*"}[5m])

Query: Compare error rate between api-gateway and user-service for the last hour

Intent: comparison
Service: api-gateway
Time Range: 1h

Return only valid PromQL query. Focus on accuracy and performance.`

	response, err := client.GenerateQuery(ctx, prompt)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}

	fmt.Printf("  PromQL: %s\n", response.PromQL)
	fmt.Printf("  Confidence: %.2f\n", response.Confidence)
	fmt.Printf("  Explanation: %s\n", truncateString(response.Explanation, 150))
	fmt.Println("  ✓ Complex query with context successful")
}

func testEmbeddings(ctx context.Context, client *llm.ClaudeClient) {
	queries := []string{
		"show error rate for user-service",
		"display error rate for user service",
		"what is the latency for payment service",
		"throughput of notification service",
	}

	embeddings := make([][]float32, len(queries))

	for i, query := range queries {
		embedding, err := client.GetEmbedding(ctx, query)
		if err != nil {
			fmt.Printf("  ❌ Error generating embedding for '%s': %v\n", query, err)
			continue
		}
		embeddings[i] = embedding
		fmt.Printf("  Generated embedding for: '%s' (dim: %d)\n", query, len(embedding))
	}

	// Test similarity between similar queries
	if len(embeddings) >= 2 && embeddings[0] != nil && embeddings[1] != nil {
		similarity := cosineSimilarity(embeddings[0], embeddings[1])
		fmt.Printf("  Similarity between similar queries: %.3f\n", similarity)

		if similarity > 0.8 {
			fmt.Println("  ✓ Similar queries have high similarity")
		} else {
			fmt.Printf("  ⚠️  Expected higher similarity (got %.3f)\n", similarity)
		}
	}

	fmt.Println("  ✓ Embedding generation successful")
}

// Helper functions
func containsPromQLPatterns(query string) bool {
	patterns := []string{"rate(", "sum(", "avg(", "{", "}", "[", "]"}
	for _, pattern := range patterns {
		if !containsPattern(query, pattern) {
			return false
		}
	}
	return true
}

func containsHistogramQuantile(query string) bool {
	return containsPattern(query, "histogram_quantile")
}

func containsRate(query string) bool {
	return containsPattern(query, "rate(")
}

func containsPattern(text, pattern string) bool {
	return len(text) > 0 && len(pattern) > 0 &&
		(text == pattern || findSubstring(text, pattern))
}

func findSubstring(text, pattern string) bool {
	if len(pattern) > len(text) {
		return false
	}
	for i := 0; i <= len(text)-len(pattern); i++ {
		if text[i:i+len(pattern)] == pattern {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float32) float32 {
	if x == 0 {
		return 0
	}

	// Simple Newton's method for square root
	guess := x
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}
