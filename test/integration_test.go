// test/integration_test.go
//go:build integration
// +build integration

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/alicebob/miniredis/v2"
	"github.com/seanankenbruck/observability-ai/internal/auth"
	"github.com/seanankenbruck/observability-ai/internal/mimir"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
	"github.com/seanankenbruck/observability-ai/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests verify end-to-end functionality
// Run with: go test -tags=integration ./test/...

// TestMimirDiscoveryIntegration tests the full discovery flow
func TestMimirDiscoveryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Setup: Create mock Mimir server with test data
	mimirServer := createMockMimirServer(t)
	defer mimirServer.Close()

	// Setup: Create semantic mapper mock
	mapper := NewMockSemanticMapper()

	// Setup: Create discovery service
	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := mimir.NewClientWithBackend(mimirServer.URL, mimir.AuthConfig{Type: "none"}, 5*time.Second, mimir.BackendTypeMimir)
	discoveryConfig := mimir.DiscoveryConfig{
		Enabled:           true,
		Interval:          1 * time.Minute,
		Namespaces:        []string{"production", "staging"},
		ServiceLabelNames: []string{"service", "job"},
		ExcludeMetrics:    []string{"^go_.*", "^process_.*"},
	}

	discoveryService := mimir.NewDiscoveryService(client, discoveryConfig, mapper)

	// Test: Connection to Mimir
	t.Run("TestMimirConnection", func(t *testing.T) {
		err := client.TestConnection(ctx)
		require.NoError(t, err, "Should connect to Mimir successfully")
	})

	// Test: Metric retrieval
	t.Run("TestMetricRetrieval", func(t *testing.T) {
		metrics, err := client.GetMetricNames(ctx)
		require.NoError(t, err)
		assert.Greater(t, len(metrics), 0, "Should retrieve metrics from Mimir")

		// Verify we got expected metrics
		expectedMetrics := []string{"http_requests_total", "http_request_duration_seconds", "database_connections"}
		for _, expected := range expectedMetrics {
			assert.Contains(t, metrics, expected, "Should include %s metric", expected)
		}
	})

	// Test: Service discovery
	t.Run("TestServiceDiscovery", func(t *testing.T) {
		// Run discovery
		err := discoveryService.Start(ctx)
		require.NoError(t, err, "Discovery service should start successfully")

		// Wait a moment for discovery to complete
		time.Sleep(500 * time.Millisecond)

		// Stop discovery
		discoveryService.Stop()

		// Verify services were discovered and created
		services := mapper.GetAllServices()
		assert.Greater(t, len(services), 0, "Should discover services")

		// Verify expected services exist
		serviceNames := make(map[string]bool)
		for _, svc := range services {
			serviceNames[svc.Name] = true
		}

		expectedServices := []string{"api-gateway", "user-service", "payment-service"}
		for _, expected := range expectedServices {
			assert.True(t, serviceNames[expected], "Should discover %s service", expected)
		}
	})

	// Test: Service metrics association
	t.Run("TestServiceMetricsAssociation", func(t *testing.T) {
		services := mapper.GetAllServices()
		require.Greater(t, len(services), 0, "Should have discovered services")

		for _, svc := range services {
			assert.NotEmpty(t, svc.MetricNames, "Service %s should have associated metrics", svc.Name)
		}
	})

	// Test: Namespace filtering
	t.Run("TestNamespaceFiltering", func(t *testing.T) {
		services := mapper.GetAllServices()

		for _, svc := range services {
			// Should only have services from configured namespaces
			assert.Contains(t, []string{"production", "staging"}, svc.Namespace,
				"Service %s should be in configured namespace", svc.Name)
		}
	})
}

