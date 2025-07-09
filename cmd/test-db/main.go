package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/seanankenbruck/observability-ai/internal/database"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

func main() {
	ctx := context.Background()

	// Database configuration
	config := semantic.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		Database: getEnv("DB_NAME", "observability_ai"),
		Username: getEnv("DB_USER", "obs_ai"),
		Password: getEnv("DB_PASSWORD", "changeme"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	fmt.Println("=== Observability AI Database Test ===")
	fmt.Printf("Connecting to database: %s@%s:%s/%s\n", config.Username, config.Host, config.Port, config.Database)

	// Test 1: Database Creation and Migration
	fmt.Println("\n1. Testing database connectivity and migration...")
	if err := testDatabaseSetup(config); err != nil {
		log.Fatalf("Database setup failed: %v", err)
	}
	fmt.Println("✓ Database setup successful")

	// Test 2: Initialize mapper
	fmt.Println("\n2. Initializing semantic mapper...")
	mapper, err := semantic.NewPostgresMapper(config)
	if err != nil {
		log.Fatalf("Failed to initialize mapper: %v", err)
	}
	defer mapper.Close()
	fmt.Println("✓ Semantic mapper initialized")

	// Test 3: Create example services
	fmt.Println("\n3. Creating example services...")
	services, err := createExampleServices(ctx, mapper)
	if err != nil {
		log.Fatalf("Failed to create services: %v", err)
	}
	fmt.Printf("✓ Created %d services\n", len(services))

	// Test 4: Create example metrics
	fmt.Println("\n4. Creating example metrics...")
	metrics, err := createExampleMetrics(ctx, mapper, services)
	if err != nil {
		log.Fatalf("Failed to create metrics: %v", err)
	}
	fmt.Printf("✓ Created %d metrics\n", len(metrics))

	// Test 5: Test service queries
	fmt.Println("\n5. Testing service queries...")
	if err := testServiceQueries(ctx, mapper); err != nil {
		log.Fatalf("Service query tests failed: %v", err)
	}
	fmt.Println("✓ Service queries working")

	// Test 6: Test metric queries
	fmt.Println("\n6. Testing metric queries...")
	if err := testMetricQueries(ctx, mapper, services); err != nil {
		log.Fatalf("Metric query tests failed: %v", err)
	}
	fmt.Println("✓ Metric queries working")

	// Test 7: Test query embeddings (with mock data)
	fmt.Println("\n7. Testing query embeddings...")
	if err := testQueryEmbeddings(ctx, mapper); err != nil {
		log.Fatalf("Query embedding tests failed: %v", err)
	}
	fmt.Println("✓ Query embeddings working")

	// Test 8: Search functionality
	fmt.Println("\n8. Testing search functionality...")
	if err := testSearchFunctionality(ctx, mapper); err != nil {
		log.Fatalf("Search tests failed: %v", err)
	}
	fmt.Println("✓ Search functionality working")

	fmt.Println("\n🎉 All database tests passed successfully!")
	fmt.Println("\nExample data created:")
	if err := printDatabaseSummary(ctx, mapper); err != nil {
		log.Printf("Warning: Failed to print summary: %v", err)
	}
}

func testDatabaseSetup(config semantic.PostgresConfig) error {
	// Verify database connectivity
	if err := database.CreateDatabase(config.Host, config.Port, config.Username, config.Password, config.Database); err != nil {
		return fmt.Errorf("failed to verify database connectivity: %w", err)
	}

	// Run migrations
	migrationConfig := database.MigrationConfig{
		DatabaseURL: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			config.Username, config.Password, config.Host, config.Port, config.Database, config.SSLMode),
		MigrationsPath: "./migrations",
	}

	if err := database.RunMigrations(migrationConfig); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func createExampleServices(ctx context.Context, mapper semantic.Mapper) ([]semantic.Service, error) {
	serviceDefinitions := []struct {
		name      string
		namespace string
		labels    map[string]string
	}{
		{
			name:      "user-service",
			namespace: "production",
			labels: map[string]string{
				"team":    "backend",
				"version": "v1.2.3",
				"tier":    "critical",
			},
		},
		{
			name:      "payment-service",
			namespace: "production",
			labels: map[string]string{
				"team":    "payments",
				"version": "v2.1.0",
				"tier":    "critical",
			},
		},
		{
			name:      "notification-service",
			namespace: "production",
			labels: map[string]string{
				"team":    "platform",
				"version": "v1.0.5",
				"tier":    "standard",
			},
		},
		{
			name:      "analytics-service",
			namespace: "staging",
			labels: map[string]string{
				"team":    "data",
				"version": "v0.9.2",
				"tier":    "experimental",
			},
		},
	}

	var services []semantic.Service
	for _, def := range serviceDefinitions {
		service, err := mapper.CreateService(ctx, def.name, def.namespace, def.labels)
		if err != nil {
			return nil, fmt.Errorf("failed to create service %s: %w", def.name, err)
		}
		services = append(services, *service)
		fmt.Printf("  Created service: %s (%s)\n", service.Name, service.Namespace)
	}

	return services, nil
}

func createExampleMetrics(ctx context.Context, mapper semantic.Mapper, services []semantic.Service) ([]semantic.Metric, error) {
	metricDefinitions := []struct {
		name        string
		metricType  string
		description string
		labels      map[string]string
	}{
		{
			name:        "http_requests_total",
			metricType:  "counter",
			description: "Total number of HTTP requests",
			labels: map[string]string{
				"method": "GET,POST,PUT,DELETE",
				"status": "200,400,500",
			},
		},
		{
			name:        "http_request_duration_seconds",
			metricType:  "histogram",
			description: "HTTP request duration in seconds",
			labels: map[string]string{
				"method": "GET,POST,PUT,DELETE",
			},
		},
		{
			name:        "database_connections_active",
			metricType:  "gauge",
			description: "Number of active database connections",
			labels:      map[string]string{},
		},
		{
			name:        "queue_messages_processed_total",
			metricType:  "counter",
			description: "Total number of queue messages processed",
			labels: map[string]string{
				"queue":  "email,sms,push",
				"status": "success,failure",
			},
		},
	}

	var allMetrics []semantic.Metric
	for _, service := range services {
		var serviceMetrics []string

		for _, metricDef := range metricDefinitions {
			metric, err := mapper.CreateMetric(ctx, metricDef.name, metricDef.metricType, metricDef.description, service.ID, metricDef.labels)
			if err != nil {
				return nil, fmt.Errorf("failed to create metric %s for service %s: %w", metricDef.name, service.Name, err)
			}
			allMetrics = append(allMetrics, *metric)
			serviceMetrics = append(serviceMetrics, metricDef.name)
		}

		// Update service with metric names
		if err := mapper.UpdateServiceMetrics(ctx, service.ID, serviceMetrics); err != nil {
			return nil, fmt.Errorf("failed to update metrics for service %s: %w", service.Name, err)
		}

		fmt.Printf("  Created %d metrics for %s\n", len(serviceMetrics), service.Name)
	}

	return allMetrics, nil
}

func testServiceQueries(ctx context.Context, mapper semantic.Mapper) error {
	// Test GetServices
	services, err := mapper.GetServices(ctx)
	if err != nil {
		return fmt.Errorf("GetServices failed: %w", err)
	}
	fmt.Printf("  Found %d services\n", len(services))

	// Test GetServiceByName
	if len(services) > 0 {
		service, err := mapper.GetServiceByName(ctx, services[0].Name)
		if err != nil {
			return fmt.Errorf("GetServiceByName failed: %w", err)
		}
		fmt.Printf("  Retrieved service by name: %s\n", service.Name)
	}

	return nil
}

func testMetricQueries(ctx context.Context, mapper semantic.Mapper, services []semantic.Service) error {
	if len(services) == 0 {
		return fmt.Errorf("no services available for metric testing")
	}

	// Test GetMetrics
	metrics, err := mapper.GetMetrics(ctx, services[0].ID)
	if err != nil {
		return fmt.Errorf("GetMetrics failed: %w", err)
	}
	fmt.Printf("  Found %d metrics for service %s\n", len(metrics), services[0].Name)

	return nil
}

func testQueryEmbeddings(ctx context.Context, mapper semantic.Mapper) error {
	// Create mock embeddings (normally these would come from an LLM)
	testQueries := []struct {
		query     string
		promql    string
		embedding []float32
	}{
		{
			query:     "show error rate for user-service",
			promql:    `rate(http_requests_total{service="user-service",status=~"5.*"}[5m])`,
			embedding: generateMockEmbedding(1536, 1),
		},
		{
			query:     "display latency for payment service",
			promql:    `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket{service="payment-service"}[5m]))`,
			embedding: generateMockEmbedding(1536, 2),
		},
		{
			query:     "throughput of notification service",
			promql:    `rate(http_requests_total{service="notification-service"}[5m])`,
			embedding: generateMockEmbedding(1536, 3),
		},
	}

	// Store query embeddings
	for _, tq := range testQueries {
		err := mapper.StoreQueryEmbedding(ctx, tq.query, tq.embedding, tq.promql)
		if err != nil {
			return fmt.Errorf("failed to store query embedding: %w", err)
		}
		fmt.Printf("  Stored embedding for: %s\n", tq.query)
	}

	// Test similarity search
	searchEmbedding := generateMockEmbedding(1536, 1) // Similar to first query
	similarQueries, err := mapper.FindSimilarQueries(ctx, searchEmbedding)
	if err != nil {
		return fmt.Errorf("failed to find similar queries: %w", err)
	}
	fmt.Printf("  Found %d similar queries\n", len(similarQueries))

	for _, sq := range similarQueries {
		fmt.Printf("    - Similarity %.3f: %s\n", sq.Similarity, sq.Query)
	}

	return nil
}

func testSearchFunctionality(ctx context.Context, mapper semantic.Mapper) error {
	// Test service search
	searchResults, err := mapper.SearchServices(ctx, "user")
	if err != nil {
		return fmt.Errorf("SearchServices failed: %w", err)
	}
	fmt.Printf("  Search for 'user' found %d services\n", len(searchResults))

	searchResults, err = mapper.SearchServices(ctx, "production")
	if err != nil {
		return fmt.Errorf("SearchServices failed: %w", err)
	}
	fmt.Printf("  Search for 'production' found %d services\n", len(searchResults))

	return nil
}

func printDatabaseSummary(ctx context.Context, mapper semantic.Mapper) error {
	services, err := mapper.GetServices(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("\nServices (%d):\n", len(services))
	for _, service := range services {
		fmt.Printf("  - %s/%s (team: %s)\n",
			service.Namespace, service.Name,
			service.Labels["team"])

		metrics, err := mapper.GetMetrics(ctx, service.ID)
		if err != nil {
			continue
		}
		fmt.Printf("    Metrics: %d\n", len(metrics))
	}

	return nil
}

// generateMockEmbedding creates a simple mock embedding for testing
func generateMockEmbedding(dim int, seed int) []float32 {
	embedding := make([]float32, dim)
	for i := 0; i < dim; i++ {
		// Simple pattern based on seed and position
		embedding[i] = float32((seed*i+i)%1000) / 1000.0
	}
	return embedding
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
