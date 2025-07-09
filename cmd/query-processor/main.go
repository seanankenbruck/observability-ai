package main

import (
	"log"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/llm"
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
	llmClient, err := llm.NewOpenAIClient(getEnv("OPENAI_API_KEY", ""))
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

	// Create query processor
	qp := processor.NewQueryProcessor(llmClient, semanticMapper, rdb)

	// Setup routes and start server
	r := qp.SetupRoutes()

	port := getEnv("PORT", "8080")
	log.Printf("Query processor starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
