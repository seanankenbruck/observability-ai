// internal/mimir/discovery_test.go
package mimir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMapper is a mock implementation of semantic.Mapper for testing
type MockMapper struct {
	mu                     sync.Mutex
	services               map[string]*semantic.Service
	getServiceError        error
	createServiceError     error
	updateMetricsError     error
	servicesByName         map[string]*semantic.Service
	createServiceCallCount int
	updateMetricsCallCount int
}

func NewMockMapper() *MockMapper {
	return &MockMapper{
		services:       make(map[string]*semantic.Service),
		servicesByName: make(map[string]*semantic.Service),
	}
}

func (m *MockMapper) GetServices(ctx context.Context) ([]semantic.Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	services := make([]semantic.Service, 0, len(m.services))
	for _, s := range m.services {
		services = append(services, *s)
	}
	return services, nil
}

func (m *MockMapper) GetServiceByName(ctx context.Context, name, namespace string) (*semantic.Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.getServiceError != nil {
		return nil, m.getServiceError
	}

	key := fmt.Sprintf("%s/%s", namespace, name)
	if service, exists := m.servicesByName[key]; exists {
		return service, nil
	}
	return nil, errors.New("service not found")
}

func (m *MockMapper) CreateService(ctx context.Context, name, namespace string, labels map[string]string) (*semantic.Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.createServiceCallCount++

	if m.createServiceError != nil {
		return nil, m.createServiceError
	}

	service := &semantic.Service{
		ID:        fmt.Sprintf("service-%d", len(m.services)+1),
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	m.services[service.ID] = service
	key := fmt.Sprintf("%s/%s", namespace, name)
	m.servicesByName[key] = service

	return service, nil
}

func (m *MockMapper) UpdateServiceMetrics(ctx context.Context, serviceID string, metrics []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateMetricsCallCount++

	if m.updateMetricsError != nil {
		return m.updateMetricsError
	}

	if service, exists := m.services[serviceID]; exists {
		service.MetricNames = metrics
		service.UpdatedAt = time.Now().Format(time.RFC3339)
		return nil
	}
	return errors.New("service not found")
}

func (m *MockMapper) DeleteService(ctx context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, serviceID)
	return nil
}

func (m *MockMapper) SearchServices(ctx context.Context, searchTerm string) ([]semantic.Service, error) {
	return nil, nil
}

func (m *MockMapper) GetMetrics(ctx context.Context, serviceID string) ([]semantic.Metric, error) {
	return nil, nil
}

func (m *MockMapper) CreateMetric(ctx context.Context, name, metricType, description, serviceID string, labels map[string]string) (*semantic.Metric, error) {
	return nil, nil
}

func (m *MockMapper) FindSimilarQueries(ctx context.Context, embedding []float32) ([]semantic.SimilarQuery, error) {
	return nil, nil
}

func (m *MockMapper) StoreQueryEmbedding(ctx context.Context, query string, embedding []float32, promql string) error {
	return nil
}

// TestNewDiscoveryService tests creation of discovery service
func TestNewDiscoveryService(t *testing.T) {
	tests := []struct {
		name           string
		config         DiscoveryConfig
		expectedLabels []string
	}{
		{
			name: "default configuration",
			config: DiscoveryConfig{
				Enabled: true,
			},
			expectedLabels: []string{"service", "job", "app", "application"},
		},
		{
			name: "custom configuration",
			config: DiscoveryConfig{
				Enabled:           true,
				Interval:          10 * time.Minute,
				ServiceLabelNames: []string{"custom_service"},
				Namespaces:        []string{"production", "staging"},
				ExcludeMetrics:    []string{"^go_.*", "^process_.*"},
			},
			expectedLabels: []string{"custom_service"},
		},
		{
			name: "disabled discovery",
			config: DiscoveryConfig{
				Enabled: false,
			},
			expectedLabels: []string{"service", "job", "app", "application"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()

			ds := NewDiscoveryService(client, tt.config, mapper)

			require.NotNil(t, ds)
			assert.NotNil(t, ds.client)
			assert.NotNil(t, ds.mapper)
			assert.NotNil(t, ds.stopChan)
			assert.Equal(t, tt.expectedLabels, ds.config.ServiceLabelNames)

			// Check default interval
			if tt.config.Interval == 0 {
				assert.Equal(t, 5*time.Minute, ds.config.Interval)
			} else {
				assert.Equal(t, tt.config.Interval, ds.config.Interval)
			}

			// Check exclude patterns are compiled
			if len(tt.config.ExcludeMetrics) > 0 {
				assert.Len(t, ds.excludePatterns, len(tt.config.ExcludeMetrics))
			}
		})
	}
}

