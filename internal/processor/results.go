// internal/processor/results.go
package processor

import (
	"fmt"
	"math"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/mimir"
)

const (
	MaxSamplesDefault = 10 // Maximum samples to return by default
	MaxTimePoints     = 50 // Maximum time points for range queries
)

// ResultProcessor handles formatting and processing of query results
type ResultProcessor struct {
	maxSamples    int
	maxTimePoints int
}

// NewResultProcessor creates a new result processor with default limits
func NewResultProcessor() *ResultProcessor {
	return &ResultProcessor{
		maxSamples:    MaxSamplesDefault,
		maxTimePoints: MaxTimePoints,
	}
}

// MetricSample represents a processed metric sample
type MetricSample struct {
	Metric    map[string]string `json:"metric"`
	Value     float64           `json:"value,omitempty"`     // For instant queries
	Values    []TimeValue       `json:"values,omitempty"`    // For range queries
	Timestamp time.Time         `json:"timestamp,omitempty"` // Sample timestamp
}

// TimeValue represents a single time-value pair
type TimeValue struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// QueryResults represents processed query results ready for presentation
type QueryResults struct {
	Summary     string         `json:"summary"`                // Human-readable summary
	Samples     []MetricSample `json:"samples"`                // Actual data points
	TotalSeries int            `json:"total_series"`           // Total number of series
	Truncated   bool           `json:"truncated"`              // Was data truncated?
	Statistics  *ResultStats   `json:"statistics,omitempty"`   // For range queries
	Warnings    []string       `json:"warnings,omitempty"`     // Warnings from Prometheus
}

// ResultStats provides statistical summary of range query results
type ResultStats struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
	Current float64 `json:"current"`
	Trend   string  `json:"trend"` // "increasing", "decreasing", "stable"
}

// ResultMetadata provides visualization hints and recommendations
type ResultMetadata struct {
	VisualizationType string   `json:"visualization_type"` // "time_series", "stat", "table"
	Recommendation    string   `json:"recommendation"`
	NextSteps         []string `json:"next_steps,omitempty"`
	GrafanaLink       string   `json:"grafana_link,omitempty"`
}

// ProcessResults converts Mimir results into user-friendly format
func (rp *ResultProcessor) ProcessResults(mimirResult *mimir.QueryResult) (*QueryResults, error) {
	switch mimirResult.ResultType {
	case "vector":
		return rp.processVector(mimirResult.Data, mimirResult.Warnings)
	case "matrix":
		return rp.processMatrix(mimirResult.Data, mimirResult.Warnings)
	case "scalar":
		return rp.processScalar(mimirResult.Data, mimirResult.Warnings)
	case "string":
		return rp.processString(mimirResult.Data, mimirResult.Warnings)
	default:
		return nil, fmt.Errorf("unsupported result type: %s", mimirResult.ResultType)
	}
}

// processVector handles instant query results (Tier 1 & 2)
func (rp *ResultProcessor) processVector(data interface{}, warnings []string) (*QueryResults, error) {
	vectors, ok := data.([]mimir.InstantVector)
	if !ok {
		return nil, fmt.Errorf("invalid vector data type")
	}

	// Convert to MetricSample format
	samples := make([]MetricSample, 0, len(vectors))
	for _, v := range vectors {
		samples = append(samples, MetricSample{
			Metric:    v.Metric,
			Value:     v.Val,
			Timestamp: time.Unix(int64(v.Timestamp), 0),
		})
	}

	results := &QueryResults{
		TotalSeries: len(samples),
		Truncated:   false,
		Warnings:    warnings,
	}

	// Tier 1: If results are small, return everything
	if len(samples) <= rp.maxSamples {
		results.Samples = samples
		results.Summary = rp.generateVectorSummary(samples, false)
		return results, nil
	}

	// Tier 2: Truncate if too large
	results.Samples = samples[:rp.maxSamples]
	results.Truncated = true
	results.Summary = rp.generateVectorSummary(samples, true)

	return results, nil
}

// generateVectorSummary creates a human-readable summary for instant queries
func (rp *ResultProcessor) generateVectorSummary(samples []MetricSample, truncated bool) string {
	if len(samples) == 0 {
		return "No data found"
	}

	if len(samples) == 1 {
		// Single series - be specific
		return fmt.Sprintf("Current value: %.2f", samples[0].Value)
	}

	// Multiple series - provide aggregate view
	var total, min, max float64
	min = samples[0].Value
	max = samples[0].Value

	for _, s := range samples {
		total += s.Value
		if s.Value < min {
			min = s.Value
		}
		if s.Value > max {
			max = s.Value
		}
	}

	summary := fmt.Sprintf("Found %d series: min=%.2f, max=%.2f, sum=%.2f", len(samples), min, max, total)

	if truncated {
		summary += fmt.Sprintf(" (showing first %d)", rp.maxSamples)
	}

	return summary
}

