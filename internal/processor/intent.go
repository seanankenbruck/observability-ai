package processor

import (
	"fmt"
	"regexp"
)

// QueryIntent represents the classified intent of a query
type QueryIntent struct {
	Type        string            `json:"type"`        // "metrics", "errors", "performance", "comparison"
	Action      string            `json:"action"`      // "show", "compare", "analyze", "alert"
	Service     string            `json:"service"`     // extracted service name
	Metric      string            `json:"metric"`      // extracted metric type
	TimeRange   string            `json:"time_range"`  // parsed time range
	Aggregation string            `json:"aggregation"` // "rate", "sum", "avg", etc.
	Filters     map[string]string `json:"filters"`     // additional filters
}

// IntentClassifier classifies natural language queries
type IntentClassifier struct {
	patterns map[string]*regexp.Regexp
}

// NewIntentClassifier creates a new intent classifier
func NewIntentClassifier() *IntentClassifier {
	patterns := map[string]*regexp.Regexp{
		"error_rate":   regexp.MustCompile(`(?i)\b(error|fail|5xx|4xx)\b.*\b(rate|percent)\b`),
		"latency":      regexp.MustCompile(`(?i)\b(latency|response time|slow|duration)\b`),
		"throughput":   regexp.MustCompile(`(?i)\b(requests|throughput|qps|rps)\b`),
		"availability": regexp.MustCompile(`(?i)\b(uptime|availability|down)\b`),
		"comparison":   regexp.MustCompile(`(?i)\b(compare|vs|versus|against)\b`),
		"service_name": regexp.MustCompile(`(?i)\b(service|app|application)\s+(\w+[-\w]*)`),
		"time_range":   regexp.MustCompile(`(?i)\b(last|past|in the)\s+(\d+)\s*(minute|hour|day|week)s?\b`),
	}
	return &IntentClassifier{patterns: patterns}
}

// ClassifyIntent analyzes the natural language query and extracts intent
func (ic *IntentClassifier) ClassifyIntent(query string) (*QueryIntent, error) {
	intent := &QueryIntent{
		Filters: make(map[string]string),
	}

	// Extract service name
	if match := ic.patterns["service_name"].FindStringSubmatch(query); len(match) > 2 {
		intent.Service = match[2]
	}

	// Extract time range
	if match := ic.patterns["time_range"].FindStringSubmatch(query); len(match) > 3 {
		intent.TimeRange = fmt.Sprintf("%s%s", match[2], match[3])
	}

	// Classify query type
	switch {
	case ic.patterns["error_rate"].MatchString(query):
		intent.Type = "errors"
		intent.Action = "show"
		intent.Metric = "error_rate"
		intent.Aggregation = "rate"
	case ic.patterns["latency"].MatchString(query):
		intent.Type = "performance"
		intent.Action = "show"
		intent.Metric = "latency"
		intent.Aggregation = "avg"
	case ic.patterns["throughput"].MatchString(query):
		intent.Type = "performance"
		intent.Action = "show"
		intent.Metric = "throughput"
		intent.Aggregation = "rate"
	case ic.patterns["comparison"].MatchString(query):
		intent.Type = "comparison"
		intent.Action = "compare"
	default:
		intent.Type = "metrics"
		intent.Action = "show"
	}

	return intent, nil
}
