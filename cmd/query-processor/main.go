package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/auth"
	"github.com/seanankenbruck/observability-ai/internal/llm"
	"github.com/seanankenbruck/observability-ai/internal/mimir"
	"github.com/seanankenbruck/observability-ai/internal/processor"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

func main() {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	})

	// Initialize LLM client
	llmClient, err := llm.NewClaudeClient(getEnv("CLAUDE_API_KEY", ""), getEnv("CLAUDE_MODEL", ""))
	if err != nil {
		log.Fatal("Failed to initialize LLM client:", err)
	}

	// Initialize semantic mapper
	semanticMapper, err := semantic.NewPostgresMapper(semantic.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		Database: getEnv("DB_NAME", "observability_ai"),
		Username: getEnv("DB_USER", "obs_ai"),
		Password: getEnv("DB_PASSWORD", "changeme"),
	})
	if err != nil {
		log.Fatal("Failed to initialize semantic mapper:", err)
	}

	// Initialize Mimir client
	mimirClient := mimir.NewClient(
		getEnv("MIMIR_ENDPOINT", "http://localhost:9009"),
		mimir.AuthConfig{
			Type:        getEnv("MIMIR_AUTH_TYPE", "none"),
			Username:    getEnv("MIMIR_USERNAME", ""),
			Password:    getEnv("MIMIR_PASSWORD", ""),
			BearerToken: getEnv("MIMIR_BEARER_TOKEN", ""),
			TenantID:    getEnv("MIMIR_TENANT_ID", "demo"),
		},
		30*time.Second,
	)

	// Initialize discovery service
	discoveryConfig := mimir.DiscoveryConfig{
		Enabled:           getEnvBool("DISCOVERY_ENABLED", true),
		Interval:          getEnvDuration("DISCOVERY_INTERVAL", 5*time.Minute),
		Namespaces:        getEnvSlice("DISCOVERY_NAMESPACES", []string{}),
		ServiceLabelNames: getEnvSlice("SERVICE_LABEL_NAMES", []string{"service", "job", "app"}),
		ExcludeMetrics:    getEnvSlice("EXCLUDE_METRICS", []string{"go_.*", "process_.*"}),
	}

	discoveryService := mimir.NewDiscoveryService(mimirClient, discoveryConfig, semanticMapper)

	// Start discovery in background
	if discoveryConfig.Enabled {
		if err := discoveryService.Start(context.Background()); err != nil {
			log.Printf("Warning: Failed to start discovery service: %v", err)
		} else {
			log.Println("Discovery service started successfully")
		}
		defer discoveryService.Stop()
	}

	// Initialize auth manager
	authManager := auth.NewAuthManager(auth.AuthConfig{
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTExpiry:      getEnvDuration("JWT_EXPIRY", 24*time.Hour),
		SessionExpiry:  getEnvDuration("SESSION_EXPIRY", 7*24*time.Hour),
		RateLimit:      getEnvInt("RATE_LIMIT", 100),
		AllowAnonymous: getEnvBool("ALLOW_ANONYMOUS", false),
	})

	// Start auth cleanup routine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			authManager.CleanupExpired()
		}
	}()

	// Create query processor
	qp := processor.NewQueryProcessor(llmClient, semanticMapper, rdb)

	// Setup Gin router
	router := qp.SetupRoutes()

	// Add auth handlers
	authHandlers := auth.NewAuthHandlers(authManager)
	authHandlers.SetupRoutes(router.Group("/api/v1"))

	// Apply auth middleware to protected routes
	// Note: Health endpoints are already excluded in middleware
	protected := router.Group("/api/v1")
	protected.Use(authManager.Middleware())
	// Your protected routes will use this group

	port := getEnv("PORT", "8080")
	log.Printf("Query processor starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
