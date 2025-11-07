package observability

import (
	"sync"
	"time"
)

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// Metric represents a single metric
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// MetricsCollector collects and stores application metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*Metric),
	}
}

// metricKey generates a unique key for a metric
func metricKey(name string, labels map[string]string) string {
	key := name
	if len(labels) > 0 {
		for k, v := range labels {
			key += "." + k + "=" + v
		}
	}
	return key
}

// Inc increments a counter metric
func (mc *MetricsCollector) Inc(name string, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		metric.Value++
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      MetricTypeCounter,
			Value:     1,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// Add adds a value to a counter metric
func (mc *MetricsCollector) Add(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		metric.Value += value
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      MetricTypeCounter,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// Set sets a gauge metric value
func (mc *MetricsCollector) Set(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := metricKey(name, labels)
	mc.metrics[key] = &Metric{
		Name:      name,
		Type:      MetricTypeGauge,
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// Observe records a histogram observation
func (mc *MetricsCollector) Observe(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		// Simple histogram - just tracking count and sum for now
		// In production, you'd use proper histogram buckets
		if metric.Extra == nil {
			metric.Extra = make(map[string]interface{})
		}
		count := 1.0
		sum := value
		if c, ok := metric.Extra["count"].(float64); ok {
			count = c + 1
		}
		if s, ok := metric.Extra["sum"].(float64); ok {
			sum = s + value
		}
		metric.Extra["count"] = count
		metric.Extra["sum"] = sum
		metric.Value = sum / count // average
		metric.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      MetricTypeHistogram,
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
			Extra: map[string]interface{}{
				"count": 1.0,
				"sum":   value,
			},
		}
	}
}

// Get retrieves a metric by name and labels
func (mc *MetricsCollector) Get(name string, labels map[string]string) (*Metric, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	key := metricKey(name, labels)
	metric, exists := mc.metrics[key]
	return metric, exists
}

// GetAll retrieves all metrics
func (mc *MetricsCollector) GetAll() map[string]*Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*Metric, len(mc.metrics))
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}

// Reset clears all metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics = make(map[string]*Metric)
}

// Standard metric names
const (
	// Query metrics
	MetricQueryTotal           = "query_processor_queries_total"
	MetricQueryDuration        = "query_processor_query_duration_seconds"
	MetricQuerySuccess         = "query_processor_queries_success_total"
	MetricQueryFailure         = "query_processor_queries_failure_total"
	MetricQueryCacheHits       = "query_processor_cache_hits_total"
	MetricQueryCacheMisses     = "query_processor_cache_misses_total"
	MetricQuerySafetyViolation = "query_processor_safety_violations_total"

	// LLM metrics
	MetricLLMRequests      = "llm_requests_total"
	MetricLLMDuration      = "llm_request_duration_seconds"
	MetricLLMTokens        = "llm_tokens_total"
	MetricLLMCost          = "llm_cost_dollars"
	MetricLLMErrors        = "llm_errors_total"
	MetricEmbeddingRequest = "llm_embedding_requests_total"

	// Database metrics
	MetricDBQueries        = "database_queries_total"
	MetricDBDuration       = "database_query_duration_seconds"
	MetricDBErrors         = "database_errors_total"
	MetricDBConnections    = "database_connections_active"
	MetricDBConnectionPool = "database_connection_pool_size"

	// Auth metrics
	MetricAuthAttempts       = "auth_attempts_total"
	MetricAuthSuccess        = "auth_success_total"
	MetricAuthFailure        = "auth_failure_total"
	MetricAuthTokensCreated  = "auth_tokens_created_total"
	MetricAuthSessionsActive = "auth_sessions_active"
	MetricAuthAPIKeyRequests = "auth_apikey_requests_total"

	// HTTP metrics
	MetricHTTPRequests     = "http_requests_total"
	MetricHTTPDuration     = "http_request_duration_seconds"
	MetricHTTPErrors       = "http_errors_total"
	MetricHTTPResponseSize = "http_response_size_bytes"

	// Discovery metrics
	MetricDiscoveryRuns       = "discovery_runs_total"
	MetricDiscoveryDuration   = "discovery_duration_seconds"
	MetricDiscoveryServices   = "discovery_services_found"
	MetricDiscoveryMetrics    = "discovery_metrics_found"
	MetricDiscoveryErrors     = "discovery_errors_total"
)

// Global metrics collector instance
var globalMetrics = NewMetricsCollector()

// GetGlobalMetrics returns the global metrics collector
func GetGlobalMetrics() *MetricsCollector {
	return globalMetrics
}

// RecordQueryMetrics records metrics for a query operation
func RecordQueryMetrics(duration time.Duration, success bool, cached bool, errorType string) {
	metrics := GetGlobalMetrics()

	labels := map[string]string{}
	if errorType != "" {
		labels["error_type"] = errorType
	}

	// Total queries
	metrics.Inc(MetricQueryTotal, nil)

	// Success/failure
	if success {
		metrics.Inc(MetricQuerySuccess, nil)
	} else {
		metrics.Inc(MetricQueryFailure, labels)
	}

	// Cache hits/misses
	if cached {
		metrics.Inc(MetricQueryCacheHits, nil)
	} else {
		metrics.Inc(MetricQueryCacheMisses, nil)
	}

	// Duration
	metrics.Observe(MetricQueryDuration, duration.Seconds(), nil)
}

// RecordLLMMetrics records metrics for LLM operations
func RecordLLMMetrics(operation string, duration time.Duration, tokens int, cost float64, err error) {
	metrics := GetGlobalMetrics()

	labels := map[string]string{"operation": operation}

	// Requests
	metrics.Inc(MetricLLMRequests, labels)

	// Duration
	metrics.Observe(MetricLLMDuration, duration.Seconds(), labels)

	// Tokens
	if tokens > 0 {
		metrics.Add(MetricLLMTokens, float64(tokens), labels)
	}

	// Cost
	if cost > 0 {
		metrics.Add(MetricLLMCost, cost, labels)
	}

	// Errors
	if err != nil {
		errorLabels := map[string]string{
			"operation": operation,
			"error":     err.Error(),
		}
		metrics.Inc(MetricLLMErrors, errorLabels)
	}
}

// RecordDBMetrics records metrics for database operations
func RecordDBMetrics(operation string, duration time.Duration, err error) {
	metrics := GetGlobalMetrics()

	labels := map[string]string{"operation": operation}

	// Queries
	metrics.Inc(MetricDBQueries, labels)

	// Duration
	metrics.Observe(MetricDBDuration, duration.Seconds(), labels)

	// Errors
	if err != nil {
		errorLabels := map[string]string{
			"operation": operation,
			"error":     err.Error(),
		}
		metrics.Inc(MetricDBErrors, errorLabels)
	}
}

// RecordHTTPMetrics records metrics for HTTP requests
func RecordHTTPMetrics(method, path string, statusCode int, duration time.Duration, responseSize int) {
	metrics := GetGlobalMetrics()

	labels := map[string]string{
		"method": method,
		"path":   path,
		"status": string(rune(statusCode)),
	}

	// Requests
	metrics.Inc(MetricHTTPRequests, labels)

	// Duration
	metrics.Observe(MetricHTTPDuration, duration.Seconds(), labels)

	// Errors (4xx, 5xx)
	if statusCode >= 400 {
		errorLabels := map[string]string{
			"method": method,
			"path":   path,
			"status": string(rune(statusCode)),
		}
		metrics.Inc(MetricHTTPErrors, errorLabels)
	}

	// Response size
	if responseSize > 0 {
		metrics.Observe(MetricHTTPResponseSize, float64(responseSize), labels)
	}
}