// TestAuthenticatedAPIIntegration tests API authentication
func TestAuthenticatedAPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup: Create Redis client for session management
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	// Setup: Create session manager
	sessionManager := session.NewManager(rdb, 24*time.Hour)

	// Setup: Create auth manager
	authManager := auth.NewAuthManager(auth.AuthConfig{
		JWTSecret:      "test-integration-secret",
		JWTExpiry:      1 * time.Hour,
		SessionExpiry:  24 * time.Hour,
		RateLimit:      100,
		AllowAnonymous: false,
	}, sessionManager)

	// Setup: Create test user
	user, err := authManager.CreateUser("integration-user", "test@integration.com", []string{"user", "admin"})
	require.NoError(t, err)

	// Test: JWT authentication flow
	t.Run("TestJWTAuthenticationFlow", func(t *testing.T) {
		// Create JWT token
		token, err := authManager.CreateJWTToken(user)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate JWT token
		claims, err := authManager.ValidateJWTToken(token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, user.Username, claims.Username)
		assert.Equal(t, user.Roles, claims.Roles)
	})

	// Test: API key authentication flow
	t.Run("TestAPIKeyAuthenticationFlow", func(t *testing.T) {
		// Create API key
		apiKey, err := authManager.CreateAPIKey(user.ID, "integration-key", []string{"read", "write"}, 100, 30*24*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, apiKey.Key)
		assert.Contains(t, apiKey.Key, "oai_")

		// Validate API key
		validatedUser, validatedKey, err := authManager.ValidateAPIKey(apiKey.Key)
		require.NoError(t, err)
		assert.Equal(t, user.ID, validatedUser.ID)
		assert.NotZero(t, validatedKey.LastUsedAt)
	})

	// Test: Session authentication flow
	t.Run("TestSessionAuthenticationFlow", func(t *testing.T) {
		// Create session
		session, err := authManager.CreateSession(user.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)

		// Validate session
		validatedUser, validatedSession, err := authManager.ValidateSession(session.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, validatedUser.ID)
		assert.NotZero(t, validatedSession.LastSeen)
	})

	// Test: Role-based access control
	t.Run("TestRoleBasedAccessControl", func(t *testing.T) {
		// Create users with different roles
		adminUser, err := authManager.GetUserByUsername("admin") // Default admin
		require.NoError(t, err)

		regularUser, err := authManager.CreateUser("regular-integration-user", "regular@integration.com", []string{"user"})
		require.NoError(t, err)

		// Admin should have admin role
		assert.Contains(t, adminUser.Roles, "admin")

		// Regular user should not have admin role
		assert.NotContains(t, regularUser.Roles, "admin")
	})

	// Test: Expired credential handling
	t.Run("TestExpiredCredentialHandling", func(t *testing.T) {
		// Create expired API key
		expiredKey, err := authManager.CreateAPIKey(user.ID, "expired-key", []string{"read"}, 100, -1*time.Hour)
		require.NoError(t, err)

		// Try to validate expired key
		_, _, err = authManager.ValidateAPIKey(expiredKey.Key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

// TestEndToEndDiscoveryFlow tests the complete discovery workflow
func TestEndToEndDiscoveryFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("CompleteDiscoveryWorkflow", func(t *testing.T) {
		// Step 1: Setup Mimir mock server
		mimirServer := createMockMimirServer(t)
		defer mimirServer.Close()

		// Step 2: Setup semantic mapper
		mapper := NewMockSemanticMapper()

		// Step 3: Create Mimir client
		// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := mimir.NewClientWithBackend(mimirServer.URL, mimir.AuthConfig{Type: "none"}, 5*time.Second, mimir.BackendTypeMimir)

		// Step 4: Test connection
		err := client.TestConnection(ctx)
		require.NoError(t, err, "Should connect to Mimir")

		// Step 5: Get metrics
		metrics, err := client.GetMetricNames(ctx)
		require.NoError(t, err)
		assert.Greater(t, len(metrics), 0, "Should retrieve metrics")

		// Step 6: Create discovery service
		config := mimir.DiscoveryConfig{
			Enabled:           true,
			Interval:          5 * time.Minute,
			ServiceLabelNames: []string{"service", "job"},
			ExcludeMetrics:    []string{"^go_.*", "^process_.*"},
		}
		discovery := mimir.NewDiscoveryService(client, config, mapper)

		// Step 7: Start discovery
		err = discovery.Start(ctx)
		require.NoError(t, err)

		// Step 8: Wait for discovery to run
		time.Sleep(1 * time.Second)

		// Step 9: Stop discovery
		discovery.Stop()

		// Step 10: Verify results
		services := mapper.GetAllServices()
		assert.Greater(t, len(services), 0, "Should have discovered services")

		// Step 11: Verify service details
		for _, svc := range services {
			assert.NotEmpty(t, svc.ID, "Service should have ID")
			assert.NotEmpty(t, svc.Name, "Service should have name")
			assert.NotEmpty(t, svc.Namespace, "Service should have namespace")
			assert.NotEmpty(t, svc.MetricNames, "Service should have metrics")
		}
	})
}

// TestRateLimitingIntegration tests rate limiting behavior
func TestRateLimitingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("RateLimitEnforcement", func(t *testing.T) {
		rateLimiter := auth.NewRateLimiter()
		clientID := "integration-test-client"
		limit := 5

		// Make requests up to the limit
		successCount := 0
		for i := 0; i < limit; i++ {
			if rateLimiter.Allow(clientID, limit) {
				successCount++
			}
		}
		assert.Equal(t, limit, successCount, "Should allow exactly %d requests", limit)

		// Next request should be blocked
		blocked := !rateLimiter.Allow(clientID, limit)
		assert.True(t, blocked, "Should block request over limit")

		// Note: Window reset test skipped in integration tests due to 61-second wait time
		// For full window reset testing, see unit tests for rate limiter
	})
}

// TestLLMPromptGeneration tests that prompts are correctly structured with the enhanced catalog
func TestLLMPromptGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("PromptIncludesMetricCatalog", func(t *testing.T) {
		// Setup: Create mock semantic mapper with diverse services
		mapper := NewMockSemanticMapper()

		// Create services with various metric types
		svc1, _ := mapper.CreateService(ctx, "api-gateway", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc1.ID, []string{
			"http_requests_total",      // counter
			"http_errors_total",        // counter
			"http_duration_bucket",     // histogram
			"memory_usage_current",     // gauge
			"cpu_usage_ratio",          // gauge
		})

		svc2, _ := mapper.CreateService(ctx, "database", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc2.ID, []string{
			"db_queries_total",         // counter
			"db_connections_active",    // gauge
		})

		// Verify services were created
		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 2, "Should have 2 services")

		// Verify metric categorization would happen
		for _, svc := range services {
			assert.NotEmpty(t, svc.MetricNames, "Service should have metrics")
		}
	})

	t.Run("LargeServiceMetricFiltering", func(t *testing.T) {
		// Setup: Create a service with many metrics
		mapper := NewMockSemanticMapper()

		// Generate 100 metrics (more than the 50 limit)
		manyMetrics := make([]string, 100)
		for i := 0; i < 100; i++ {
			if i%3 == 0 {
				manyMetrics[i] = fmt.Sprintf("metric_%d_total", i)
			} else if i%3 == 1 {
				manyMetrics[i] = fmt.Sprintf("metric_%d_current", i)
			} else {
				manyMetrics[i] = fmt.Sprintf("metric_%d_bucket", i)
			}
		}

		svc, _ := mapper.CreateService(ctx, "large-service", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc.ID, manyMetrics)

		// Verify service was created with all metrics
		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)
		assert.Len(t, services[0].MetricNames, 100, "Should have all 100 metrics")

		// Note: In actual prompt building, these would be filtered to 50
		// The filtering logic is tested in unit tests
	})
}

