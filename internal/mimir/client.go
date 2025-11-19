// internal/mimir/client.go
package mimir

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AuthConfig holds authentication configuration for Mimir
type AuthConfig struct {
	Type        string // "basic", "bearer", "none"
	Username    string
	Password    string
	BearerToken string
	TenantID    string // Mimir tenant/org ID (X-Scope-OrgID header)
}

// QueryResponse represents the response from Mimir query endpoints
type QueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string      `json:"resultType"`
		Result     interface{} `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// MetricMetadata represents metadata for a metric
type MetricMetadata struct {
	Type string `json:"type"` // "counter", "gauge", "histogram", "summary"
	Help string `json:"help"`
	Unit string `json:"unit"`
}

// BackendType represents the type of Prometheus-compatible backend
type BackendType string

const (
	BackendTypeAuto       BackendType = "auto"
	BackendTypeMimir      BackendType = "mimir"
	BackendTypePrometheus BackendType = "prometheus"
)

// Client is an HTTP client for communicating with Mimir/Prometheus API
type Client struct {
	endpoint    string
	auth        AuthConfig
	httpClient  *http.Client
	backendType BackendType
	apiPrefix   string // "/prometheus/api/v1" for Mimir, "/api/v1" for Prometheus
}

// NewClient creates a new Mimir client with default backend type (auto-detect)
func NewClient(endpoint string, auth AuthConfig, timeout time.Duration) *Client {
	return NewClientWithBackend(endpoint, auth, timeout, BackendTypeAuto)
}

// NewClientWithBackend creates a new client with a specific backend type
func NewClientWithBackend(endpoint string, auth AuthConfig, timeout time.Duration, backendType BackendType) *Client {
	client := &Client{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		auth:     auth,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		backendType: backendType,
	}

	// Set the API prefix based on backend type
	client.apiPrefix = client.determineAPIPrefix()

	return client
}

// determineAPIPrefix determines the correct API prefix based on backend type
func (c *Client) determineAPIPrefix() string {
	switch c.backendType {
	case BackendTypeMimir:
		return "/prometheus/api/v1"
	case BackendTypePrometheus:
		return "/api/v1"
	case BackendTypeAuto:
		// Try to auto-detect by checking which endpoint responds
		if c.detectBackendType() == BackendTypeMimir {
			return "/prometheus/api/v1"
		}
		return "/api/v1"
	default:
		// Default to Prometheus-style paths
		return "/api/v1"
	}
}

// detectBackendType attempts to detect the backend type by probing endpoints
func (c *Client) detectBackendType() BackendType {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try Prometheus-style endpoint first (more common)
	if c.testEndpoint(ctx, "/api/v1/query") {
		return BackendTypePrometheus
	}

	// Try Mimir-style endpoint
	if c.testEndpoint(ctx, "/prometheus/api/v1/query") {
		return BackendTypeMimir
	}

	// Default to Prometheus if detection fails
	return BackendTypePrometheus
}

// testEndpoint tests if an endpoint is accessible
func (c *Client) testEndpoint(ctx context.Context, path string) bool {
	// Use a simple query to test the endpoint
	params := url.Values{}
	params.Set("query", "up")
	params.Set("time", fmt.Sprintf("%d", time.Now().Unix()))

	reqURL := fmt.Sprintf("%s%s?%s", c.endpoint, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return false
	}

	// Add authentication
	switch c.auth.Type {
	case "basic":
		req.SetBasicAuth(c.auth.Username, c.auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
	}

	// Add Mimir tenant ID header if specified
	if c.auth.TenantID != "" {
		req.Header.Set("X-Scope-OrgID", c.auth.TenantID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider 2xx and 4xx responses as "accessible" (4xx means auth issue, but endpoint exists)
	// Only 404, 5xx, or connection errors mean the endpoint doesn't exist
	return resp.StatusCode != http.StatusNotFound && resp.StatusCode < 500
}

// doRequest executes an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, params url.Values) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s%s", c.endpoint, path)
	if params != nil && len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	switch c.auth.Type {
	case "basic":
		req.SetBasicAuth(c.auth.Username, c.auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
	case "none":
		// No authentication
	default:
		// No authentication
	}

	// Add Mimir tenant ID header (required for multi-tenant Mimir)
	if c.auth.TenantID != "" {
		req.Header.Set("X-Scope-OrgID", c.auth.TenantID)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// Query executes an instant PromQL query
func (c *Client) Query(ctx context.Context, query string, timestamp time.Time) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	if !timestamp.IsZero() {
		params.Set("time", fmt.Sprintf("%d", timestamp.Unix()))
	}

	resp, err := c.doRequest(ctx, "GET", c.apiPrefix+"/query", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if queryResp.Status != "success" {
		return nil, fmt.Errorf("query error: %s - %s", queryResp.ErrorType, queryResp.Error)
	}

	return &queryResp, nil
}

// QueryRange executes a range PromQL query
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%d", int(step.Seconds())))

	resp, err := c.doRequest(ctx, "GET", c.apiPrefix+"/query_range", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query_range failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.Unmarshal(body, &queryResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if queryResp.Status != "success" {
		return nil, fmt.Errorf("query_range error: %s - %s", queryResp.ErrorType, queryResp.Error)
	}

	return &queryResp, nil
}

// GetMetricNames retrieves all metric names from Mimir
func (c *Client) GetMetricNames(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", c.apiPrefix+"/label/__name__/values", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get metric names failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("get metric names failed")
	}

	return result.Data, nil
}

// GetLabelValues gets possible values for a specific label
func (c *Client) GetLabelValues(ctx context.Context, labelName string, metricMatchers ...string) ([]string, error) {
	params := url.Values{}
	if len(metricMatchers) > 0 {
		params.Set("match[]", metricMatchers[0])
	}

	path := fmt.Sprintf("%s/label/%s/values", c.apiPrefix, url.PathEscape(labelName))
	resp, err := c.doRequest(ctx, "GET", path, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get label values failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("get label values failed")
	}

	return result.Data, nil
}

// GetMetricMetadata retrieves metadata for a specific metric
func (c *Client) GetMetricMetadata(ctx context.Context, metricName string) (*MetricMetadata, error) {
	params := url.Values{}
	if metricName != "" {
		params.Set("metric", metricName)
	}

	resp, err := c.doRequest(ctx, "GET", c.apiPrefix+"/metadata", params)
	if err != nil {
		// Fallback to inferring type from metric name
		return &MetricMetadata{
			Type: inferMetricType(metricName),
			Help: "",
			Unit: "",
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &MetricMetadata{
			Type: inferMetricType(metricName),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		// Fallback to inferring type
		return &MetricMetadata{
			Type: inferMetricType(metricName),
		}, nil
	}

	var result struct {
		Status string                      `json:"status"`
		Data   map[string][]MetricMetadata `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &MetricMetadata{
			Type: inferMetricType(metricName),
		}, nil
	}

	if result.Status == "success" && len(result.Data[metricName]) > 0 {
		return &result.Data[metricName][0], nil
	}

	// Fallback to inferring type
	return &MetricMetadata{
		Type: inferMetricType(metricName),
	}, nil
}

// TestConnection tests connectivity to Mimir
func (c *Client) TestConnection(ctx context.Context) error {
	// Execute a simple query to test connectivity
	_, err := c.Query(ctx, "up", time.Time{})
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

// inferMetricType infers metric type from naming conventions
func inferMetricType(metricName string) string {
	if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") {
		return "counter"
	}
	if strings.Contains(metricName, "_bucket") || strings.Contains(metricName, "_histogram") {
		return "histogram"
	}
	if strings.Contains(metricName, "_duration") || strings.Contains(metricName, "_time") || strings.Contains(metricName, "_latency") {
		return "histogram"
	}
	if strings.Contains(metricName, "_summary") {
		return "summary"
	}
	return "gauge"
}
