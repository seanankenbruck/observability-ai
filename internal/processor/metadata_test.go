package processor

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataGenerator_GenerateMetadata(t *testing.T) {
	mg := NewMetadataGenerator()

	t.Run("time series visualization", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 1,
			Statistics: &ResultStats{
				Min:     10.0,
				Max:     100.0,
				Avg:     50.0,
				Current: 75.0,
				Trend:   "increasing",
			},
		}

		metadata := mg.GenerateMetadata("rate(http_requests_total[5m])", results)
		assert.Equal(t, "time_series", metadata.VisualizationType)
		assert.Contains(t, metadata.Recommendation, "graph showing the trend")
		assert.Contains(t, metadata.NextSteps, "View full time series in Grafana")
		assert.Contains(t, metadata.NextSteps, "Consider setting an upper threshold alert")
	})

	t.Run("stat visualization for single series", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 1,
			Statistics:  nil, // No statistics means instant query
		}

		metadata := mg.GenerateMetadata("up{job=\"api\"}", results)
		assert.Equal(t, "stat", metadata.VisualizationType)
		assert.Contains(t, metadata.Recommendation, "single value")
		// Check that any next step contains "alert"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "alert") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have alert-related next step")
	})

	t.Run("table visualization for multiple series", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 10,
			Statistics:  nil,
		}

		metadata := mg.GenerateMetadata("node_cpu_usage", results)
		assert.Equal(t, "table", metadata.VisualizationType)
		assert.Contains(t, metadata.Recommendation, "table")
		// Check that any next step contains "Sort" or "filter"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "Sort") || strings.Contains(step, "filter") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have sort/filter-related next step")
	})

	t.Run("truncated results", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 50,
			Truncated:   true,
		}

		metadata := mg.GenerateMetadata("some_metric", results)
		// Check that any next step contains "Grafana" and "all"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "Grafana") && strings.Contains(step, "all") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have Grafana-related next step for viewing all results")
	})

	t.Run("no data found", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 0,
		}

		metadata := mg.GenerateMetadata("non_existent_metric", results)
		assert.Contains(t, metadata.Recommendation, "No data found")

		// Check for metric name suggestion
		foundMetric := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "metric") && strings.Contains(step, "correct") {
				foundMetric = true
				break
			}
		}
		assert.True(t, foundMetric, "Should have metric name check suggestion")

		// Check for time range suggestion
		foundTime := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "time") && strings.Contains(step, "range") {
				foundTime = true
				break
			}
		}
		assert.True(t, foundTime, "Should have time range verification suggestion")

		// Check for label filter suggestion
		foundLabel := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "label") && strings.Contains(step, "filter") {
				foundLabel = true
				break
			}
		}
		assert.True(t, foundLabel, "Should have label filter confirmation suggestion")
	})

	t.Run("rate query suggestions", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 1,
		}

		metadata := mg.GenerateMetadata("rate(http_requests_total[5m])", results)
		// Check that any next step contains "Compare" and "historical" or "baseline"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "Compare") || strings.Contains(step, "historical") || strings.Contains(step, "baseline") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have historical comparison suggestion")
	})

	t.Run("many series aggregation suggestion", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 100,
		}

		metadata := mg.GenerateMetadata("node_cpu_usage", results)
		// Check that any next step contains "aggregat" and "label"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "aggregat") && strings.Contains(step, "label") {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have aggregation suggestion")
	})

	t.Run("decreasing trend alert", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 1,
			Statistics: &ResultStats{
				Trend: "decreasing",
			},
		}

		metadata := mg.GenerateMetadata("memory_available", results)
		// Check that any next step contains "lower" and "threshold" or "alert"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "lower") && (strings.Contains(step, "threshold") || strings.Contains(step, "alert")) {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have lower threshold alert suggestion")
	})

	t.Run("increasing trend alert", func(t *testing.T) {
		results := &QueryResults{
			TotalSeries: 1,
			Statistics: &ResultStats{
				Trend: "increasing",
			},
		}

		metadata := mg.GenerateMetadata("error_rate", results)
		// Check that any next step contains "upper" and "threshold" or "alert"
		found := false
		for _, step := range metadata.NextSteps {
			if strings.Contains(step, "upper") && (strings.Contains(step, "threshold") || strings.Contains(step, "alert")) {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have upper threshold alert suggestion")
	})
}

func TestMetadataGenerator_DetermineVisualizationType(t *testing.T) {
	mg := NewMetadataGenerator()

	tests := []struct {
		name     string
		results  *QueryResults
		expected string
	}{
		{
			name: "time series with statistics",
			results: &QueryResults{
				Statistics: &ResultStats{},
			},
			expected: "time_series",
		},
		{
			name: "single series instant query",
			results: &QueryResults{
				TotalSeries: 1,
				Statistics:  nil,
			},
			expected: "stat",
		},
		{
			name: "multiple series instant query",
			results: &QueryResults{
				TotalSeries: 5,
				Statistics:  nil,
			},
			expected: "table",
		},
		{
			name: "no series",
			results: &QueryResults{
				TotalSeries: 0,
			},
			expected: "table", // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vizType := mg.determineVisualizationType("test_query", tt.results)
			assert.Equal(t, tt.expected, vizType)
		})
	}
}

func TestMetadataGenerator_GenerateGrafanaLink(t *testing.T) {
	mg := NewMetadataGenerator()

	t.Run("generate link with valid URL", func(t *testing.T) {
		link := mg.GenerateGrafanaLink("http://grafana.example.com", "up")
		assert.Equal(t, "http://grafana.example.com/explore", link)
	})

	t.Run("empty URL returns empty link", func(t *testing.T) {
		link := mg.GenerateGrafanaLink("", "up")
		assert.Equal(t, "", link)
	})
}