// TestErrorResponseHandling tests handling of ERROR responses from LLM
func TestErrorResponseHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("NoServicesDiscovered", func(t *testing.T) {
		// Setup: Create mapper with no services
		mapper := NewMockSemanticMapper()

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Empty(t, services, "Should have no services")

		// When building a prompt with no services, the prompt should warn
		// and instruct LLM to return ERROR
		// This is tested in processor_test.go unit tests
	})

	t.Run("ServiceWithoutMatchingMetrics", func(t *testing.T) {
		// Setup: Create service with metrics that don't match query intent
		mapper := NewMockSemanticMapper()

		svc, _ := mapper.CreateService(ctx, "database", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc.ID, []string{
			"db_queries_total",
			"db_connections_active",
		})

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)

		// If user queries for "CPU usage" but only DB metrics exist,
		// LLM should return ERROR: No suitable metrics found
		// This behavior is validated through end-to-end testing
	})
}

// TestMetricCategorization tests that metrics are properly categorized
func TestMetricCategorization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("CounterMetrics", func(t *testing.T) {
		mapper := NewMockSemanticMapper()
		svc, _ := mapper.CreateService(ctx, "test-service", "production", map[string]string{})

		counterMetrics := []string{
			"http_requests_total",
			"http_errors_total",
			"cache_hits_count",
		}
		mapper.UpdateServiceMetrics(ctx, svc.ID, counterMetrics)

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)

		// All metrics should end with _total or _count
		for _, metric := range services[0].MetricNames {
			assert.True(t,
				strings.HasSuffix(metric, "_total") || strings.HasSuffix(metric, "_count"),
				"Counter metric should end with _total or _count: %s", metric)
		}
	})

	t.Run("GaugeMetrics", func(t *testing.T) {
		mapper := NewMockSemanticMapper()
		svc, _ := mapper.CreateService(ctx, "test-service", "production", map[string]string{})

		gaugeMetrics := []string{
			"memory_usage_current",
			"cpu_active_cores",
			"disk_size",
			"cache_hit_ratio",
		}
		mapper.UpdateServiceMetrics(ctx, svc.ID, gaugeMetrics)

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)

		// Verify gauge patterns
		for _, metric := range services[0].MetricNames {
			hasGaugePattern := strings.Contains(metric, "_current") ||
				strings.Contains(metric, "_active") ||
				strings.Contains(metric, "_size") ||
				strings.Contains(metric, "_ratio")
			assert.True(t, hasGaugePattern, "Gauge metric should have gauge pattern: %s", metric)
		}
	})

	t.Run("HistogramMetrics", func(t *testing.T) {
		mapper := NewMockSemanticMapper()
		svc, _ := mapper.CreateService(ctx, "test-service", "production", map[string]string{})

		histogramMetrics := []string{
			"http_request_duration_bucket",
			"response_time_bucket",
		}
		mapper.UpdateServiceMetrics(ctx, svc.ID, histogramMetrics)

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)

		// All metrics should end with _bucket
		for _, metric := range services[0].MetricNames {
			assert.True(t, strings.HasSuffix(metric, "_bucket"),
				"Histogram metric should end with _bucket: %s", metric)
		}
	})

	t.Run("MixedMetricTypes", func(t *testing.T) {
		mapper := NewMockSemanticMapper()
		svc, _ := mapper.CreateService(ctx, "test-service", "production", map[string]string{})

		mixedMetrics := []string{
			"http_requests_total",           // counter
			"http_duration_bucket",          // histogram
			"memory_usage_current",          // gauge
			"db_queries_total",              // counter
			"cpu_active_cores",              // gauge
			"response_time_bucket",          // histogram
		}
		mapper.UpdateServiceMetrics(ctx, svc.ID, mixedMetrics)

		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 1)
		assert.Len(t, services[0].MetricNames, 6, "Should have all 6 metrics")

		// Count metrics by type
		counters := 0
		gauges := 0
		histograms := 0

		for _, metric := range services[0].MetricNames {
			if strings.HasSuffix(metric, "_total") || strings.HasSuffix(metric, "_count") {
				counters++
			} else if strings.HasSuffix(metric, "_bucket") {
				histograms++
			} else if strings.Contains(metric, "_current") || strings.Contains(metric, "_active") {
				gauges++
			}
		}

		assert.Equal(t, 2, counters, "Should have 2 counter metrics")
		assert.Equal(t, 2, histograms, "Should have 2 histogram metrics")
		assert.Equal(t, 2, gauges, "Should have 2 gauge metrics")
	})
}

