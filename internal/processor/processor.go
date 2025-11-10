package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/errors"
	"github.com/seanankenbruck/observability-ai/internal/llm"
	"github.com/seanankenbruck/observability-ai/internal/observability"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

// QueryRequest represents an incoming natural language query
type QueryRequest struct {
	Query     string            `json:"query" binding:"required"`
	TimeRange string            `json:"time_range,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
}

// QueryResponse represents the processed query result
type QueryResponse struct {
	PromQL         string                 `json:"promql"`
	Explanation    string                 `json:"explanation"`
	Confidence     float64                `json:"confidence"`
	Suggestions    []string               `json:"suggestions,omitempty"`
	EstimatedCost  int                    `json:"estimated_cost"`
	CacheHit       bool                   `json:"cache_hit"`
	ProcessingTime time.Duration          `json:"processing_time"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// QueryProcessor is the main service struct
type QueryProcessor struct {
	llmClient        llm.Client
	semanticMapper   semantic.Mapper
	safetyChecker    *SafetyChecker
	cache            *redis.Client
	intentClassifier *IntentClassifier
	logger           *observability.Logger
	healthChecker    *observability.HealthChecker
}

// NewQueryProcessor creates a new query processor instance
func NewQueryProcessor(llmClient llm.Client, semanticMapper semantic.Mapper, cache *redis.Client) *QueryProcessor {
	return &QueryProcessor{
		llmClient:        llmClient,
		semanticMapper:   semanticMapper,
		cache:            cache,
		safetyChecker:    NewSafetyChecker(),
		intentClassifier: NewIntentClassifier(),
		logger:           observability.NewLogger("query-processor"),
	}
}

// SetHealthChecker sets the health checker for the processor
func (qp *QueryProcessor) SetHealthChecker(healthChecker *observability.HealthChecker) {
	qp.healthChecker = healthChecker
}

// ProcessQuery handles the main query processing logic
func (qp *QueryProcessor) ProcessQuery(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	start := time.Now()

	// Log query start
	qp.logger.Info(ctx, "Processing query", map[string]interface{}{
		"query":      req.Query,
		"time_range": req.TimeRange,
	})

	var errorType string
	var response *QueryResponse
	var processingErr error

	defer func() {
		// Record metrics at the end
		duration := time.Since(start)
		success := processingErr == nil
		cached := response != nil && response.CacheHit
		observability.RecordQueryMetrics(duration, success, cached, errorType)

		if processingErr != nil {
			qp.logger.Error(ctx, "Query processing failed", processingErr, map[string]interface{}{
				"query":       req.Query,
				"duration_ms": duration.Milliseconds(),
				"error_type":  errorType,
			})
		} else {
			qp.logger.Info(ctx, "Query processed successfully", map[string]interface{}{
				"query":       req.Query,
				"duration_ms": duration.Milliseconds(),
				"cache_hit":   cached,
				"confidence":  response.Confidence,
			})
		}
	}()

	// Check cache first
	if cachedResult, err := qp.getCachedResult(ctx, req.Query); err == nil {
		qp.logger.Debug(ctx, "Cache hit for query", map[string]interface{}{
			"query": req.Query,
		})
		cachedResult.CacheHit = true
		cachedResult.ProcessingTime = time.Since(start)
		response = cachedResult
		return cachedResult, nil
	}

	// Classify intent
	intent, err := qp.intentClassifier.ClassifyIntent(req.Query)
	if err != nil {
		errorType = "intent_classification"
		processingErr = errors.NewIntentClassificationError(err, req.Query)
		return nil, processingErr
	}

	// Generate embeddings for semantic search
	embedding, err := qp.llmClient.GetEmbedding(ctx, req.Query)
	if err != nil {
		errorType = "embedding_generation"
		processingErr = errors.NewEmbeddingGenerationError(err)
		return nil, processingErr
	}

	// Find similar queries
	similarQueries, err := qp.semanticMapper.FindSimilarQueries(ctx, embedding)
	if err != nil {
		// Log warning but don't fail - similar queries are optional
		qp.logger.Warn(ctx, "Failed to find similar queries", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Build enhanced prompt
	prompt, err := qp.buildPrompt(ctx, req, intent, similarQueries)
	if err != nil {
		errorType = "prompt_building"
		processingErr = errors.Wrap(err, errors.ErrCodePromptBuilding, "Failed to build prompt for query generation").
			WithDetails("An error occurred while constructing the prompt for the AI model").
			WithSuggestion("This is an internal error. Please try your query again.").
			WithMetadata("retryable", true)
		return nil, processingErr
	}

	// Log the prompt for debugging
	qp.logger.Debug(ctx, "Generated prompt for LLM", map[string]interface{}{
		"prompt": prompt,
	})

	// Generate PromQL using LLM
	llmResponse, err := qp.llmClient.GenerateQuery(ctx, prompt)
	if err != nil {
		errorType = "query_generation"
		processingErr = errors.NewQueryGenerationError(err)
		return nil, processingErr
	}

	// Check if LLM returned an error message (no suitable metrics found)
	if strings.HasPrefix(strings.TrimSpace(llmResponse.PromQL), "ERROR:") {
		errorType = "no_suitable_metrics"
		processingErr = errors.Wrap(nil, errors.ErrCodeQueryGeneration, strings.TrimPrefix(strings.TrimSpace(llmResponse.PromQL), "ERROR:")).
			WithDetails("The requested query cannot be fulfilled with the currently discovered metrics").
			WithSuggestion("Check available services and metrics, or wait for service discovery to complete").
			WithMetadata("retryable", true).
			WithMetadata("llm_message", llmResponse.PromQL)
		return nil, processingErr
	}

	// Validate query safety
	if err := qp.safetyChecker.ValidateQuery(llmResponse.PromQL); err != nil {
		errorType = "safety_validation"
		processingErr = err // Already an enhanced error from SafetyChecker
		observability.GetGlobalMetrics().Inc(observability.MetricQuerySafetyViolation, map[string]string{
			"error_type": errorType,
		})
		return nil, processingErr
	}

	// Build response
	response = &QueryResponse{
		PromQL:         llmResponse.PromQL,
		Explanation:    llmResponse.Explanation,
		Confidence:     llmResponse.Confidence,
		EstimatedCost:  qp.estimateQueryCost(llmResponse.PromQL),
		CacheHit:       false,
		ProcessingTime: time.Since(start),
		Metadata: map[string]interface{}{
			"intent":          intent,
			"similar_queries": len(similarQueries),
		},
	}

	// Cache the result
	if err := qp.cacheResult(ctx, req.Query, response); err != nil {
		qp.logger.Warn(ctx, "Failed to cache query result", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return response, nil
}

// buildPrompt creates an enhanced prompt for the LLM
func (qp *QueryProcessor) buildPrompt(ctx context.Context, req *QueryRequest, intent *QueryIntent, similarQueries []semantic.SimilarQuery) (string, error) {
	var promptBuilder strings.Builder

	promptBuilder.WriteString("You are a PromQL expert assistant. Your task is to convert natural language queries into accurate PromQL queries.\n\n")

	promptBuilder.WriteString("=== CRITICAL RULES ===\n")
	promptBuilder.WriteString("1. ONLY use metrics from the Available Metrics Catalog below - no exceptions\n")
	promptBuilder.WriteString("2. If the requested metric type doesn't exist, respond with: ERROR: No suitable metrics found. [explanation]\n")
	promptBuilder.WriteString("3. Return ONLY the PromQL query or ERROR message - no markdown, explanations, or code blocks\n")
	promptBuilder.WriteString("4. Apply correct PromQL functions based on metric types:\n")
	promptBuilder.WriteString("   - Counters (e.g., *_total, *_count): Use rate() or increase()\n")
	promptBuilder.WriteString("   - Gauges (e.g., *_active_*, *_current_*, *_size_): Use directly or with aggregations\n")
	promptBuilder.WriteString("   - Histograms (*_bucket): Use histogram_quantile() for percentiles\n")
	promptBuilder.WriteString("   - Summaries (*_sum, *_count): Calculate averages using sum/count\n\n")

	// Add ALL discovered services and their metrics
	services, err := qp.semanticMapper.GetServices(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get services for prompt: %w", err)
	}

	// Log the number of services discovered
	fmt.Printf("DEBUG: Building prompt with %d discovered services\n", len(services))

	if len(services) > 0 {
		promptBuilder.WriteString("=== AVAILABLE METRICS CATALOG ===\n")
		promptBuilder.WriteString("These are the ONLY metrics you can use:\n\n")

		// Track if we need to filter metrics for large services
		const maxMetricsPerService = 50 // Limit to avoid token limits

		for _, service := range services {
			promptBuilder.WriteString(fmt.Sprintf("Service: %s (namespace: %s)\n", service.Name, service.Namespace))
			if len(service.MetricNames) > 0 {
				// Categorize metrics by type for better context
				counters, gauges, histograms, others := categorizeMetrics(service.MetricNames)

				// Filter to relevant metrics if service is targeted or limit if too many
				var filteredCounters, filteredGauges, filteredHistograms, filteredOthers []string

				// If a specific service is requested, prioritize showing all its metrics
				if intent.Service != "" && strings.EqualFold(service.Name, intent.Service) {
					filteredCounters = counters
					filteredGauges = gauges
					filteredHistograms = histograms
					filteredOthers = others
				} else if len(service.MetricNames) > maxMetricsPerService {
					// For large services, show a sample with metric count
					filteredCounters = limitSlice(counters, 10)
					filteredGauges = limitSlice(gauges, 10)
					filteredHistograms = limitSlice(histograms, 5)
					filteredOthers = limitSlice(others, 5)
				} else {
					filteredCounters = counters
					filteredGauges = gauges
					filteredHistograms = histograms
					filteredOthers = others
				}

				totalMetrics := len(service.MetricNames)
				shownMetrics := len(filteredCounters) + len(filteredGauges) + len(filteredHistograms) + len(filteredOthers)

				if len(filteredCounters) > 0 {
					promptBuilder.WriteString("  Counters (use rate/increase):\n")
					for _, metric := range filteredCounters {
						promptBuilder.WriteString(fmt.Sprintf("    - %s\n", metric))
					}
				}
				if len(filteredGauges) > 0 {
					promptBuilder.WriteString("  Gauges (use directly or aggregate):\n")
					for _, metric := range filteredGauges {
						promptBuilder.WriteString(fmt.Sprintf("    - %s\n", metric))
					}
				}
				if len(filteredHistograms) > 0 {
					promptBuilder.WriteString("  Histograms (use histogram_quantile):\n")
					for _, metric := range filteredHistograms {
						promptBuilder.WriteString(fmt.Sprintf("    - %s\n", metric))
					}
				}
				if len(filteredOthers) > 0 {
					promptBuilder.WriteString("  Other metrics:\n")
					for _, metric := range filteredOthers {
						promptBuilder.WriteString(fmt.Sprintf("    - %s\n", metric))
					}
				}

				// Note if metrics were filtered
				if shownMetrics < totalMetrics {
					promptBuilder.WriteString(fmt.Sprintf("  ... and %d more metrics (search for specific patterns)\n", totalMetrics-shownMetrics))
				}
			} else {
				promptBuilder.WriteString("  (No metrics discovered yet)\n")
			}
			promptBuilder.WriteString("\n")
		}
		promptBuilder.WriteString("=== END CATALOG ===\n\n")
	} else {
		promptBuilder.WriteString("WARNING: No services have been discovered yet. Return ERROR.\n\n")
	}

	// Add similar queries as examples
	if len(similarQueries) > 0 {
		promptBuilder.WriteString("=== EXAMPLES FROM PAST QUERIES ===\n")
		for _, sq := range similarQueries[:min(3, len(similarQueries))] {
			promptBuilder.WriteString(fmt.Sprintf("Q: %s\nA: %s\n\n", sq.Query, sq.PromQL))
		}
	}

	// Add the main query with context
	promptBuilder.WriteString("=== YOUR TASK ===\n")
	promptBuilder.WriteString(fmt.Sprintf("User Query: \"%s\"\n", req.Query))

	// Add extracted intent for context
	if intent.Type != "" || intent.Service != "" || intent.TimeRange != "" {
		promptBuilder.WriteString("\nDetected Context:\n")
		if intent.Type != "" {
			promptBuilder.WriteString(fmt.Sprintf("  - Intent: %s\n", intent.Type))
		}
		if intent.Service != "" {
			promptBuilder.WriteString(fmt.Sprintf("  - Target Service: %s\n", intent.Service))
		}
		if intent.TimeRange != "" {
			promptBuilder.WriteString(fmt.Sprintf("  - Time Range: %s\n", intent.TimeRange))
		}
	}

	promptBuilder.WriteString("\nYour Response (PromQL query or ERROR):")

	return promptBuilder.String(), nil
}

// categorizeMetrics categorizes metrics by type based on naming conventions
func categorizeMetrics(metrics []string) (counters, gauges, histograms, others []string) {
	for _, metric := range metrics {
		metricLower := strings.ToLower(metric)
		switch {
		case strings.HasSuffix(metricLower, "_total") || strings.HasSuffix(metricLower, "_count"):
			counters = append(counters, metric)
		case strings.HasSuffix(metricLower, "_bucket"):
			histograms = append(histograms, metric)
		case strings.Contains(metricLower, "_active_") ||
		     strings.Contains(metricLower, "_current_") ||
		     strings.Contains(metricLower, "_size") ||
		     strings.Contains(metricLower, "_gauge") ||
		     strings.HasSuffix(metricLower, "_bytes") ||
		     strings.HasSuffix(metricLower, "_ratio"):
			gauges = append(gauges, metric)
		default:
			others = append(others, metric)
		}
	}
	return
}

// limitSlice returns the first n elements of a slice, or the whole slice if shorter
func limitSlice(slice []string, n int) []string {
	if len(slice) <= n {
		return slice
	}
	return slice[:n]
}

// estimateQueryCost provides a rough estimate of query execution cost
func (qp *QueryProcessor) estimateQueryCost(promql string) int {
	cost := 1

	// Add cost for aggregations
	if strings.Contains(promql, "sum") || strings.Contains(promql, "avg") {
		cost += 2
	}

	// Add cost for rate calculations
	if strings.Contains(promql, "rate") || strings.Contains(promql, "increase") {
		cost += 3
	}

	// Add cost for regex matching
	if strings.Contains(promql, "=~") {
		cost += 5
	}

	return cost
}

// getCachedResult retrieves cached query results
func (qp *QueryProcessor) getCachedResult(ctx context.Context, query string) (*QueryResponse, error) {
	key := fmt.Sprintf("query:%s", query)
	cached, err := qp.cache.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var response QueryResponse
	if err := json.Unmarshal([]byte(cached), &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// cacheResult stores query results in cache
func (qp *QueryProcessor) cacheResult(ctx context.Context, query string, response *QueryResponse) error {
	key := fmt.Sprintf("query:%s", query)

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return qp.cache.Set(ctx, key, data, 5*time.Minute).Err()
}

// AuthMiddleware is an interface for authentication middleware
type AuthMiddleware interface {
	Middleware() gin.HandlerFunc
}

// SetupRoutes configures HTTP routes with optional authentication
func (qp *QueryProcessor) SetupRoutes(authMiddleware AuthMiddleware) *gin.Engine {
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Public health check endpoint
	r.GET("/health", func(c *gin.Context) {
		if qp.healthChecker != nil {
			response := qp.healthChecker.GetHealthResponse(c.Request.Context())
			statusCode := http.StatusOK
			if response.Status == observability.HealthStatusUnhealthy {
				statusCode = http.StatusServiceUnavailable
			}
			c.JSON(statusCode, response)
		} else {
			// Fallback for when health checker is not configured
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
				"service": "query-processor",
			})
		}
	})

	// Public API v1 health endpoint
	publicAPI := r.Group("/api/v1")
	{
		publicAPI.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
				"service": "query-processor",
			})
		})
	}

	// Protected API routes (require authentication)
	api := r.Group("/api/v1")
	if authMiddleware != nil {
		api.Use(authMiddleware.Middleware())
	}
	{
		// Main query endpoint
		api.POST("/query", func(c *gin.Context) {
			var req QueryRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				enhancedErr := errors.NewInvalidInputError("request body", err.Error())
				c.JSON(http.StatusBadRequest, formatErrorResponse(enhancedErr))
				return
			}

			response, err := qp.ProcessQuery(c.Request.Context(), &req)
			if err != nil {
				c.JSON(getErrorStatusCode(err), formatErrorResponse(err))
				return
			}

			c.JSON(http.StatusOK, response)
		})

		// Services endpoints
		api.GET("/services", qp.handleGetServices)
		api.GET("/services/:id", qp.handleGetService)
		api.GET("/services/search", qp.handleSearchServices)
		api.GET("/services/:id/metrics", qp.handleGetServiceMetrics)

		// Metrics endpoints
		api.GET("/metrics", qp.handleGetAllMetrics)

		// Query history endpoint
		api.GET("/history", qp.handleGetHistory)

		// Query suggestions
		api.GET("/suggestions", qp.handleGetSuggestions)
	}

	// Serve static files for the web interface
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/", "./web/dist/index.html")

	return r
}

