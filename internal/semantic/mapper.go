package semantic

import (
	"context"
)

// Mapper handles service and metric mapping
type Mapper interface {
	// Service operations
	GetServices(ctx context.Context) ([]Service, error)
	GetServiceByName(ctx context.Context, name, namespace string) (*Service, error)
	CreateService(ctx context.Context, name, namespace string, labels map[string]string) (*Service, error)
	UpdateServiceMetrics(ctx context.Context, serviceID string, metrics []string) error
	DeleteService(ctx context.Context, serviceID string) error
	SearchServices(ctx context.Context, searchTerm string) ([]Service, error)

	// Metric operations
	GetMetrics(ctx context.Context, serviceID string) ([]Metric, error)
	CreateMetric(ctx context.Context, name, metricType, description, serviceID string, labels map[string]string) (*Metric, error)

	// Query embedding operations
	FindSimilarQueries(ctx context.Context, embedding []float32) ([]SimilarQuery, error)
	StoreQueryEmbedding(ctx context.Context, query string, embedding []float32, promql string) error
}

// Service represents a monitored service
type Service struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	MetricNames []string          `json:"metric_names"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// Metric represents a metric definition
type Metric struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // counter, gauge, histogram
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	ServiceID   string            `json:"service_id"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// SimilarQuery represents a cached similar query
type SimilarQuery struct {
	ID         string  `json:"id"`
	Query      string  `json:"query"`
	PromQL     string  `json:"promql"`
	Similarity float64 `json:"similarity"`
	CreatedAt  string  `json:"created_at"`
}
