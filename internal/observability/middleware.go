package observability

import (
	"bytes"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is the header name for request/correlation IDs
const RequestIDHeader = "X-Request-ID"

// responseWriter is a wrapper around gin.ResponseWriter that captures response size
type responseWriter struct {
	gin.ResponseWriter
	size int
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	if w.body != nil {
		w.body.Write(b)
	}
	return size, err
}

func (w *responseWriter) WriteString(s string) (int, error) {
	size, err := w.ResponseWriter.WriteString(s)
	w.size += size
	if w.body != nil {
		w.body.WriteString(s)
	}
	return size, err
}

// RequestLoggingMiddleware logs all HTTP requests with correlation IDs
func RequestLoggingMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Get or generate correlation ID
		correlationID := c.GetHeader(RequestIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		c.Set("correlation_id", correlationID)
		c.Header(RequestIDHeader, correlationID)

		// Add correlation ID to context
		ctx := WithCorrelationID(c.Request.Context(), correlationID)
		c.Request = c.Request.WithContext(ctx)

		// Get user ID from auth if available
		if userID, exists := c.Get("user_id"); exists {
			if uid, ok := userID.(string); ok {
				ctx = WithUserID(ctx, uid)
				c.Request = c.Request.WithContext(ctx)
			}
		}

		// Wrap response writer to capture size
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			size:           0,
		}
		c.Writer = rw

		// Log request start
		logger.Info(ctx, "HTTP request started", map[string]interface{}{
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"query":      c.Request.URL.RawQuery,
			"user_agent": c.Request.UserAgent(),
			"ip":         c.ClientIP(),
		})

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log request completion
		fields := map[string]interface{}{
			"method":        c.Request.Method,
			"path":          c.Request.URL.Path,
			"status":        c.Writer.Status(),
			"duration_ms":   duration.Milliseconds(),
			"response_size": rw.size,
			"ip":            c.ClientIP(),
		}

		// Log errors separately
		if len(c.Errors) > 0 {
			fields["errors"] = c.Errors.String()
			logger.Error(ctx, "HTTP request failed", c.Errors.Last().Err, fields)
		} else if c.Writer.Status() >= 400 {
			logger.Warn(ctx, "HTTP request completed with error status", fields)
		} else {
			logger.Info(ctx, "HTTP request completed", fields)
		}

		// Record metrics
		RecordHTTPMetrics(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			rw.size,
		)
	}
}

// MetricsMiddleware records metrics for HTTP requests
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Wrap response writer to capture size
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			size:           0,
		}
		c.Writer = rw

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		RecordHTTPMetrics(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			rw.size,
		)
	}
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				ctx := c.Request.Context()
				logger.Error(ctx, "Panic recovered", nil, map[string]interface{}{
					"panic":  err,
					"method": c.Request.Method,
					"path":   c.Request.URL.Path,
				})

				// Return error response
				c.JSON(500, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}

// HealthCheckMiddleware provides a simple health check endpoint
func HealthCheckMiddleware(checker *HealthChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/api/v1/health" {
			response := checker.GetHealthResponse(c.Request.Context())

			statusCode := 200
			if response.Status == HealthStatusDegraded {
				statusCode = 200 // Still return 200 for degraded
			} else if response.Status == HealthStatusUnhealthy {
				statusCode = 503 // Service unavailable for unhealthy
			}

			c.JSON(statusCode, response)
			c.Abort()
		} else {
			c.Next()
		}
	}
}

// MetricsEndpointMiddleware provides a metrics endpoint
func MetricsEndpointMiddleware(collector *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			metrics := collector.GetAll()
			c.JSON(200, gin.H{
				"metrics":   metrics,
				"timestamp": time.Now(),
			})
			c.Abort()
		} else {
			c.Next()
		}
	}
}

// CORSWithLogging adds CORS headers and logs cross-origin requests
func CORSWithLogging(logger *Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			if origin != "" {
				logger.Debug(c.Request.Context(), "CORS preflight request", map[string]interface{}{
					"origin": origin,
					"method": c.Request.Header.Get("Access-Control-Request-Method"),
				})
			}
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
