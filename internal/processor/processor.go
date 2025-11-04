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
	"github.com/seanankenbruck/observability-ai/internal/llm"
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
}

// NewQueryProcessor creates a new query processor instance
func NewQueryProcessor(llmClient llm.Client, semanticMapper semantic.Mapper, cache *redis.Client) *QueryProcessor {
	return &QueryProcessor{
		llmClient:        llmClient,
		semanticMapper:   semanticMapper,
		cache:            cache,
		safetyChecker:    NewSafetyChecker(),
		intentClassifier: NewIntentClassifier(),
	}
}

// ProcessQuery handles the main query processing logic
func (qp *QueryProcessor) ProcessQuery(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	start := time.Now()

	// Check cache first
	if cachedResult, err := qp.getCachedResult(ctx, req.Query); err == nil {
		cachedResult.CacheHit = true
		cachedResult.ProcessingTime = time.Since(start)
		return cachedResult, nil
	}

	// Classify intent
	intent, err := qp.intentClassifier.ClassifyIntent(req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to classify intent: %w", err)
	}

	// Generate embeddings for semantic search
	embedding, err := qp.llmClient.GetEmbedding(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	// Find similar queries
	similarQueries, err := qp.semanticMapper.FindSimilarQueries(ctx, embedding)
	if err != nil {
		// Log warning but don't fail - similar queries are optional
		fmt.Printf("Warning: failed to find similar queries: %v\n", err)
	}

	// Build enhanced prompt
	prompt, err := qp.buildPrompt(ctx, req, intent, similarQueries)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate PromQL using LLM
	llmResponse, err := qp.llmClient.GenerateQuery(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query: %w", err)
	}

	// Validate query safety
	if err := qp.safetyChecker.ValidateQuery(llmResponse.PromQL); err != nil {
		return nil, fmt.Errorf("query safety check failed: %w", err)
	}

	// Build response
	response := &QueryResponse{
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
		fmt.Printf("Warning: failed to cache result: %v\n", err)
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

// SetupRoutes configures HTTP routes
func (qp *QueryProcessor) SetupRoutes() *gin.Engine {
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

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Main query endpoint
		api.POST("/query", func(c *gin.Context) {
			var req QueryRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			response, err := qp.ProcessQuery(c.Request.Context(), &req)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

		// Future: Query suggestions
		api.GET("/suggestions", qp.handleGetSuggestions)
	}

	// Serve static files for the web interface (in production)
	r.Static("/static", "./web/dist/assets")
	r.StaticFile("/", "./web/dist/index.html")

	return r
}

// Service-related handlers
func (qp *QueryProcessor) handleGetServices(c *gin.Context) {
	services, err := qp.semanticMapper.GetServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func (qp *QueryProcessor) handleGetService(c *gin.Context) {
	serviceID := c.Param("id")
	// For now, we'll search by name since that's what we have
	service, err := qp.semanticMapper.GetServiceByName(c.Request.Context(), serviceID, "default")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, services)
}

func (qp *QueryProcessor) handleGetServiceMetrics(c *gin.Context) {
	serviceID := c.Param("id")
	metrics, err := qp.semanticMapper.GetMetrics(c.Request.Context(), serviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (qp *QueryProcessor) handleGetAllMetrics(c *gin.Context) {
	// Get all services first, then get metrics for each
	services, err := qp.semanticMapper.GetServices(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var allMetrics []interface{}
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

// Utility function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
