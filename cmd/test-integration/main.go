package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/llm"
	"github.com/seanankenbruck/observability-ai/internal/processor"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

func main() {
	fmt.Println("=== Query Processor Integration Test ===")

	ctx := context.Background()

	// Check required environment variables
	if err := checkEnvironment(); err != nil {
		log.Fatal(err)
	}

	// Initialize all components
	components, err := initializeComponents()
	if err != nil {
		log.Fatalf("Failed to initialize components: %v", err)
	}
	defer components.cleanup()

	fmt.Println("✓ All components initialized successfully")

	// Ensure we have test data
	fmt.Println("\nSetting up test data...")
	if err := setupTestData(ctx, components.semanticMapper); err != nil {
		log.Fatalf("Failed to setup test data: %v", err)
	}
	fmt.Println("✓ Test data ready")

	// Run integration tests
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("RUNNING INTEGRATION TESTS")
	fmt.Println(strings.Repeat("=", 50))

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "Error Rate Query",
			query: "show me error rate for user-service in the last 5 minutes",
		},
		{
			name:  "Latency Query",
			query: "what is the 95th percentile latency for payment-service",
		},
		{
			name:  "Throughput Query",
			query: "requests per second for notification-service",
		},
		{
			name:  "Simple Metrics Query",
			query: "show http requests for user-service",
		},
		{
			name:  "Database Connections",
			query: "how many database connections does payment-service have",
		},
	}

	var successCount, totalCount int
	var totalProcessingTime time.Duration

	for i, test := range tests {
		fmt.Printf("\n%d. Testing: %s\n", i+1, test.name)
		fmt.Printf("   Query: \"%s\"\n", test.query)

		success, processingTime := runSingleTest(ctx, components.queryProcessor, test.query)
		if success {
			successCount++
			fmt.Printf("   ✓ SUCCESS (%.2fs)\n", processingTime.Seconds())
		} else {
			fmt.Printf("   ❌ FAILED (%.2fs)\n", processingTime.Seconds())
		}

		totalCount++
		totalProcessingTime += processingTime

		// Small delay between tests to be nice to the API
		time.Sleep(1 * time.Second)
	}

	// Test similarity search
	fmt.Printf("\n%d. Testing Similarity Search\n", len(tests)+1)
	if err := testSimilaritySearch(ctx, components.semanticMapper); err != nil {
		fmt.Printf("   ❌ Similarity search failed: %v\n", err)
	} else {
		fmt.Printf("   ✓ Similarity search working\n")
		successCount++
	}
	totalCount++

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Total tests: %d\n", totalCount)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", totalCount-successCount)
	fmt.Printf("Success rate: %.1f%%\n", float64(successCount)/float64(totalCount)*100)
	fmt.Printf("Average processing time: %.2fs\n", totalProcessingTime.Seconds()/float64(totalCount))

	if successCount == totalCount {
		fmt.Println("\n🎉 All integration tests passed!")
	} else {
		fmt.Printf("\n⚠️  %d tests failed. Check the output above for details.\n", totalCount-successCount)
	}
}

type Components struct {
	queryProcessor *processor.QueryProcessor
	semanticMapper semantic.Mapper
	llmClient      llm.Client
	redisClient    *redis.Client
}

func (c *Components) cleanup() {
	if c.redisClient != nil {
		c.redisClient.Close()
	}
	// Note: semanticMapper cleanup would be handled by its Close() method if available
}