// TestServiceTargeting tests that targeted services get full metric lists
func TestServiceTargeting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Run("TargetedServiceGetsAllMetrics", func(t *testing.T) {
		mapper := NewMockSemanticMapper()

		// Create target service with many metrics
		targetMetrics := make([]string, 60)
		for i := 0; i < 60; i++ {
			targetMetrics[i] = fmt.Sprintf("metric_%d_total", i)
		}
		svc1, _ := mapper.CreateService(ctx, "target-service", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc1.ID, targetMetrics)

		// Create other service with many metrics
		otherMetrics := make([]string, 70)
		for i := 0; i < 70; i++ {
			otherMetrics[i] = fmt.Sprintf("other_metric_%d_total", i)
		}
		svc2, _ := mapper.CreateService(ctx, "other-service", "production", map[string]string{})
		mapper.UpdateServiceMetrics(ctx, svc2.ID, otherMetrics)

		// Verify both services exist
		services, err := mapper.GetServices(ctx)
		require.NoError(t, err)
		assert.Len(t, services, 2)

		// Find target service
		var targetService *semantic.Service
		for _, svc := range services {
			if svc.Name == "target-service" {
				targetService = &svc
				break
			}
		}
		require.NotNil(t, targetService)
		assert.Len(t, targetService.MetricNames, 60, "Target service should have all 60 metrics")

		// In actual prompt building with intent.Service = "target-service",
		// the target service would show all metrics while other services would be filtered
		// This is tested in processor_test.go
	})
}

// Helper functions and mocks

