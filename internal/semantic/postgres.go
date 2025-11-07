package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

// PostgresConfig holds PostgreSQL connection configuration
type PostgresConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

// PostgresMapper implements the Mapper interface using PostgreSQL
type PostgresMapper struct {
	db *sql.DB
}

// NewPostgresMapper creates a new PostgreSQL-based semantic mapper
func NewPostgresMapper(config PostgresConfig) (*PostgresMapper, error) {
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username, config.Password, config.Database, config.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresMapper{db: db}, nil
}

// Ping tests the database connection
func (pm *PostgresMapper) Ping(ctx context.Context) error {
	return pm.db.PingContext(ctx)
}

// Close closes the database connection
func (pm *PostgresMapper) Close() error {
	return pm.db.Close()
}

// GetServices retrieves all services
func (pm *PostgresMapper) GetServices(ctx context.Context) ([]Service, error) {
	query := `
		SELECT id, name, namespace, labels, metric_names, created_at, updated_at
		FROM services
		ORDER BY name
	`

	rows, err := pm.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var service Service
		var labelsJSON, metricNamesJSON sql.NullString

		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Namespace,
			&labelsJSON,
			&metricNamesJSON,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service row: %w", err)
		}

		// Parse JSON fields
		if labelsJSON.Valid {
			if err := json.Unmarshal([]byte(labelsJSON.String), &service.Labels); err != nil {
				return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
			}
		}
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}

		if metricNamesJSON.Valid {
			if err := json.Unmarshal([]byte(metricNamesJSON.String), &service.MetricNames); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metric names: %w", err)
			}
		}
		if service.MetricNames == nil {
			service.MetricNames = []string{}
		}

		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating service rows: %w", err)
	}

	return services, nil
}

// GetMetrics retrieves metrics for a specific service
func (pm *PostgresMapper) GetMetrics(ctx context.Context, serviceID string) ([]Metric, error) {
	query := `
		SELECT id, name, type, description, labels, service_id, created_at, updated_at
		FROM metrics
		WHERE service_id = $1
		ORDER BY name
	`

	rows, err := pm.db.QueryContext(ctx, query, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var metric Metric
		var descriptionNull sql.NullString
		var labelsJSON sql.NullString

		err := rows.Scan(
			&metric.ID,
			&metric.Name,
			&metric.Type,
			&descriptionNull,
			&labelsJSON,
			&metric.ServiceID,
			&metric.CreatedAt,
			&metric.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric row: %w", err)
		}

		// Handle nullable description
		if descriptionNull.Valid {
			metric.Description = descriptionNull.String
		}

		// Parse labels JSON
		if labelsJSON.Valid {
			if err := json.Unmarshal([]byte(labelsJSON.String), &metric.Labels); err != nil {
				return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
			}
		}
		if metric.Labels == nil {
			metric.Labels = make(map[string]string)
		}

		metrics = append(metrics, metric)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metric rows: %w", err)
	}

	return metrics, nil
}

// FindSimilarQueries finds queries similar to the given embedding using cosine similarity
func (pm *PostgresMapper) FindSimilarQueries(ctx context.Context, embedding []float32) ([]SimilarQuery, error) {
	// Convert float32 slice to pgvector.Vector
	vector := pgvector.NewVector(embedding)

	query := `
		SELECT id, query_text, promql_template,
		       1 - (embedding <=> $1) as similarity,
		       created_at
		FROM query_embeddings
		WHERE 1 - (embedding <=> $1) > 0.8
		ORDER BY similarity DESC
		LIMIT 5
	`

	rows, err := pm.db.QueryContext(ctx, query, vector)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar queries: %w", err)
	}
	defer rows.Close()

	var similarQueries []SimilarQuery
	for rows.Next() {
		var sq SimilarQuery
		err := rows.Scan(
			&sq.ID,
			&sq.Query,
			&sq.PromQL,
			&sq.Similarity,
			&sq.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan similar query row: %w", err)
		}

		similarQueries = append(similarQueries, sq)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating similar query rows: %w", err)
	}

	return similarQueries, nil
}

