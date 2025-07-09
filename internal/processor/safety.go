package processor

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SafetyChecker validates queries for safety
type SafetyChecker struct {
	MaxQueryRange    time.Duration
	MaxCardinality   int
	TimeoutSeconds   int
	ForbiddenMetrics []string
}

// NewSafetyChecker creates a new safety checker with default settings
func NewSafetyChecker() *SafetyChecker {
	return &SafetyChecker{
		MaxQueryRange:  7 * 24 * time.Hour, // 7 days
		MaxCardinality: 10000,
		TimeoutSeconds: 30,
		ForbiddenMetrics: []string{
			".*_secret.*",
			".*_password.*",
			".*_token.*",
			".*_key.*",
		},
	}
}

// ValidateQuery checks if a PromQL query is safe to execute
func (sc *SafetyChecker) ValidateQuery(promql string) error {
	// Check for forbidden metrics
	for _, forbidden := range sc.ForbiddenMetrics {
		if matched, _ := regexp.MatchString(forbidden, promql); matched {
			return fmt.Errorf("query contains forbidden metric pattern: %s", forbidden)
		}
	}

	// Check for excessively long time ranges
	if strings.Contains(promql, "[") {
		// This is a simplified check - in production, you'd parse the range properly
		dangerousRanges := []string{"365d", "1y", "52w", "8760h"}
		for _, dangerous := range dangerousRanges {
			if strings.Contains(promql, dangerous) {
				return fmt.Errorf("query time range exceeds maximum allowed: %s", dangerous)
			}
		}
	}

	// Check for high cardinality operations
	if strings.Contains(promql, "by ()") || strings.Contains(promql, "without ()") {
		return fmt.Errorf("query may produce high cardinality results")
	}

	// Check for potentially expensive operations
	expensiveOps := []string{
		"group_left",
		"group_right",
		"or vector",
		"absent(",
	}
	for _, op := range expensiveOps {
		if strings.Contains(strings.ToLower(promql), op) {
			return fmt.Errorf("query contains potentially expensive operation: %s", op)
		}
	}

	// Check for nested subqueries (can be very expensive)
	if strings.Count(promql, "(") > 3 {
		return fmt.Errorf("query contains too many nested operations")
	}

	return nil
}

// ValidateTimeRange checks if a time range is within safe limits
func (sc *SafetyChecker) ValidateTimeRange(timeRange string) error {
	// Parse common time range formats
	patterns := map[string]time.Duration{
		`(\d+)m`: time.Minute,
		`(\d+)h`: time.Hour,
		`(\d+)d`: 24 * time.Hour,
		`(\d+)w`: 7 * 24 * time.Hour,
	}

	for pattern, unit := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(timeRange); len(matches) > 1 {
			// Extract the number and calculate duration
			var multiplier int
			fmt.Sscanf(matches[1], "%d", &multiplier)
			duration := time.Duration(multiplier) * unit

			if duration > sc.MaxQueryRange {
				return fmt.Errorf("time range %s exceeds maximum allowed duration of %s", timeRange, sc.MaxQueryRange)
			}
		}
	}

	return nil
}

// EstimateCardinality provides a rough estimate of query result cardinality
func (sc *SafetyChecker) EstimateCardinality(promql string) int {
	cardinality := 1

	// Count label matchers
	labelMatchers := regexp.MustCompile(`\{[^}]+\}`).FindAllString(promql, -1)
	for _, matcher := range labelMatchers {
		// Each label matcher potentially increases cardinality
		labelCount := strings.Count(matcher, ",") + 1
		cardinality *= labelCount
	}

	// Aggregation reduces cardinality
	if strings.Contains(promql, "sum") || strings.Contains(promql, "avg") {
		cardinality /= 2
	}

	// Group by increases cardinality
	if strings.Contains(promql, "by (") {
		cardinality *= 10 // rough estimate
	}

	return cardinality
}