// Service-related handlers
func (qp *QueryProcessor) handleGetServices(c *gin.Context) {
	services, err := qp.semanticMapper.GetServices(c.Request.Context())
	if err != nil {
		enhancedErr := errors.NewDatabaseQueryError(err, "fetching services")
		c.JSON(http.StatusInternalServerError, formatErrorResponse(enhancedErr))
		return
	}
	c.JSON(http.StatusOK, services)
}

func (qp *QueryProcessor) handleGetService(c *gin.Context) {
	serviceID := c.Param("id")
	// For now, we'll search by name since that's what we have
	service, err := qp.semanticMapper.GetServiceByName(c.Request.Context(), serviceID, "default")
	if err != nil {
		enhancedErr := errors.NewServiceNotFoundError(serviceID)
		c.JSON(http.StatusNotFound, formatErrorResponse(enhancedErr))
		return
	}
	c.JSON(http.StatusOK, service)
}

func (qp *QueryProcessor) handleSearchServices(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		qp.handleGetServices(c)
		return
	}

	services, err := qp.semanticMapper.SearchServices(c.Request.Context(), query)
	if err != nil {
		enhancedErr := errors.NewDatabaseQueryError(err, "searching services")
		c.JSON(http.StatusInternalServerError, formatErrorResponse(enhancedErr))
		return
	}
	c.JSON(http.StatusOK, services)
}

