package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Database configuration
	Database DatabaseConfig

	// Redis configuration
	Redis RedisConfig

	// Claude LLM configuration
	Claude ClaudeConfig

	// Mimir configuration
	Mimir MimirConfig

	// Discovery configuration
	Discovery DiscoveryConfig

	// Authentication configuration
	Auth AuthConfig

	// Server configuration
	Server ServerConfig

	// Query configuration
	Query QueryConfig
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// ClaudeConfig holds Claude API configuration
type ClaudeConfig struct {
	APIKey string
	Model  string
}

// MimirConfig holds Mimir/Prometheus configuration
type MimirConfig struct {
	Endpoint    string
	AuthType    string
	Username    string
	Password    string
	BearerToken string
	TenantID    string
	Timeout     time.Duration
	BackendType string // "auto", "mimir", "prometheus"
}

// DiscoveryConfig holds service discovery configuration
type DiscoveryConfig struct {
	Enabled           bool
	Interval          time.Duration
	Namespaces        []string
	ServiceLabelNames []string
	ExcludeMetrics    []string
}

// AuthConfig holds authentication and authorization configuration
type AuthConfig struct {
	JWTSecret      string
	JWTExpiry      time.Duration
	SessionExpiry  time.Duration
	RateLimit      int
	AllowAnonymous bool
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port    string
	GinMode string
}

// QueryConfig holds query processing configuration
type QueryConfig struct {
	MaxResultSamples     int
	MaxResultTimepoints  int
	Timeout              time.Duration
	CacheTTL             time.Duration
	MaxQueryLength       int
	MaxNestingDepth      int
	MaxTimeRangeDays     int
	EnableSafetyChecks   bool
	ForbiddenMetricNames []string
}

// Loader handles loading configuration from various sources
type Loader struct {
	provider SecretProvider
}

// NewLoader creates a new configuration loader with the given secret provider
func NewLoader(provider SecretProvider) *Loader {
	return &Loader{
		provider: provider,
	}
}

// NewDefaultLoader creates a loader with the default provider chain:
// 1. Kubernetes secrets (if available)
// 2. File-based secrets (if available)
// 3. Environment variables (fallback)
func NewDefaultLoader() *Loader {
	providers := []SecretProvider{
		NewK8sProvider("", ""),           // Auto-detect K8s environment
		NewFileProvider("/var/secrets"),  // Common secret mount path
		NewEnvProvider(),                 // Always available fallback
	}

	return &Loader{
		provider: NewChainProvider(providers...),
	}
}

