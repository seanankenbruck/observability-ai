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
	"testing"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/auth"
	"github.com/seanankenbruck/observability-ai/internal/mimir"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
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
	client := mimir.NewClient(mimirServer.URL, mimir.AuthConfig{Type: "none"}, 5*time.Second)
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

	// Setup: Create auth manager
	authManager := auth.NewAuthManager(auth.AuthConfig{
		JWTSecret:      "test-integration-secret",
		JWTExpiry:      1 * time.Hour,
		SessionExpiry:  24 * time.Hour,
		RateLimit:      100,
		AllowAnonymous: false,
	})

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
		client := mimir.NewClient(mimirServer.URL, mimir.AuthConfig{Type: "none"}, 5*time.Second)

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