func (qp *QueryProcessor) handleGetServiceMetrics(c *gin.Context) {
	serviceID := c.Param("id")
	metrics, err := qp.semanticMapper.GetMetrics(c.Request.Context(), serviceID)
	if err != nil {
		enhancedErr := errors.NewDatabaseQueryError(err, "fetching metrics for service")
		c.JSON(http.StatusInternalServerError, formatErrorResponse(enhancedErr))
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (qp *QueryProcessor) handleGetAllMetrics(c *gin.Context) {
	// Get all services first, then get metrics for each
	services, err := qp.semanticMapper.GetServices(c.Request.Context())
	if err != nil {
		enhancedErr := errors.NewDatabaseQueryError(err, "fetching all metrics")
		c.JSON(http.StatusInternalServerError, formatErrorResponse(enhancedErr))
		return
	}

	// Initialize as empty array instead of nil to ensure JSON returns [] instead of null
	allMetrics := make([]interface{}, 0)
	for _, service := range services {
		metrics, err := qp.semanticMapper.GetMetrics(c.Request.Context(), service.ID)
		if err != nil {
			continue // Skip services with metric errors
		}
		for _, metric := range metrics {
			allMetrics = append(allMetrics, metric)
		}
	}

	c.JSON(http.StatusOK, allMetrics)
}

func (qp *QueryProcessor) handleGetSuggestions(c *gin.Context) {
	query := c.Query("q")

	// For now, return some basic suggestions
	// In the future, this could use the semantic mapper to find similar queries
	suggestions := []string{
		"Show error rate for " + query,
		"What's the latency for " + query,
		"Requests per second for " + query,
		"Memory usage for " + query,
	}

	c.JSON(http.StatusOK, suggestions)
}

func (qp *QueryProcessor) handleGetHistory(c *gin.Context) {
	// For now, we'll use an empty embedding to get all queries
	// In a real implementation, you might want to add a GetRecentQueries method
	// or filter by user ID from the auth context
	emptyEmbedding := make([]float32, 1536) // Claude embedding size

	queries, err := qp.semanticMapper.FindSimilarQueries(c.Request.Context(), emptyEmbedding)
	if err != nil {
		enhancedErr := errors.NewDatabaseQueryError(err, "fetching query history")
		c.JSON(http.StatusInternalServerError, formatErrorResponse(enhancedErr))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queries": queries,
		"count":   len(queries),
	})
}

// Utility function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// formatErrorResponse formats an error into a user-friendly response
func formatErrorResponse(err error) gin.H {
	// Check if it's an EnhancedError
	if enhancedErr, ok := err.(*errors.EnhancedError); ok {
		response := gin.H{
			"error": gin.H{
				"code":    enhancedErr.Code,
				"message": enhancedErr.Message,
			},
		}

		if enhancedErr.Details != "" {
			response["error"].(gin.H)["details"] = enhancedErr.Details
		}

		if enhancedErr.Suggestion != "" {
			response["error"].(gin.H)["suggestion"] = enhancedErr.Suggestion
		}

		if enhancedErr.Documentation != "" {
			response["error"].(gin.H)["documentation"] = enhancedErr.Documentation
		}

		if len(enhancedErr.Metadata) > 0 {
			response["error"].(gin.H)["metadata"] = enhancedErr.Metadata
		}

		return response
	}

	// Fallback for regular errors
	return gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		},
	}
}

// getErrorStatusCode returns the appropriate HTTP status code for an error
func getErrorStatusCode(err error) int {
	if enhancedErr, ok := err.(*errors.EnhancedError); ok {
		switch enhancedErr.Code {
		case errors.ErrCodeInvalidInput, errors.ErrCodeMissingRequired, errors.ErrCodeInvalidDuration:
			return http.StatusBadRequest
		case errors.ErrCodeInvalidCredentials, errors.ErrCodeNotAuthenticated:
			return http.StatusUnauthorized
		case errors.ErrCodeInsufficientPerms:
			return http.StatusForbidden
		case errors.ErrCodeServiceNotFound:
			return http.StatusNotFound
		case errors.ErrCodeSafetyValidation, errors.ErrCodeForbiddenMetric,
			errors.ErrCodeExcessiveTimeRange, errors.ErrCodeHighCardinality,
			errors.ErrCodeExpensiveOperation, errors.ErrCodeTooManyNested:
			return http.StatusBadRequest
		default:
			return http.StatusInternalServerError
		}
	}
	return http.StatusInternalServerError
}
