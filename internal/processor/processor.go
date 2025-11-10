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
	"github.com/seanankenbruck/observability-ai/internal/mimir"
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
	Query             string           `json:"query,omitempty"`
	PromQL            string           `json:"promql"`
	Explanation       string           `json:"explanation"`
	Confidence        float64          `json:"confidence,omitempty"`
	Suggestions       []string         `json:"suggestions,omitempty"`
	Results           *QueryResults    `json:"results,omitempty"`           // Enhanced query results
	ResultMetadata    *ResultMetadata  `json:"result_metadata,omitempty"`   // Visualization hints
	Cached            bool             `json:"cached"`
	EstimatedCost     int              `json:"estimated_cost,omitempty"`
	CacheHit          bool             `json:"cache_hit,omitempty"`
	ProcessingTime    time.Duration    `json:"processing_time,omitempty"`
	ExecutionTime     int64            `json:"execution_time_ms,omitempty"`
	Metadata          map[string]interface{}  `json:"metadata,omitempty"`
}

// QueryProcessor is the main service struct
type QueryProcessor struct {
	llmClient         llm.Client
	semanticMapper    semantic.Mapper
	safetyChecker     *SafetyChecker
	cache             *redis.Client
	intentClassifier  *IntentClassifier
	logger            *observability.Logger
	healthChecker     *observability.HealthChecker
	mimirClient       MimirClient
	resultProcessor   *ResultProcessor
	metadataGenerator *MetadataGenerator
}

// MimirClient defines the interface for executing PromQL queries
type MimirClient interface {
	Query(ctx context.Context, query string, timestamp time.Time) (*mimir.QueryResponse, error)
	QueryInstant(ctx context.Context, query string, timestamp time.Time) (*mimir.QueryResult, error)
	QueryRangeWithResult(ctx context.Context, query string, start, end time.Time, step time.Duration) (*mimir.QueryResult, error)
}

// ProcessorConfig holds configuration for the query processor
type ProcessorConfig struct {
	MaxResultSamples    int
	MaxResultTimePoints int
}

// NewQueryProcessor creates a new query processor instance
func NewQueryProcessor(llmClient llm.Client, semanticMapper semantic.Mapper, cache *redis.Client, mimirClient MimirClient, config ProcessorConfig) *QueryProcessor {
	// Create result processor with configured limits
	resultProcessor := &ResultProcessor{
		maxSamples:    config.MaxResultSamples,
		maxTimePoints: config.MaxResultTimePoints,
	}

	// Set defaults if not configured
	if resultProcessor.maxSamples == 0 {
		resultProcessor.maxSamples = MaxSamplesDefault
	}
	if resultProcessor.maxTimePoints == 0 {
		resultProcessor.maxTimePoints = MaxTimePoints
	}

	return &QueryProcessor{
		llmClient:         llmClient,
		semanticMapper:    semanticMapper,
		cache:             cache,
		mimirClient:       mimirClient,
		safetyChecker:     NewSafetyChecker(),
		intentClassifier:  NewIntentClassifier(),
		logger:            observability.NewLogger("query-processor"),
		resultProcessor:   resultProcessor,
		metadataGenerator: NewMetadataGenerator(),
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

	// Generate PromQL using LLM
	llmResponse, err := qp.llmClient.GenerateQuery(ctx, prompt)
	if err != nil {
		errorType = "query_generation"
		processingErr = errors.NewQueryGenerationError(err)
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

	// Execute the PromQL query against Mimir to get actual results
	var processedResults *QueryResults
	var resultMetadata *ResultMetadata
	if qp.mimirClient != nil {
		// Use QueryInstant for better result handling
		mimirResult, err := qp.mimirClient.QueryInstant(ctx, llmResponse.PromQL, time.Time{})
		if err != nil {
			qp.logger.Warn(ctx, "Failed to execute PromQL query against Mimir", map[string]interface{}{
				"error":  err.Error(),
				"promql": llmResponse.PromQL,
			})
			// Don't fail the entire request - just proceed without results
		} else {
			// Process results with the result processor
			processedResults, err = qp.resultProcessor.ProcessResults(mimirResult)
			if err != nil {
				qp.logger.Warn(ctx, "Failed to process query results", map[string]interface{}{
					"error":  err.Error(),
					"promql": llmResponse.PromQL,
				})
			} else {
				// Generate metadata for visualization hints
				resultMetadata = qp.metadataGenerator.GenerateMetadata(llmResponse.PromQL, processedResults)
			}
		}
	}

	// Build response
	response = &QueryResponse{
		Query:          req.Query,
		PromQL:         llmResponse.PromQL,
		Explanation:    llmResponse.Explanation,
		Confidence:     llmResponse.Confidence,
		Results:        processedResults,
		ResultMetadata: resultMetadata,
		Cached:         false,
		CacheHit:       false,
		EstimatedCost:  qp.estimateQueryCost(llmResponse.PromQL),
		ProcessingTime: time.Since(start),
		ExecutionTime:  time.Since(start).Milliseconds(),
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

	promptBuilder.WriteString("You are a PromQL expert. Convert the natural language query to PromQL.\n\n")
	promptBuilder.WriteString("IMPORTANT: Return ONLY the PromQL query. Do not include explanations, descriptions, or code blocks.\n\n")

	// Add context about available services
	if intent.Service != "" {
		if service, err := qp.semanticMapper.GetServiceByName(ctx, intent.Service, "default"); err == nil {
			promptBuilder.WriteString(fmt.Sprintf("Service Context:\n- Name: %s\n- Namespace: %s\n- Available metrics: %v\n\n",
				service.Name, service.Namespace, service.MetricNames))
		}
	}

	// Add similar queries as examples
	if len(similarQueries) > 0 {
		promptBuilder.WriteString("Examples:\n")
		for _, sq := range similarQueries[:min(3, len(similarQueries))] {
			promptBuilder.WriteString(fmt.Sprintf("Query: %s\nPromQL: %s\n\n", sq.Query, sq.PromQL))
		}
	}

	// Add the main query
	promptBuilder.WriteString(fmt.Sprintf("Query: %s\n\n", req.Query))

	// Add extracted intent for context
	if intent.Type != "" {
		promptBuilder.WriteString(fmt.Sprintf("Intent: %s\n", intent.Type))
	}
	if intent.Service != "" {
		promptBuilder.WriteString(fmt.Sprintf("Target Service: %s\n", intent.Service))
	}
	if intent.TimeRange != "" {
		promptBuilder.WriteString(fmt.Sprintf("Time Range: %s\n", intent.TimeRange))
	}

	promptBuilder.WriteString("\nReturn only the PromQL query:")

	return promptBuilder.String(), nil
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