// processMatrix handles range query results (Tier 3)
func (rp *ResultProcessor) processMatrix(data interface{}, warnings []string) (*QueryResults, error) {
	matrices, ok := data.([]mimir.RangeVector)
	if !ok {
		return nil, fmt.Errorf("invalid matrix data type")
	}

	// Convert to MetricSample format
	samples := make([]MetricSample, 0, len(matrices))
	for _, m := range matrices {
		timeValues := make([]TimeValue, 0, len(m.Values))
		for _, v := range m.Values {
			ts, ok := v[0].(float64)
			if !ok {
				continue
			}
			valStr, ok := v[1].(string)
			if !ok {
				continue
			}
			var val float64
			fmt.Sscanf(valStr, "%f", &val)
			timeValues = append(timeValues, TimeValue{
				Timestamp: time.Unix(int64(ts), 0),
				Value:     val,
			})
		}
		samples = append(samples, MetricSample{
			Metric: m.Metric,
			Values: timeValues,
		})
	}

	results := &QueryResults{
		TotalSeries: len(samples),
		Warnings:    warnings,
	}

	// For range queries, compute statistics instead of returning all points
	if len(samples) > 0 {
		stats := rp.computeStatistics(samples)
		results.Statistics = stats

		// Sample time points (first, last, and evenly distributed middle points)
		sampledSeries := rp.sampleTimeSeries(samples, rp.maxTimePoints)
		results.Samples = sampledSeries

		results.Summary = rp.generateMatrixSummary(stats, len(samples))
	} else {
		results.Summary = "No data found"
	}

	return results, nil
}

// computeStatistics calculates min, max, avg, current, and trend from time series data
func (rp *ResultProcessor) computeStatistics(series []MetricSample) *ResultStats {
	var min, max, sum float64
	var count int
	var current float64
	var firstVal, lastVal float64
	firstValSet := false

	// Initialize with first value
	if len(series) > 0 && len(series[0].Values) > 0 {
		min = series[0].Values[0].Value
		max = series[0].Values[0].Value
		current = series[0].Values[len(series[0].Values)-1].Value
	}

	for _, s := range series {
		if len(s.Values) == 0 {
			continue
		}

		// Track first and last values for trend calculation
		if !firstValSet {
			firstVal = s.Values[0].Value
			firstValSet = true
		}
		lastVal = s.Values[len(s.Values)-1].Value

		for _, tv := range s.Values {
			if tv.Value < min {
				min = tv.Value
			}
			if tv.Value > max {
				max = tv.Value
			}
			sum += tv.Value
			count++
		}
	}

	avg := 0.0
	if count > 0 {
		avg = sum / float64(count)
	}

	// Determine trend (compare first vs last)
	trend := "stable"
	if firstValSet && count > 1 {
		// Avoid division by zero
		if math.Abs(firstVal) > 0.001 {
			percentChange := (lastVal - firstVal) / math.Abs(firstVal)
			if percentChange > 0.1 { // 10% increase threshold
				trend = "increasing"
			} else if percentChange < -0.1 { // 10% decrease threshold
				trend = "decreasing"
			}
		} else {
			// Handle case where first value is near zero
			if lastVal > firstVal+0.1 {
				trend = "increasing"
			} else if lastVal < firstVal-0.1 {
				trend = "decreasing"
			}
		}
	}

	return &ResultStats{
		Min:     min,
		Max:     max,
		Avg:     avg,
		Current: current,
		Trend:   trend,
	}
}

// generateMatrixSummary creates a human-readable summary for range queries
func (rp *ResultProcessor) generateMatrixSummary(stats *ResultStats, seriesCount int) string {
	return fmt.Sprintf(
		"%d series over time: min=%.2f, max=%.2f, avg=%.2f, current=%.2f (trend: %s)",
		seriesCount, stats.Min, stats.Max, stats.Avg, stats.Current, stats.Trend,
	)
}

// sampleTimeSeries reduces the number of time points while preserving key data points
func (rp *ResultProcessor) sampleTimeSeries(series []MetricSample, maxPoints int) []MetricSample {
	// For each series, sample time points evenly
	sampled := make([]MetricSample, len(series))

	for i, s := range series {
		if len(s.Values) <= maxPoints {
			sampled[i] = s
			continue
		}

		// Sample evenly: always include first and last, then evenly distribute middle
		sampledValues := make([]TimeValue, maxPoints)
		sampledValues[0] = s.Values[0]                        // First
		sampledValues[maxPoints-1] = s.Values[len(s.Values)-1] // Last

		// Evenly distribute middle points
		step := float64(len(s.Values)-1) / float64(maxPoints-1)
		for j := 1; j < maxPoints-1; j++ {
			idx := int(float64(j) * step)
			sampledValues[j] = s.Values[idx]
		}

		sampled[i] = MetricSample{
			Metric: s.Metric,
			Values: sampledValues,
		}
	}

	return sampled
}

// processScalar handles single-value results
func (rp *ResultProcessor) processScalar(data interface{}, warnings []string) (*QueryResults, error) {
	// Scalar result is [timestamp, value_string]
	scalarArray, ok := data.([]interface{})
	if !ok || len(scalarArray) != 2 {
		return nil, fmt.Errorf("invalid scalar data format")
	}

	ts, ok := scalarArray[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid scalar timestamp")
	}

	valStr, ok := scalarArray[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid scalar value")
	}

	var val float64
	fmt.Sscanf(valStr, "%f", &val)

	sample := MetricSample{
		Metric:    map[string]string{},
		Value:     val,
		Timestamp: time.Unix(int64(ts), 0),
	}

	return &QueryResults{
		Summary:     fmt.Sprintf("Scalar value: %.2f", val),
		Samples:     []MetricSample{sample},
		TotalSeries: 1,
		Truncated:   false,
		Warnings:    warnings,
	}, nil
}

// processString handles string results
func (rp *ResultProcessor) processString(data interface{}, warnings []string) (*QueryResults, error) {
	// String result is [timestamp, string_value]
	stringArray, ok := data.([]interface{})
	if !ok || len(stringArray) != 2 {
		return nil, fmt.Errorf("invalid string data format")
	}

	valStr, ok := stringArray[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid string value")
	}

	return &QueryResults{
		Summary:     fmt.Sprintf("String value: %s", valStr),
		Samples:     []MetricSample{},
		TotalSeries: 1,
		Truncated:   false,
		Warnings:    warnings,
	}, nil
}