// GetServiceByName retrieves a service by name
func (pm *PostgresMapper) GetServiceByName(ctx context.Context, name, namespace string) (*Service, error) {
	query := `
		SELECT id, name, namespace, labels, metric_names, created_at, updated_at
		FROM services
		WHERE LOWER(name) = LOWER($1) AND LOWER(namespace) = LOWER($2)
		LIMIT 1
	`

	var service Service
	var labelsJSON, metricNamesJSON sql.NullString

	err := pm.db.QueryRowContext(ctx, query, name, namespace).Scan(
		&service.ID,
		&service.Name,
		&service.Namespace,
		&labelsJSON,
		&metricNamesJSON,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service not found: %s", name)
		}
		return nil, fmt.Errorf("failed to query service by name: %w", err)
	}

	// Parse JSON fields
	if labelsJSON.Valid {
		if err := json.Unmarshal([]byte(labelsJSON.String), &service.Labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}
	}
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}

	if metricNamesJSON.Valid {
		if err := json.Unmarshal([]byte(metricNamesJSON.String), &service.MetricNames); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metric names: %w", err)
		}
	}
	if service.MetricNames == nil {
		service.MetricNames = []string{}
	}

	return &service, nil
}

// StoreQueryEmbedding stores a query embedding for future similarity search
func (pm *PostgresMapper) StoreQueryEmbedding(ctx context.Context, query string, embedding []float32, promql string) error {
	// Convert to pgvector.Vector
	vector := pgvector.NewVector(embedding)

	insertQuery := `
		INSERT INTO query_embeddings (id, query_text, embedding, promql_template, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (query_text) DO UPDATE SET
			embedding = $3,
			promql_template = $4,
			updated_at = $5
	`

	id := uuid.New().String()
	now := time.Now()

	_, err := pm.db.ExecContext(ctx, insertQuery, id, query, vector, promql, now)
	if err != nil {
		return fmt.Errorf("failed to store query embedding: %w", err)
	}

	return nil
}

// UpdateServiceMetrics updates the metric names for a service
func (pm *PostgresMapper) UpdateServiceMetrics(ctx context.Context, serviceID string, metrics []string) error {
	metricNamesJSON, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metric names: %w", err)
	}

	// Update the service's metric_names field
	query := `
		UPDATE services
		SET metric_names = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := pm.db.ExecContext(ctx, query, metricNamesJSON, time.Now(), serviceID)
	if err != nil {
		return fmt.Errorf("failed to update service metrics: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("service not found: %s", serviceID)
	}

	// Insert/update individual metric rows in the metrics table
	// Use INSERT ... ON CONFLICT to handle duplicates
	for _, metricName := range metrics {
		metricID := uuid.New().String()

		// Determine metric type from name (simple heuristic)
		metricType := "gauge"
		if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") {
			metricType = "counter"
		} else if strings.HasSuffix(metricName, "_bucket") {
			metricType = "histogram"
		}

		metricQuery := `
			INSERT INTO metrics (id, name, type, service_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (name, service_id)
			DO UPDATE SET type = EXCLUDED.type, updated_at = EXCLUDED.updated_at
		`

		now := time.Now()
		_, err := pm.db.ExecContext(ctx, metricQuery, metricID, metricName, metricType, serviceID, now, now)
		if err != nil {
			// Log error but continue with other metrics
			fmt.Printf("Warning: failed to insert metric %s: %v\n", metricName, err)
		}
	}

	return nil
}

// CreateService creates a new service
func (pm *PostgresMapper) CreateService(ctx context.Context, name, namespace string, labels map[string]string) (*Service, error) {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal labels: %w", err)
	}

	metricNamesJSON, err := json.Marshal([]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal empty metric names: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO services (id, name, namespace, labels, metric_names, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, namespace, labels, metric_names, created_at, updated_at
	`

	var service Service
	var labelsJSONResult, metricNamesJSONResult sql.NullString

	err = pm.db.QueryRowContext(ctx, query, id, name, namespace, labelsJSON, metricNamesJSON, now, now).Scan(
		&service.ID,
		&service.Name,
		&service.Namespace,
		&labelsJSONResult,
		&metricNamesJSONResult,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique violation
			return nil, fmt.Errorf("service already exists: %s", name)
		}
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	// Parse JSON fields
	if labelsJSONResult.Valid {
		if err := json.Unmarshal([]byte(labelsJSONResult.String), &service.Labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}
	}
	if service.Labels == nil {
		service.Labels = make(map[string]string)
	}

	if metricNamesJSONResult.Valid {
		if err := json.Unmarshal([]byte(metricNamesJSONResult.String), &service.MetricNames); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metric names: %w", err)
		}
	}
	if service.MetricNames == nil {
		service.MetricNames = []string{}
	}

	return &service, nil
}