// TestFilterMetrics tests metric filtering
func TestFilterMetrics(t *testing.T) {
	tests := []struct {
		name            string
		excludePatterns []string
		metrics         []string
		expectedCount   int
		expectedMetrics []string
	}{
		{
			name:            "no exclusions",
			excludePatterns: []string{},
			metrics:         []string{"http_requests_total", "go_goroutines", "process_cpu_seconds_total"},
			expectedCount:   3,
			expectedMetrics: []string{"http_requests_total", "go_goroutines", "process_cpu_seconds_total"},
		},
		{
			name:            "exclude go metrics",
			excludePatterns: []string{"^go_.*"},
			metrics:         []string{"http_requests_total", "go_goroutines", "go_threads", "process_cpu_seconds_total"},
			expectedCount:   2,
			expectedMetrics: []string{"http_requests_total", "process_cpu_seconds_total"},
		},
		{
			name:            "exclude multiple patterns",
			excludePatterns: []string{"^go_.*", "^process_.*"},
			metrics:         []string{"http_requests_total", "go_goroutines", "process_cpu_seconds_total", "api_latency_seconds"},
			expectedCount:   2,
			expectedMetrics: []string{"http_requests_total", "api_latency_seconds"},
		},
		{
			name:            "exclude all metrics",
			excludePatterns: []string{".*"},
			metrics:         []string{"http_requests_total", "go_goroutines", "process_cpu_seconds_total"},
			expectedCount:   0,
			expectedMetrics: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()

			config := DiscoveryConfig{
				Enabled:        true,
				ExcludeMetrics: tt.excludePatterns,
			}

			ds := NewDiscoveryService(client, config, mapper)
			filtered := ds.filterMetrics(tt.metrics)

			assert.Len(t, filtered, tt.expectedCount)
			assert.Equal(t, tt.expectedMetrics, filtered)
		})
	}
}

// TestExtractServiceFromMetricName tests service name extraction from metric names
func TestExtractServiceFromMetricName(t *testing.T) {
	tests := []struct {
		metricName      string
		expectedService string
	}{
		// Pattern 1: service_metric_name
		{"frontend_errors_count", "frontend"},
		{"backend_latency_seconds", "backend"},
		{"userservice_requests_total", "userservice"},

		// Pattern 2: prefix_service_total
		{"grpc_backend_total", "backend"},
		{"http_userservice_total", "userservice"},

		// Pattern 3: prefix_service_count
		{"http_frontend_count", "frontend"},
		{"grpc_backend_count", "backend"},

		// Common metric words should be excluded
		{"http_requests_total", "unknown"},
		{"cpu_usage_percent", "unknown"},
		{"memory_bytes", "unknown"},
		{"process_cpu_seconds_total", "unknown"},
		{"api_requests_total", "unknown"}, // api is a common word

		// Complex patterns
		{"myservice_http_requests_total", "myservice"},
		{"user_service_latency_seconds", "user"},
		{"customapp_errors_total", "customapp"},
	}

	for _, tt := range tests {
		t.Run(tt.metricName, func(t *testing.T) {
			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()
			ds := NewDiscoveryService(client, DiscoveryConfig{Enabled: true}, mapper)

			result := ds.extractServiceFromMetricName(tt.metricName)
			assert.Equal(t, tt.expectedService, result)
		})
	}
}

// TestIsCommonMetricWord tests common metric word detection
func TestIsCommonMetricWord(t *testing.T) {
	tests := []struct {
		word       string
		isCommon   bool
	}{
		{"http", true},
		{"cpu", true},
		{"memory", true},
		{"latency", true},
		{"myservice", false},
		{"api", true},
		{"customapp", false},
		{"gauge", true},
		{"counter", true},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()
			ds := NewDiscoveryService(client, DiscoveryConfig{Enabled: true}, mapper)

			result := ds.isCommonMetricWord(tt.word)
			assert.Equal(t, tt.isCommon, result)
		})
	}
}