func checkEnvironment() error {
	required := []string{"CLAUDE_API_KEY", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD"}

	for _, env := range required {
		if os.Getenv(env) == "" {
			return fmt.Errorf("required environment variable %s is not set", env)
		}
	}

	return nil
}

func initializeComponents() (*Components, error) {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	// Test Redis connection
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize Claude client
	claudeClient, err := llm.NewClaudeClient(
		getEnv("CLAUDE_API_KEY", ""),
		getEnv("CLAUDE_MODEL", ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Claude client: %w", err)
	}

	// Initialize semantic mapper
	semanticMapper, err := semantic.NewPostgresMapper(semantic.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		Database: getEnv("DB_NAME", "observability_ai"),
		Username: getEnv("DB_USER", "obs_ai"),
		Password: getEnv("DB_PASSWORD", "changeme"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize semantic mapper: %w", err)
	}

	// Initialize query processor
	queryProcessor := processor.NewQueryProcessor(claudeClient, semanticMapper, rdb)

	return &Components{
		queryProcessor: queryProcessor,
		semanticMapper: semanticMapper,
		llmClient:      claudeClient,
		redisClient:    rdb,
	}, nil
}

func setupTestData(ctx context.Context, mapper semantic.Mapper) error {
	// Check if we already have services
	services, err := mapper.GetServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get services: %w", err)
	}

	if len(services) >= 3 {
		fmt.Printf("Found %d existing services, using existing data\n", len(services))
		return nil
	}

	// Create minimal test services if they don't exist
	testServices := []struct {
		name      string
		namespace string
		labels    map[string]string
		metrics   []string
	}{
		{
			name:      "user-service",
			namespace: "production",
			labels:    map[string]string{"team": "backend"},
			metrics:   []string{"http_requests_total", "http_request_duration_seconds"},
		},
		{
			name:      "payment-service",
			namespace: "production",
			labels:    map[string]string{"team": "payments"},
			metrics:   []string{"http_requests_total", "database_connections_active"},
		},
		{
			name:      "notification-service",
			namespace: "production",
			labels:    map[string]string{"team": "platform"},
			metrics:   []string{"http_requests_total", "queue_messages_processed_total"},
		},
	}

	for _, svc := range testServices {
		// Check if service already exists
		existing, err := mapper.GetServiceByName(ctx, svc.name)
		if err == nil && existing != nil {
			continue // Service already exists
		}

		// Create service
		service, err := mapper.CreateService(ctx, svc.name, svc.namespace, svc.labels)
		if err != nil {
			return fmt.Errorf("failed to create service %s: %w", svc.name, err)
		}

		// Update with metrics
		if err := mapper.UpdateServiceMetrics(ctx, service.ID, svc.metrics); err != nil {
			return fmt.Errorf("failed to update metrics for %s: %w", svc.name, err)
		}

		fmt.Printf("Created service: %s\n", svc.name)
	}

	return nil
}

func runSingleTest(ctx context.Context, qp *processor.QueryProcessor, naturalQuery string) (success bool, processingTime time.Duration) {
	req := &processor.QueryRequest{
		Query:  naturalQuery,
		UserID: "test-user",
	}

	start := time.Now()
	response, err := qp.ProcessQuery(ctx, req)
	processingTime = time.Since(start)

	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		return false, processingTime
	}

	// Validate response
	if response.PromQL == "" {
		fmt.Printf("   Error: Empty PromQL response\n")
		return false, processingTime
	}

	// Print results
	fmt.Printf("   PromQL: %s\n", response.PromQL)
	fmt.Printf("   Confidence: %.2f\n", response.Confidence)
	fmt.Printf("   Cache Hit: %v\n", response.CacheHit)
	fmt.Printf("   Estimated Cost: %d\n", response.EstimatedCost)

	if response.Explanation != "" {
		fmt.Printf("   Explanation: %s\n", truncateString(response.Explanation, 100))
	}

	// Basic validation - check if it looks like PromQL
	if !looksLikePromQL(response.PromQL) {
		fmt.Printf("   Warning: Response doesn't look like valid PromQL\n")
		return false, processingTime
	}

	return true, processingTime
}

func testSimilaritySearch(ctx context.Context, mapper semantic.Mapper) error {
	// This tests that embeddings were stored and can be retrieved

	// Create a test embedding (this mimics what the LLM client would do)
	testEmbedding := make([]float32, 1536)
	for i := range testEmbedding {
		testEmbedding[i] = float32(i) / float32(len(testEmbedding))
	}

	// Search for similar queries
	similarQueries, err := mapper.FindSimilarQueries(ctx, testEmbedding)
	if err != nil {
		return fmt.Errorf("similarity search failed: %w", err)
	}

	fmt.Printf("   Found %d similar queries in database\n", len(similarQueries))

	for i, sq := range similarQueries {
		if i >= 3 { // Show max 3 examples
			break
		}
		fmt.Printf("     - \"%s\" (similarity: %.3f)\n", sq.Query, sq.Similarity)
	}

	return nil
}

func looksLikePromQL(query string) bool {
	// Basic validation - check for common PromQL patterns
	hasMetricName := false
	hasPromQLFunction := false

	// Check for metric name pattern (letters, numbers, underscores)
	for i, char := range query {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' {
			if i == 0 || query[i-1] == ' ' || query[i-1] == '(' {
				hasMetricName = true
				break
			}
		}
	}

	// Check for common PromQL functions
	promqlFunctions := []string{"rate(", "sum(", "avg(", "histogram_quantile(", "increase(", "max(", "min("}
	for _, fn := range promqlFunctions {
		if contains(query, fn) {
			hasPromQLFunction = true
			break
		}
	}

	// Should have either a metric name or a PromQL function
	return hasMetricName || hasPromQLFunction
}

func contains(text, substring string) bool {
	return len(text) >= len(substring) && findInString(text, substring)
}

func findInString(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
