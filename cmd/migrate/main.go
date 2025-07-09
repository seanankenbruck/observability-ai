package main

import (
	"fmt"
	"log"
	"os"

	"github.com/seanankenbruck/observability-ai/internal/database"
	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

func main() {
	// Database configuration
	config := semantic.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5433"),
		Database: getEnv("DB_NAME", "observability_ai"),
		Username: getEnv("DB_USER", "obs_ai"),
		Password: getEnv("DB_PASSWORD", "changeme"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	fmt.Println("=== Running Database Migrations ===")
	fmt.Printf("Connecting to database: %s@%s:%s/%s\n", config.Username, config.Host, config.Port, config.Database)

	// Verify database connectivity
	if err := database.CreateDatabase(config.Host, config.Port, config.Username, config.Password, config.Database); err != nil {
		log.Fatalf("Database connectivity failed: %v", err)
	}
	fmt.Println("✓ Database connectivity verified")

	// Run migrations
	migrationConfig := database.MigrationConfig{
		DatabaseURL: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			config.Username, config.Password, config.Host, config.Port, config.Database, config.SSLMode),
		MigrationsPath: "./migrations",
	}

	if err := database.RunMigrations(migrationConfig); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Println("✓ Database migrations completed successfully!")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
