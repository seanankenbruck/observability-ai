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
	Error     string   `json:"error,omitempty"`
	ErrorType string   `json:"errorType,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// QueryResult represents a processed query result supporting both instant and range queries
type QueryResult struct {
	ResultType string      // "vector", "matrix", "scalar", "string"
	Data       interface{} // Can be []InstantVector or []RangeVector
	Warnings   []string
}

// InstantVector represents a single metric value at a point in time
type InstantVector struct {
	Metric    map[string]string `json:"metric"`
	Value     [2]interface{}    `json:"value"` // [timestamp, value_string]
	Timestamp float64           `json:"-"`     // Parsed timestamp
	Val       float64           `json:"-"`     // Parsed value
}

// RangeVector represents a time series with multiple values
type RangeVector struct {
	Metric map[string]string `json:"metric"`
	Values [][2]interface{}  `json:"values"` // [[timestamp, value_string], ...]
}

// MetricMetadata represents metadata for a metric
type MetricMetadata struct {
	Type string `json:"type"` // "counter", "gauge", "histogram", "summary"
	Help string `json:"help"`
	Unit string `json:"unit"`
}

// Client is an HTTP client for communicating with Mimir/Prometheus API
type Client struct {
	endpoint     string
	auth         AuthConfig
	httpClient   *http.Client
	queryTimeout time.Duration // Default timeout for query operations
}

// NewClient creates a new Mimir client
func NewClient(endpoint string, auth AuthConfig, timeout time.Duration) *Client {
	// Default query timeout to 30s if not specified
	queryTimeout := 30 * time.Second
	if timeout > 0 {
		queryTimeout = timeout
	}

	return &Client{
		endpoint:     strings.TrimSuffix(endpoint, "/"),
		auth:         auth,
		queryTimeout: queryTimeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
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

// parseQueryResult converts QueryResponse to QueryResult with proper type handling
func parseQueryResult(qr *QueryResponse) (*QueryResult, error) {
	result := &QueryResult{
		ResultType: qr.Data.ResultType,
		Warnings:   qr.Warnings,
	}

	// Handle different result types
	switch qr.Data.ResultType {
	case "vector":
		// Instant vector query result
		vectors := []InstantVector{}
		if resultArray, ok := qr.Data.Result.([]interface{}); ok {
			for _, item := range resultArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					vector := InstantVector{}

					// Parse metric labels
					if metric, ok := itemMap["metric"].(map[string]interface{}); ok {
						vector.Metric = make(map[string]string)
						for k, v := range metric {
							if strVal, ok := v.(string); ok {
								vector.Metric[k] = strVal
							}
						}
					}

					// Parse value [timestamp, value_string]
					if value, ok := itemMap["value"].([]interface{}); ok && len(value) == 2 {
						vector.Value = [2]interface{}{value[0], value[1]}

						// Parse timestamp
						if ts, ok := value[0].(float64); ok {
							vector.Timestamp = ts
						}

						// Parse value
						if valStr, ok := value[1].(string); ok {
							var val float64
							fmt.Sscanf(valStr, "%f", &val)
							vector.Val = val
						}
					}

					vectors = append(vectors, vector)
				}
			}
		}
		result.Data = vectors

	case "matrix":
		// Range query result
		matrices := []RangeVector{}
		if resultArray, ok := qr.Data.Result.([]interface{}); ok {
			for _, item := range resultArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					matrix := RangeVector{}

					// Parse metric labels
					if metric, ok := itemMap["metric"].(map[string]interface{}); ok {
						matrix.Metric = make(map[string]string)
						for k, v := range metric {
							if strVal, ok := v.(string); ok {
								matrix.Metric[k] = strVal
							}
						}
					}

					// Parse values [[timestamp, value_string], ...]
					if values, ok := itemMap["values"].([]interface{}); ok {
						matrix.Values = make([][2]interface{}, 0, len(values))
						for _, val := range values {
							if valArray, ok := val.([]interface{}); ok && len(valArray) == 2 {
								matrix.Values = append(matrix.Values, [2]interface{}{valArray[0], valArray[1]})
							}
						}
					}

					matrices = append(matrices, matrix)
				}
			}
		}
		result.Data = matrices

	case "scalar":
		// Scalar result [timestamp, value_string]
		result.Data = qr.Data.Result

	case "string":
		// String result [timestamp, value_string]
		result.Data = qr.Data.Result

	default:
		return nil, fmt.Errorf("unsupported result type: %s", qr.Data.ResultType)
	}

	return result, nil
}

// QueryInstant executes an instant query (current values) and returns processed results
func (c *Client) QueryInstant(ctx context.Context, query string, timestamp time.Time) (*QueryResult, error) {
	// Add timeout to context if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.queryTimeout)
		defer cancel()
	}

	params := url.Values{}
	params.Set("query", query)
	if !timestamp.IsZero() {
		params.Set("time", fmt.Sprintf("%d", timestamp.Unix()))
	}

	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/query", params)
	if err != nil {
		// Handle context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timeout after %v: %w", c.queryTimeout, err)
		}
		return nil, fmt.Errorf("query request failed: %w", err)
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

	// Parse the response into QueryResult
	result, err := parseQueryResult(&queryResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query result: %w", err)
	}

	return result, nil
}

// Query executes an instant PromQL query
func (c *Client) Query(ctx context.Context, query string, timestamp time.Time) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	if !timestamp.IsZero() {
		params.Set("time", fmt.Sprintf("%d", timestamp.Unix()))
	}

	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/query", params)
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

// QueryRange executes a range PromQL query (deprecated - use QueryRangeWithResult)
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%d", int(step.Seconds())))

	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/query_range", params)
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

// QueryRangeWithResult executes a range query (time series) and returns processed results
func (c *Client) QueryRangeWithResult(ctx context.Context, query string, start, end time.Time, step time.Duration) (*QueryResult, error) {
	// Add timeout to context if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.queryTimeout)
		defer cancel()
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%d", int(step.Seconds())))

	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/query_range", params)
	if err != nil {
		// Handle context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query_range timeout after %v: %w", c.queryTimeout, err)
		}
		return nil, fmt.Errorf("query_range request failed: %w", err)
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

	// Parse the response into QueryResult
	result, err := parseQueryResult(&queryResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query_range result: %w", err)
	}

	return result, nil
}

// GetMetricNames retrieves all metric names from Mimir
func (c *Client) GetMetricNames(ctx context.Context) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/label/__name__/values", nil)
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

	path := fmt.Sprintf("/prometheus/api/v1/label/%s/values", url.PathEscape(labelName))
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

	resp, err := c.doRequest(ctx, "GET", "/prometheus/api/v1/metadata", params)
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
