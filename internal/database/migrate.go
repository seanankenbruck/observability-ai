package database

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// MigrationConfig holds migration configuration
type MigrationConfig struct {
	DatabaseURL    string
	MigrationsPath string
}

// RunMigrations runs database migrations
func RunMigrations(config MigrationConfig) error {
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create migration driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", config.MigrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// CreateDatabase verifies the database exists and is accessible
func CreateDatabase(host, port, username, password, dbname string) error {
	// Try to connect directly to the target database
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, username, password, dbname)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Check if database exists and is accessible
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)`
	err = db.QueryRow(checkQuery, dbname).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		return fmt.Errorf("database %s does not exist", dbname)
	}

	fmt.Printf("Database %s is accessible\n", dbname)
	return nil
}

// HealthCheck performs a basic database health check
func HealthCheck(db *sql.DB) error {
	// Test basic connectivity
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Test pgvector extension
	var hasVector bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&hasVector)
	if err != nil {
		return fmt.Errorf("failed to check vector extension: %w", err)
	}

	if !hasVector {
		return fmt.Errorf("pgvector extension is not installed")
	}

	// Test basic query
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM services").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query services table: %w", err)
	}

	return nil
}
