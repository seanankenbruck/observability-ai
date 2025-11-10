// internal/processor/metadata.go
package processor

import "strings"

// MetadataGenerator generates visualization hints and recommendations for query results
type MetadataGenerator struct{}

// NewMetadataGenerator creates a new metadata generator
func NewMetadataGenerator() *MetadataGenerator {
	return &MetadataGenerator{}
}

// GenerateMetadata creates metadata with visualization hints and next steps
func (mg *MetadataGenerator) GenerateMetadata(query string, results *QueryResults) *ResultMetadata {
	metadata := &ResultMetadata{
		VisualizationType: mg.determineVisualizationType(query, results),
		Recommendation:    "",
		NextSteps:         []string{},
	}

	// Generate recommendation based on result type
	switch metadata.VisualizationType {
	case "time_series":
		metadata.Recommendation = "This query works best as a graph showing the trend over time"
		metadata.NextSteps = append(metadata.NextSteps, "View full time series in Grafana")

		// Add trend-specific recommendations
		if results.Statistics != nil {
			switch results.Statistics.Trend {
			case "increasing":
				metadata.NextSteps = append(metadata.NextSteps, "Consider setting an upper threshold alert")
			case "decreasing":
				metadata.NextSteps = append(metadata.NextSteps, "Consider setting a lower threshold alert")
			}
		}

	case "stat":
		metadata.Recommendation = "This query returns a single value, perfect for a stat panel"
		metadata.NextSteps = append(metadata.NextSteps, "Create an alert if value exceeds threshold")

		// Check if it's a rate or counter
		if strings.Contains(strings.ToLower(query), "rate(") || strings.Contains(strings.ToLower(query), "increase(") {
			metadata.NextSteps = append(metadata.NextSteps, "Compare with historical baselines")
		}

	case "table":
		metadata.Recommendation = "This query returns multiple series, best viewed as a table"
		metadata.NextSteps = append(metadata.NextSteps, "Sort or filter series in Grafana")

		// If many series, suggest aggregation
		if results.TotalSeries > 10 {
			metadata.NextSteps = append(metadata.NextSteps, "Consider aggregating by label to reduce series count")
		}
	}

	// Add common next steps based on result characteristics
	if results.Truncated {
		metadata.NextSteps = append(metadata.NextSteps, "View all results in Grafana - showing limited sample here")
	}

	// Warn if no data found
	if results.TotalSeries == 0 {
		metadata.Recommendation = "No data found for this query"
		metadata.NextSteps = []string{
			"Check if the metric name is correct",
			"Verify the time range includes data",
			"Confirm label filters are not too restrictive",
		}
	}

	return metadata
}

// determineVisualizationType analyzes the query and results to suggest the best visualization
func (mg *MetadataGenerator) determineVisualizationType(query string, results *QueryResults) string {
	// If we have statistics (range query), it's a time series
	if results.Statistics != nil {
		return "time_series"
	}

	// If single series, it's a stat
	if results.TotalSeries == 1 {
		return "stat"
	}

	// Multiple series without time range = table
	if results.TotalSeries > 1 {
		return "table"
	}

	// Default to table for safety
	return "table"
}

// GenerateGrafanaLink creates a deep link to Grafana (if Grafana URL is configured)
func (mg *MetadataGenerator) GenerateGrafanaLink(grafanaURL, query string) string {
	if grafanaURL == "" {
		return ""
	}

	// Construct a Grafana explore link
	// Format: http://grafana/explore?left={"queries":[{"expr":"query"}]}
	// For simplicity, we'll return a basic URL that can be enhanced
	return grafanaURL + "/explore"
}