// TestDiscoverServicesWithMockedMimir tests service discovery with mocked Mimir responses
func TestDiscoverServicesWithMockedMimir(t *testing.T) {
	tests := []struct {
		name                 string
		metrics              []string
		labelResponses       map[string]map[string][]string // metric -> label -> values
		expectedServiceCount int
		expectedServices     []string
	}{
		{
			name:    "discover services from job label",
			metrics: []string{"http_requests_total", "http_errors_total"},
			labelResponses: map[string]map[string][]string{
				"http_requests_total": {
					"job":       {"api", "frontend"},
					"namespace": {"production"},
				},
				"http_errors_total": {
					"job":       {"api"},
					"namespace": {"production"},
				},
			},
			expectedServiceCount: 2,
			expectedServices:     []string{"api", "frontend"},
		},
		{
			name:    "discover services from service label",
			metrics: []string{"api_requests_total"},
			labelResponses: map[string]map[string][]string{
				"api_requests_total": {
					"service":   {"backend-api"},
					"namespace": {"staging"},
				},
			},
			expectedServiceCount: 1,
			expectedServices:     []string{"backend-api"},
		},
		{
			name:    "fallback to metric name extraction",
			metrics: []string{"myservice_requests_total"},
			labelResponses: map[string]map[string][]string{
				"myservice_requests_total": {},
			},
			expectedServiceCount: 1,
			expectedServices:     []string{"myservice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Mimir server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				metricName := r.URL.Query().Get("match[]")
				path := r.URL.Path

				// Handle label values requests
				if path == "/prometheus/api/v1/label/job/values" {
					if responses, ok := tt.labelResponses[metricName]; ok {
						if values, ok := responses["job"]; ok {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"status": "success",
								"data":   values,
							})
							return
						}
					}
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status": "success",
						"data":   []string{},
					})
				} else if path == "/prometheus/api/v1/label/service/values" {
					if responses, ok := tt.labelResponses[metricName]; ok {
						if values, ok := responses["service"]; ok {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"status": "success",
								"data":   values,
							})
							return
						}
					}
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status": "success",
						"data":   []string{},
					})
				} else if path == "/prometheus/api/v1/label/namespace/values" {
					if responses, ok := tt.labelResponses[metricName]; ok {
						if values, ok := responses["namespace"]; ok {
							json.NewEncoder(w).Encode(map[string]interface{}{
								"status": "success",
								"data":   values,
							})
							return
						}
					}
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status": "success",
						"data":   []string{"default"},
					})
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"data":   []interface{}{},
				})
			}))
			defer server.Close()

			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()
			ds := NewDiscoveryService(client, DiscoveryConfig{Enabled: true}, mapper)

			ctx := context.Background()
			services, err := ds.discoverServices(ctx, tt.metrics)

			require.NoError(t, err)
			assert.Len(t, services, tt.expectedServiceCount)

			// Check service names
			serviceNames := make(map[string]bool)
			for _, service := range services {
				serviceNames[service.Name] = true
			}

			for _, expectedName := range tt.expectedServices {
				assert.True(t, serviceNames[expectedName], "Expected service %s not found", expectedName)
			}
		})
	}
}

// TestUpdateDatabase tests database update functionality
func TestUpdateDatabase(t *testing.T) {
	tests := []struct {
		name                   string
		discoveredServices     []DiscoveredService
		existingServices       map[string]*semantic.Service
		expectedCreates        int
		expectedUpdates        int
		createServiceError     error
		updateMetricsError     error
	}{
		{
			name: "create new services",
			discoveredServices: []DiscoveredService{
				{
					Name:      "api",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"http_requests_total", "http_errors_total"},
				},
				{
					Name:      "frontend",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"http_requests_total"},
				},
			},
			existingServices: map[string]*semantic.Service{},
			expectedCreates:  2,
			expectedUpdates:  2,
		},
		{
			name: "update existing services",
			discoveredServices: []DiscoveredService{
				{
					Name:      "api",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"http_requests_total", "http_errors_total", "new_metric"},
				},
			},
			existingServices: map[string]*semantic.Service{
				"production/api": {
					ID:          "service-1",
					Name:        "api",
					Namespace:   "production",
					Labels:      map[string]string{"namespace": "production"},
					MetricNames: []string{"http_requests_total"},
				},
			},
			expectedCreates: 0,
			expectedUpdates: 1,
		},
		{
			name: "mixed create and update",
			discoveredServices: []DiscoveredService{
				{
					Name:      "api",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"http_requests_total"},
				},
				{
					Name:      "new-service",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"custom_metric"},
				},
			},
			existingServices: map[string]*semantic.Service{
				"production/api": {
					ID:        "service-1",
					Name:      "api",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
				},
			},
			expectedCreates: 1,
			expectedUpdates: 2,
		},
		{
			name: "handle create service error",
			discoveredServices: []DiscoveredService{
				{
					Name:      "api",
					Namespace: "production",
					Labels:    map[string]string{"namespace": "production"},
					Metrics:   []string{"http_requests_total"},
				},
			},
			existingServices:   map[string]*semantic.Service{},
			createServiceError: errors.New("database error"),
			expectedCreates:    1, // CreateService is called even if it fails
			expectedUpdates:    0, // No updates because creation failed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Mimir backend type explicitly for tests to avoid auto-detection
			client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
			mapper := NewMockMapper()

			// Setup existing services
			mapper.servicesByName = tt.existingServices
			for _, service := range tt.existingServices {
				mapper.services[service.ID] = service
			}

			// Setup errors
			mapper.createServiceError = tt.createServiceError
			mapper.updateMetricsError = tt.updateMetricsError

			ds := NewDiscoveryService(client, DiscoveryConfig{Enabled: true}, mapper)

			ctx := context.Background()
			updates, err := ds.updateDatabase(ctx, tt.discoveredServices)

			if tt.createServiceError != nil || tt.updateMetricsError != nil {
				assert.Equal(t, tt.expectedUpdates, updates)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUpdates, updates)
			}

			assert.Equal(t, tt.expectedCreates, mapper.createServiceCallCount)
		})
	}
}