// Load loads the complete configuration
func (l *Loader) Load(ctx context.Context) (*Config, error) {
	cfg := &Config{}

	// Load Database config
	cfg.Database = DatabaseConfig{
		Host:     l.getString(ctx, "DB_HOST", "localhost"),
		Port:     l.getString(ctx, "DB_PORT", "5432"),
		Database: l.getString(ctx, "DB_NAME", "observability_ai"),
		Username: l.getString(ctx, "DB_USER", "obs_ai"),
		Password: l.getString(ctx, "DB_PASSWORD", ""),
		SSLMode:  l.getString(ctx, "DB_SSLMODE", "disable"),
	}

	// Load Redis config
	cfg.Redis = RedisConfig{
		Addr:     l.getString(ctx, "REDIS_ADDR", "localhost:6379"),
		Password: l.getString(ctx, "REDIS_PASSWORD", ""),
		DB:       l.getInt(ctx, "REDIS_DB", 0),
	}

	// Load Claude config
	cfg.Claude = ClaudeConfig{
		APIKey: l.getString(ctx, "CLAUDE_API_KEY", ""),
		Model:  l.getString(ctx, "CLAUDE_MODEL", "claude-3-haiku-20240307"),
	}

	// Load Mimir config
	cfg.Mimir = MimirConfig{
		Endpoint:    l.getString(ctx, "MIMIR_ENDPOINT", "http://localhost:9009"),
		AuthType:    l.getString(ctx, "MIMIR_AUTH_TYPE", "none"),
		Username:    l.getString(ctx, "MIMIR_USERNAME", ""),
		Password:    l.getString(ctx, "MIMIR_PASSWORD", ""),
		BearerToken: l.getString(ctx, "MIMIR_BEARER_TOKEN", ""),
		TenantID:    l.getString(ctx, "MIMIR_TENANT_ID", "demo"),
		Timeout:     l.getDuration(ctx, "MIMIR_TIMEOUT", 30*time.Second),
		BackendType: l.getString(ctx, "MIMIR_BACKEND_TYPE", "auto"),
	}

	// Load Discovery config
	cfg.Discovery = DiscoveryConfig{
		Enabled:           l.getBool(ctx, "DISCOVERY_ENABLED", true),
		Interval:          l.getDuration(ctx, "DISCOVERY_INTERVAL", 5*time.Minute),
		Namespaces:        l.getSlice(ctx, "DISCOVERY_NAMESPACES", []string{}),
		ServiceLabelNames: l.getSlice(ctx, "SERVICE_LABEL_NAMES", []string{"service", "job", "app"}),
		ExcludeMetrics:    l.getSlice(ctx, "EXCLUDE_METRICS", []string{"go_.*", "process_.*"}),
	}

	// Load Auth config
	cfg.Auth = AuthConfig{
		JWTSecret:      l.getString(ctx, "JWT_SECRET", ""),
		JWTExpiry:      l.getDuration(ctx, "JWT_EXPIRY", 24*time.Hour),
		SessionExpiry:  l.getDuration(ctx, "SESSION_EXPIRY", 7*24*time.Hour),
		RateLimit:      l.getInt(ctx, "RATE_LIMIT", 100),
		AllowAnonymous: l.getBool(ctx, "ALLOW_ANONYMOUS", false),
	}

	// Load Server config
	cfg.Server = ServerConfig{
		Port:    l.getString(ctx, "PORT", "8080"),
		GinMode: l.getString(ctx, "GIN_MODE", "debug"),
	}

	// Load Query config
	cfg.Query = QueryConfig{
		MaxResultSamples:     l.getInt(ctx, "MAX_RESULT_SAMPLES", 10),
		MaxResultTimepoints:  l.getInt(ctx, "MAX_RESULT_TIMEPOINTS", 50),
		Timeout:              l.getDuration(ctx, "QUERY_TIMEOUT", 30*time.Second),
		CacheTTL:             l.getDuration(ctx, "CACHE_TTL", 5*time.Minute),
		MaxQueryLength:       l.getInt(ctx, "MAX_QUERY_LENGTH", 500),
		MaxNestingDepth:      l.getInt(ctx, "MAX_NESTING_DEPTH", 3),
		MaxTimeRangeDays:     l.getInt(ctx, "MAX_TIME_RANGE_DAYS", 7),
		EnableSafetyChecks:   l.getBool(ctx, "ENABLE_SAFETY_CHECKS", true),
		ForbiddenMetricNames: l.getSlice(ctx, "FORBIDDEN_METRIC_NAMES", []string{".*_secret.*", ".*_password.*", ".*_token.*", ".*_key.*"}),
	}

	return cfg, nil
}

// Helper methods for retrieving and parsing configuration values

func (l *Loader) getString(ctx context.Context, key, defaultValue string) string {
	value, err := l.provider.GetSecret(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}
	return value
}

func (l *Loader) getBool(ctx context.Context, key string, defaultValue bool) bool {
	value, err := l.provider.GetSecret(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

func (l *Loader) getInt(ctx context.Context, key string, defaultValue int) int {
	value, err := l.provider.GetSecret(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return i
}

func (l *Loader) getDuration(ctx context.Context, key string, defaultValue time.Duration) time.Duration {
	value, err := l.provider.GetSecret(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return d
}

func (l *Loader) getSlice(ctx context.Context, key string, defaultValue []string) []string {
	value, err := l.provider.GetSecret(ctx, key)
	if err != nil || value == "" {
		return defaultValue
	}

	// Split by comma and trim whitespace
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// MustLoad loads configuration and panics on error
// Useful for application startup
func (l *Loader) MustLoad(ctx context.Context) *Config {
	cfg, err := l.Load(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}
