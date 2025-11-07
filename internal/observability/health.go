package observability

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a health check for a component
type HealthCheck struct {
	Name        string                 `json:"name"`
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	LastChecked time.Time              `json:"last_checked"`
	Duration    time.Duration          `json:"duration_ms"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HealthChecker performs health checks on dependencies
type HealthChecker struct {
	checks map[string]HealthCheckFunc
	cache  map[string]*HealthCheck
	mu     sync.RWMutex
	ttl    time.Duration
}

// HealthCheckFunc is a function that performs a health check
type HealthCheckFunc func(context.Context) *HealthCheck

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheckFunc),
		cache:  make(map[string]*HealthCheck),
		ttl:    5 * time.Second, // Cache health checks for 5 seconds
	}
}

// Register registers a health check
func (hc *HealthChecker) Register(name string, check HealthCheckFunc) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// Check performs all health checks
func (hc *HealthChecker) Check(ctx context.Context) map[string]*HealthCheck {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	results := make(map[string]*HealthCheck)
	now := time.Now()

	for name, checkFunc := range hc.checks {
		// Check if cached result is still valid
		if cached, exists := hc.cache[name]; exists {
			if now.Sub(cached.LastChecked) < hc.ttl {
				results[name] = cached
				continue
			}
		}

		// Perform the check
		result := checkFunc(ctx)
		result.LastChecked = time.Now()

		// Cache the result
		hc.cache[name] = result
		results[name] = result
	}

	return results
}

// GetOverallStatus determines the overall health status
func (hc *HealthChecker) GetOverallStatus(ctx context.Context) HealthStatus {
	checks := hc.Check(ctx)

	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return HealthStatusUnhealthy
	}
	if hasDegraded {
		return HealthStatusDegraded
	}
	return HealthStatusHealthy
}

// HealthResponse represents the complete health check response
type HealthResponse struct {
	Status    HealthStatus              `json:"status"`
	Timestamp time.Time                 `json:"timestamp"`
	Checks    map[string]*HealthCheck   `json:"checks"`
	Metadata  map[string]interface{}    `json:"metadata,omitempty"`
}

// GetHealthResponse returns a complete health response
func (hc *HealthChecker) GetHealthResponse(ctx context.Context) *HealthResponse {
	checks := hc.Check(ctx)

	return &HealthResponse{
		Status:    hc.GetOverallStatus(ctx),
		Timestamp: time.Now(),
		Checks:    checks,
		Metadata: map[string]interface{}{
			"version": "1.0.0",
			"service": "query-processor",
		},
	}
}

// Common health check functions

// DatabaseHealthCheck creates a health check for database connectivity
func DatabaseHealthCheck(pingFunc func(context.Context) error) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		start := time.Now()

		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		err := pingFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return &HealthCheck{
				Name:     "database",
				Status:   HealthStatusUnhealthy,
				Message:  fmt.Sprintf("Database connection failed: %v", err),
				Duration: duration,
			}
		}

		return &HealthCheck{
			Name:     "database",
			Status:   HealthStatusHealthy,
			Message:  "Database connection successful",
			Duration: duration,
			Metadata: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}
}

// RedisHealthCheck creates a health check for Redis connectivity
func RedisHealthCheck(pingFunc func(context.Context) error) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		start := time.Now()

		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		err := pingFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return &HealthCheck{
				Name:     "redis",
				Status:   HealthStatusUnhealthy,
				Message:  fmt.Sprintf("Redis connection failed: %v", err),
				Duration: duration,
			}
		}

		return &HealthCheck{
			Name:     "redis",
			Status:   HealthStatusHealthy,
			Message:  "Redis connection successful",
			Duration: duration,
			Metadata: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}
}

// LLMHealthCheck creates a health check for LLM service
func LLMHealthCheck(checkFunc func(context.Context) error) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		start := time.Now()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := checkFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			// LLM being down is degraded, not unhealthy (queries can still be cached)
			return &HealthCheck{
				Name:     "llm_service",
				Status:   HealthStatusDegraded,
				Message:  fmt.Sprintf("LLM service unavailable: %v", err),
				Duration: duration,
			}
		}

		return &HealthCheck{
			Name:     "llm_service",
			Status:   HealthStatusHealthy,
			Message:  "LLM service available",
			Duration: duration,
			Metadata: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}
}

// MimirHealthCheck creates a health check for Mimir/Prometheus
func MimirHealthCheck(queryFunc func(context.Context) error) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		start := time.Now()

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := queryFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return &HealthCheck{
				Name:     "mimir",
				Status:   HealthStatusUnhealthy,
				Message:  fmt.Sprintf("Mimir connection failed: %v", err),
				Duration: duration,
			}
		}

		return &HealthCheck{
			Name:     "mimir",
			Status:   HealthStatusHealthy,
			Message:  "Mimir connection successful",
			Duration: duration,
			Metadata: map[string]interface{}{
				"response_time_ms": duration.Milliseconds(),
			},
		}
	}
}

// MemoryHealthCheck creates a health check for memory usage
func MemoryHealthCheck(getMemoryUsage func() (used, total uint64)) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		used, total := getMemoryUsage()
		usagePercent := float64(used) / float64(total) * 100

		status := HealthStatusHealthy
		message := "Memory usage normal"

		if usagePercent > 90 {
			status = HealthStatusUnhealthy
			message = "Memory usage critical"
		} else if usagePercent > 75 {
			status = HealthStatusDegraded
			message = "Memory usage high"
		}

		return &HealthCheck{
			Name:    "memory",
			Status:  status,
			Message: message,
			Metadata: map[string]interface{}{
				"used_bytes":      used,
				"total_bytes":     total,
				"usage_percent":   usagePercent,
			},
		}
	}
}

// DiskHealthCheck creates a health check for disk usage
func DiskHealthCheck(getDiskUsage func() (used, total uint64)) HealthCheckFunc {
	return func(ctx context.Context) *HealthCheck {
		used, total := getDiskUsage()
		usagePercent := float64(used) / float64(total) * 100

		status := HealthStatusHealthy
		message := "Disk usage normal"

		if usagePercent > 90 {
			status = HealthStatusUnhealthy
			message = "Disk usage critical"
		} else if usagePercent > 80 {
			status = HealthStatusDegraded
			message = "Disk usage high"
		}

		return &HealthCheck{
			Name:    "disk",
			Status:  status,
			Message: message,
			Metadata: map[string]interface{}{
				"used_bytes":    used,
				"total_bytes":   total,
				"usage_percent": usagePercent,
			},
		}
	}
}