// TestRunDiscovery tests full discovery cycle
func TestRunDiscovery(t *testing.T) {
	// Create mock Mimir server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/prometheus/api/v1/label/__name__/values" {
			// Return metric names
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": []string{
					"http_requests_total",
					"http_errors_total",
					"go_goroutines",
					"process_cpu_seconds_total",
				},
			})
		} else if path == "/prometheus/api/v1/label/service/values" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"api", "frontend"},
			})
		} else if path == "/prometheus/api/v1/label/namespace/values" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"production"},
			})
		} else if path == "/prometheus/api/v1/query" {
			// Connection test
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result":     []interface{}{},
				},
			})
		}
	}))
	defer server.Close()

	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
	mapper := NewMockMapper()

	config := DiscoveryConfig{
		Enabled:        true,
		ExcludeMetrics: []string{"^go_.*", "^process_.*"},
	}

	ds := NewDiscoveryService(client, config, mapper)
	ctx := context.Background()

	err := ds.runDiscovery(ctx)
	require.NoError(t, err)

	// Verify services were created
	assert.Greater(t, mapper.createServiceCallCount, 0)
	assert.Greater(t, mapper.updateMetricsCallCount, 0)
}

// TestDiscoveryServiceStartStop tests starting and stopping the discovery service
func TestDiscoveryServiceStartStop(t *testing.T) {
	// Create mock Mimir server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/prometheus/api/v1/query" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"resultType": "vector",
					"result":     []interface{}{},
				},
			})
		} else if r.URL.Path == "/prometheus/api/v1/label/__name__/values" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"test_metric"},
			})
		}
	}))
	defer server.Close()

	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
	mapper := NewMockMapper()

	config := DiscoveryConfig{
		Enabled:  true,
		Interval: 1 * time.Second, // Short interval for testing
	}

	ds := NewDiscoveryService(client, config, mapper)
	ctx := context.Background()

	// Test start
	err := ds.Start(ctx)
	require.NoError(t, err)
	assert.True(t, ds.running)

	// Test double start (should fail)
	err = ds.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Wait a bit to let it run
	time.Sleep(500 * time.Millisecond)

	// Test stop
	ds.Stop()
	assert.False(t, ds.running)

	// Test double stop (should be safe)
	ds.Stop()
	assert.False(t, ds.running)
}

// TestDiscoveryServiceDisabled tests that discovery doesn't run when disabled
func TestDiscoveryServiceDisabled(t *testing.T) {
	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend("http://localhost:9009", AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
	mapper := NewMockMapper()

	config := DiscoveryConfig{
		Enabled: false,
	}

	ds := NewDiscoveryService(client, config, mapper)
	ctx := context.Background()

	err := ds.Start(ctx)
	require.NoError(t, err)
	assert.False(t, ds.running)
}

// TestDiscoveryServiceConnectionFailure tests handling of Mimir connection failures
func TestDiscoveryServiceConnectionFailure(t *testing.T) {
	// Create server that always returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
	mapper := NewMockMapper()

	config := DiscoveryConfig{
		Enabled:  true,
		Interval: 1 * time.Second,
	}

	ds := NewDiscoveryService(client, config, mapper)
	ctx := context.Background()

	// Start should fail due to connection test failure
	err := ds.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Mimir")
}

// TestDiscoverServicesWithNamespaceFilter tests namespace filtering
func TestDiscoverServicesWithNamespaceFilter(t *testing.T) {
	// Create mock Mimir server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/prometheus/api/v1/label/service/values" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"api"},
			})
		} else if path == "/prometheus/api/v1/label/namespace/values" {
			// Return different namespaces for testing
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "success",
				"data":   []string{"production", "staging", "development"},
			})
		}
	}))
	defer server.Close()

	// Use Mimir backend type explicitly for tests to avoid auto-detection
	client := NewClientWithBackend(server.URL, AuthConfig{Type: "none"}, 5*time.Second, BackendTypeMimir)
	mapper := NewMockMapper()

	config := DiscoveryConfig{
		Enabled:    true,
		Namespaces: []string{"production", "staging"}, // Only these namespaces
	}

	ds := NewDiscoveryService(client, config, mapper)
	ctx := context.Background()

	metrics := []string{"http_requests_total"}
	services, err := ds.discoverServices(ctx, metrics)

	require.NoError(t, err)

	// Should only discover services in allowed namespaces
	for _, service := range services {
		assert.Contains(t, []string{"production", "staging"}, service.Namespace)
		assert.NotEqual(t, "development", service.Namespace)
	}
}
