package processor

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/errors"
)

// SafetyChecker validates queries for safety
type SafetyChecker struct {
	MaxQueryRange    time.Duration
	MaxCardinality   int
	TimeoutSeconds   int
	ForbiddenMetrics []string
	MaxQueryLength   int // Maximum query length in characters
	ForbiddenPatterns []string // Additional forbidden patterns (compiled as case-insensitive)
}

// NewSafetyChecker creates a new safety checker with default settings
func NewSafetyChecker() *SafetyChecker {
	return &SafetyChecker{
		MaxQueryRange:  7 * 24 * time.Hour, // 7 days
		MaxCardinality: 10000,
		TimeoutSeconds: 30,
		MaxQueryLength: 500, // Maximum 500 characters
		ForbiddenMetrics: []string{
			".*_secret.*",
			".*_password.*",
			".*_token.*",
			".*_key.*",
		},
		ForbiddenPatterns: []string{
			// Add any additional forbidden patterns here
		},
	}
}

// ValidateQuery checks if a PromQL query is safe to execute
func (sc *SafetyChecker) ValidateQuery(promql string) error {
	// Check query length limit
	if sc.MaxQueryLength > 0 && len(promql) > sc.MaxQueryLength {
		return errors.New(errors.ErrCodeInvalidInput, "Query exceeds maximum length").
			WithDetails(fmt.Sprintf("Query length: %d characters, maximum allowed: %d", len(promql), sc.MaxQueryLength)).
			WithSuggestion("Please simplify your query or break it into smaller queries.")
	}

	// Sanitize query for log injection prevention
	sanitizedQuery := sanitizeForLogging(promql)
	_ = sanitizedQuery // Used for logging purposes

	// Check for forbidden metrics (case-insensitive)
	promqlLower := strings.ToLower(promql)
	for _, forbidden := range sc.ForbiddenMetrics {
		forbiddenLower := strings.ToLower(forbidden)
		if matched, _ := regexp.MatchString(forbiddenLower, promqlLower); matched {
			return errors.NewForbiddenMetricError(forbidden)
		}
	}

	// Check for additional forbidden patterns (case-insensitive)
	for _, pattern := range sc.ForbiddenPatterns {
		patternLower := strings.ToLower(pattern)
		if matched, _ := regexp.MatchString(patternLower, promqlLower); matched {
			return errors.New(errors.ErrCodeForbiddenMetric, "Query contains forbidden pattern").
				WithDetails(fmt.Sprintf("Forbidden pattern: %s", pattern)).
				WithSuggestion("Modify your query to avoid using this pattern.")
		}
	}

	// Check for excessively long time ranges
	if strings.Contains(promql, "[") {
		// This is a simplified check - in production, you'd parse the range properly
		dangerousRanges := []string{"365d", "1y", "52w", "8760h"}
		for _, dangerous := range dangerousRanges {
			if strings.Contains(promql, dangerous) {
				return errors.NewExcessiveTimeRangeError(dangerous, sc.MaxQueryRange.String())
			}
		}
	}

	// Check for high cardinality operations
	if strings.Contains(promql, "by ()") || strings.Contains(promql, "without ()") {
		return errors.NewHighCardinalityError()
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
			return errors.NewExpensiveOperationError(op)
		}
	}

	// Check for nested subqueries (can be very expensive)
	if strings.Count(promql, "(") > 3 {
		return errors.New(errors.ErrCodeTooManyNested, "Query contains too many nested operations").
			WithDetails(fmt.Sprintf("The query has %d levels of nesting, maximum allowed is 3", strings.Count(promql, "("))).
			WithSuggestion("Break down complex queries into simpler parts, or reduce the number of nested function calls.")
	}

	return nil
}

// ValidateTimeRange checks if a time range is within safe limits
func (sc *SafetyChecker) ValidateTimeRange(timeRange string) error {
	// Validate time range format first
	if !isValidTimeRangeFormat(timeRange) {
		return errors.New(errors.ErrCodeInvalidInput, "Invalid time range format").
			WithDetails(fmt.Sprintf("Time range: %s", timeRange)).
			WithSuggestion("Use valid time range formats like: 5m, 1h, 24h, 7d, 1w")
	}

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
				return errors.NewExcessiveTimeRangeError(timeRange, sc.MaxQueryRange.String())
			}
		}
	}

	return nil
}

// isValidTimeRangeFormat validates the format of a time range string
func isValidTimeRangeFormat(timeRange string) bool {
	// Valid formats: 5m, 1h, 24h, 7d, 1w, etc.
	validFormat := regexp.MustCompile(`^\d+[mhdw]$`)
	return validFormat.MatchString(timeRange)
}

// sanitizeForLogging removes or escapes characters that could be used for log injection
func sanitizeForLogging(input string) string {
	// Replace newlines and carriage returns to prevent log injection
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")

	// Limit length for logging
	if len(sanitized) > 200 {
		sanitized = sanitized[:200] + "..."
	}

	return sanitized
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