// CreateMetric creates a new metric
func (pm *PostgresMapper) CreateMetric(ctx context.Context, name, metricType, description, serviceID string, labels map[string]string) (*Metric, error) {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal labels: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO metrics (id, name, type, description, labels, service_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, type, description, labels, service_id, created_at, updated_at
	`

	var metric Metric
	var labelsJSONResult sql.NullString

	err = pm.db.QueryRowContext(ctx, query, id, name, metricType, description, labelsJSON, serviceID, now, now).Scan(
		&metric.ID,
		&metric.Name,
		&metric.Type,
		&metric.Description,
		&labelsJSONResult,
		&metric.ServiceID,
		&metric.CreatedAt,
		&metric.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique violation
			return nil, fmt.Errorf("metric already exists: %s", name)
		}
		return nil, fmt.Errorf("failed to create metric: %w", err)
	}

	// Parse labels JSON
	if labelsJSONResult.Valid {
		if err := json.Unmarshal([]byte(labelsJSONResult.String), &metric.Labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}
	}
	if metric.Labels == nil {
		metric.Labels = make(map[string]string)
	}

	return &metric, nil
}

// DeleteService deletes a service and all its metrics
func (pm *PostgresMapper) DeleteService(ctx context.Context, serviceID string) error {
	tx, err := pm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete metrics first (foreign key constraint)
	_, err = tx.ExecContext(ctx, "DELETE FROM metrics WHERE service_id = $1", serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete metrics: %w", err)
	}

	// Delete service
	result, err := tx.ExecContext(ctx, "DELETE FROM services WHERE id = $1", serviceID)
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("service not found: %s", serviceID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SearchServices searches for services by name or namespace
func (pm *PostgresMapper) SearchServices(ctx context.Context, searchTerm string) ([]Service, error) {
	query := `
		SELECT id, name, namespace, labels, metric_names, created_at, updated_at
		FROM services
		WHERE LOWER(name) LIKE LOWER($1) OR LOWER(namespace) LIKE LOWER($1)
		ORDER BY name
		LIMIT 20
	`

	searchPattern := "%" + strings.ToLower(searchTerm) + "%"

	rows, err := pm.db.QueryContext(ctx, query, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search services: %w", err)
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var service Service
		var labelsJSON, metricNamesJSON sql.NullString

		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Namespace,
			&labelsJSON,
			&metricNamesJSON,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service row: %w", err)
		}

		// Parse JSON fields
		if labelsJSON.Valid {
			if err := json.Unmarshal([]byte(labelsJSON.String), &service.Labels); err != nil {
				return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
			}
		}
		if service.Labels == nil {
			service.Labels = make(map[string]string)
		}

		if metricNamesJSON.Valid {
			if err := json.Unmarshal([]byte(metricNamesJSON.String), &service.MetricNames); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metric names: %w", err)
			}
		}
		if service.MetricNames == nil {
			service.MetricNames = []string{}
		}

		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating service rows: %w", err)
	}

	return services, nil
}