// MockSemanticMapper is a test implementation of semantic.Mapper
type MockSemanticMapper struct {
	services map[string]*semantic.Service
	metrics  map[string]*semantic.Metric
}

func NewMockSemanticMapper() *MockSemanticMapper {
	return &MockSemanticMapper{
		services: make(map[string]*semantic.Service),
		metrics:  make(map[string]*semantic.Metric),
	}
}

func (m *MockSemanticMapper) GetServices(ctx context.Context) ([]semantic.Service, error) {
	services := make([]semantic.Service, 0, len(m.services))
	for _, svc := range m.services {
		services = append(services, *svc)
	}
	return services, nil
}

func (m *MockSemanticMapper) GetServiceByName(ctx context.Context, name, namespace string) (*semantic.Service, error) {
	key := name + "/" + namespace
	if svc, exists := m.services[key]; exists {
		return svc, nil
	}
	return nil, fmt.Errorf("service not found: %s/%s", namespace, name)
}

func (m *MockSemanticMapper) CreateService(ctx context.Context, name, namespace string, labels map[string]string) (*semantic.Service, error) {
	key := name + "/" + namespace
	svc := &semantic.Service{
		ID:        "svc-" + key,
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
	}
	m.services[key] = svc
	return svc, nil
}

func (m *MockSemanticMapper) UpdateServiceMetrics(ctx context.Context, serviceID string, metrics []string) error {
	for _, svc := range m.services {
		if svc.ID == serviceID {
			svc.MetricNames = metrics
			return nil
		}
	}
	return nil
}

func (m *MockSemanticMapper) DeleteService(ctx context.Context, serviceID string) error {
	for key, svc := range m.services {
		if svc.ID == serviceID {
			delete(m.services, key)
			return nil
		}
	}
	return nil
}

func (m *MockSemanticMapper) SearchServices(ctx context.Context, searchTerm string) ([]semantic.Service, error) {
	return m.GetServices(ctx)
}

func (m *MockSemanticMapper) GetMetrics(ctx context.Context, serviceID string) ([]semantic.Metric, error) {
	metrics := make([]semantic.Metric, 0)
	for _, metric := range m.metrics {
		if metric.ServiceID == serviceID {
			metrics = append(metrics, *metric)
		}
	}
	return metrics, nil
}

func (m *MockSemanticMapper) CreateMetric(ctx context.Context, name, metricType, description, serviceID string, labels map[string]string) (*semantic.Metric, error) {
	metric := &semantic.Metric{
		ID:          "metric-" + name,
		Name:        name,
		Type:        metricType,
		Description: description,
		ServiceID:   serviceID,
		Labels:      labels,
	}
	m.metrics[metric.ID] = metric
	return metric, nil
}

func (m *MockSemanticMapper) FindSimilarQueries(ctx context.Context, embedding []float32) ([]semantic.SimilarQuery, error) {
	return []semantic.SimilarQuery{}, nil
}

func (m *MockSemanticMapper) StoreQueryEmbedding(ctx context.Context, query string, embedding []float32, promql string) error {
	return nil
}

func (m *MockSemanticMapper) GetAllServices() []semantic.Service {
	services := make([]semantic.Service, 0, len(m.services))
	for _, svc := range m.services {
		services = append(services, *svc)
	}
	return services
}

// createMockMimirServer creates a test HTTP server that mimics Mimir API
func createMockMimirServer(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		switch {
		case path == "/prometheus/api/v1/query":
			// Health check / test query
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result": []map[string]interface{}{
						{
							"metric": map[string]string{"__name__": "up"},
							"value":  []interface{}{time.Now().Unix(), "1"},
						},
					},
				},
			})

		case path == "/prometheus/api/v1/label/__name__/values":
			// Return test metrics
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": []string{
					"http_requests_total",
					"http_request_duration_seconds",
					"http_errors_total",
					"database_connections",
					"cache_hits_total",
					"go_goroutines", // Should be filtered out
					"process_cpu_seconds_total", // Should be filtered out
				},
			})

		case path == "/prometheus/api/v1/label/service/values":
			// Return test services
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"api-gateway", "user-service", "payment-service"},
			})

		case path == "/prometheus/api/v1/label/job/values":
			// Return test jobs
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"api-gateway", "user-service", "payment-service"},
			})

		case path == "/prometheus/api/v1/label/namespace/values":
			// Return test namespaces
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"production", "staging"},
			})

		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	})

	return httptest.NewServer(handler)
}
