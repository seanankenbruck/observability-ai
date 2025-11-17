package main

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/auth"
	"github.com/seanankenbruck/observability-ai/internal/config"
	"github.com/seanankenbruck/observability-ai/internal/llm"
	"github.com/seanankenbruck/observability-ai/internal/mimir"
	"github.com/seanankenbruck/observability-ai/internal/observability"
	"github.com/seanankenbruck/observability-ai/internal/processor"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

func main() {
	ctx := context.Background()

	// Load configuration using the new config package
	loader := config.NewDefaultLoader()
	cfg := loader.MustLoad(ctx)

	// Validate configuration
	if err := cfg.ValidateWithContext(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	log.Printf("Configuration loaded successfully from provider chain")
	if cfg.IsProduction() {
		log.Printf("Running in PRODUCTION mode - production validation enabled")
	}

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Initialize LLM client
	llmClient, err := llm.NewClaudeClient(cfg.Claude.APIKey, cfg.Claude.Model)
	if err != nil {
		log.Fatal("Failed to initialize LLM client:", err)
	}

	// Initialize semantic mapper
	semanticMapper, err := semantic.NewPostgresMapper(semantic.PostgresConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		Database: cfg.Database.Database,
		Username: cfg.Database.Username,
		Password: cfg.Database.Password,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		log.Fatal("Failed to initialize semantic mapper:", err)
	}

	// Initialize Mimir client
	mimirClient := mimir.NewClient(
		cfg.Mimir.Endpoint,
		mimir.AuthConfig{
			Type:        cfg.Mimir.AuthType,
			Username:    cfg.Mimir.Username,
			Password:    cfg.Mimir.Password,
			BearerToken: cfg.Mimir.BearerToken,
			TenantID:    cfg.Mimir.TenantID,
		},
		cfg.Mimir.Timeout,
	)

	// Initialize discovery service
	discoveryConfig := mimir.DiscoveryConfig{
		Enabled:           cfg.Discovery.Enabled,
		Interval:          cfg.Discovery.Interval,
		Namespaces:        cfg.Discovery.Namespaces,
		ServiceLabelNames: cfg.Discovery.ServiceLabelNames,
		ExcludeMetrics:    cfg.Discovery.ExcludeMetrics,
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
		JWTSecret:      cfg.Auth.JWTSecret,
		JWTExpiry:      cfg.Auth.JWTExpiry,
		SessionExpiry:  cfg.Auth.SessionExpiry,
		RateLimit:      cfg.Auth.RateLimit,
		AllowAnonymous: cfg.Auth.AllowAnonymous,
	})

	// Start auth cleanup routine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			authManager.CleanupExpired()
		}
	}()

	// Initialize observability
	logger := observability.NewLogger("main")
	healthChecker := observability.NewHealthChecker()

	// Register health checks
	healthChecker.Register("database", observability.DatabaseHealthCheck(func(ctx context.Context) error {
		return semanticMapper.Ping(ctx)
	}))

	healthChecker.Register("redis", observability.RedisHealthCheck(func(ctx context.Context) error {
		return rdb.Ping(ctx).Err()
	}))

	healthChecker.Register("memory", observability.MemoryHealthCheck(func() (uint64, uint64) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.Alloc, m.Sys
	}))

	// Register LLM health check
	healthChecker.Register("llm_service", observability.LLMHealthCheck(func(ctx context.Context) error {
		// Simple health check - try to generate a minimal embedding
		_, err := llmClient.GetEmbedding(ctx, "health check")
		return err
	}))

	// Register Mimir health check
	healthChecker.Register("mimir", observability.MimirHealthCheck(func(ctx context.Context) error {
		return mimirClient.TestConnection(ctx)
	}))

	// Create query processor
	qp := processor.NewQueryProcessor(llmClient, semanticMapper, rdb)
	qp.SetHealthChecker(healthChecker)

	// Setup Gin router with authentication
	router := qp.SetupRoutes(authManager)

	// Add observability middleware
	router.Use(observability.RecoveryMiddleware(logger))
	router.Use(observability.RequestLoggingMiddleware(logger))
	router.Use(observability.MetricsMiddleware())

	// Add metrics endpoint
	router.GET("/metrics", func(c *gin.Context) {
		metrics := observability.GetGlobalMetrics().GetAll()
		c.JSON(200, gin.H{
			"metrics":   metrics,
			"timestamp": time.Now(),
		})
	})

	// Note: /health endpoint is registered in processor.SetupRoutes()

	// Add auth handlers for login/logout/user management
	authHandlers := auth.NewAuthHandlers(authManager)
	authHandlers.SetupRoutes(router.Group("/api/v1"))

	logger.Info(context.Background(), "Query processor starting", map[string]interface{}{
		"port":    cfg.Server.Port,
		"version": "1.0.0",
		"mode":    cfg.Server.GinMode,
	})
	if err := router.Run(":" + cfg.Server.Port); err != nil {
		logger.Error(context.Background(), "Failed to start server", err, nil)
		log.Fatal("Failed to start server:", err)
	}
}
