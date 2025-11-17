package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestEnvProvider(t *testing.T) {
	ctx := context.Background()

	// Set test environment variable
	os.Setenv("TEST_SECRET", "test-value")
	defer os.Unsetenv("TEST_SECRET")

	provider := NewEnvProvider()

	t.Run("retrieves existing env var", func(t *testing.T) {
		value, err := provider.GetSecret(ctx, "TEST_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "test-value" {
			t.Errorf("expected 'test-value', got '%s'", value)
		}
	})

	t.Run("returns empty for non-existent env var", func(t *testing.T) {
		value, err := provider.GetSecret(ctx, "NON_EXISTENT")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "" {
			t.Errorf("expected empty string, got '%s'", value)
		}
	})

	t.Run("is always available", func(t *testing.T) {
		if !provider.IsAvailable(ctx) {
			t.Error("env provider should always be available")
		}
	})

	t.Run("has correct name", func(t *testing.T) {
		if provider.Name() != "env" {
			t.Errorf("expected name 'env', got '%s'", provider.Name())
		}
	})
}

func TestFileProvider(t *testing.T) {
	ctx := context.Background()

	// Create temporary directory for test secrets
	tmpDir := t.TempDir()

	// Write test secret file
	secretFile := tmpDir + "/claude-api-key"
	err := os.WriteFile(secretFile, []byte("sk-ant-test-key\n"), 0600)
	if err != nil {
		t.Fatalf("failed to create test secret file: %v", err)
	}

	provider := NewFileProvider(tmpDir)

	t.Run("retrieves secret from file", func(t *testing.T) {
		value, err := provider.GetSecret(ctx, "CLAUDE_API_KEY")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "sk-ant-test-key" {
			t.Errorf("expected 'sk-ant-test-key', got '%s'", value)
		}
	})

	t.Run("returns empty for non-existent file", func(t *testing.T) {
		value, err := provider.GetSecret(ctx, "NON_EXISTENT_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "" {
			t.Errorf("expected empty string, got '%s'", value)
		}
	})

	t.Run("is available when directory exists", func(t *testing.T) {
		if !provider.IsAvailable(ctx) {
			t.Error("file provider should be available when directory exists")
		}
	})

	t.Run("is not available when directory doesn't exist", func(t *testing.T) {
		provider := NewFileProvider("/non/existent/path")
		if provider.IsAvailable(ctx) {
			t.Error("file provider should not be available for non-existent directory")
		}
	})

	t.Run("has correct name", func(t *testing.T) {
		if provider.Name() != "file" {
			t.Errorf("expected name 'file', got '%s'", provider.Name())
		}
	})
}

func TestChainProvider(t *testing.T) {
	ctx := context.Background()

	// Set up test environment
	os.Setenv("ENV_SECRET", "from-env")
	defer os.Unsetenv("ENV_SECRET")

	tmpDir := t.TempDir()
	err := os.WriteFile(tmpDir+"/file-secret", []byte("from-file"), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	envProvider := NewEnvProvider()
	fileProvider := NewFileProvider(tmpDir)
	chain := NewChainProvider(fileProvider, envProvider)

	t.Run("uses first available provider", func(t *testing.T) {
		value, err := chain.GetSecret(ctx, "FILE_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "from-file" {
			t.Errorf("expected 'from-file', got '%s'", value)
		}
	})

	t.Run("falls back to next provider", func(t *testing.T) {
		value, err := chain.GetSecret(ctx, "ENV_SECRET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "from-env" {
			t.Errorf("expected 'from-env', got '%s'", value)
		}
	})

	t.Run("returns error when all providers fail", func(t *testing.T) {
		emptyChain := NewChainProvider(NewFileProvider("/non/existent"))
		_, err := emptyChain.GetSecret(ctx, "ANY_KEY")
		if err == nil {
			t.Error("expected error when all providers fail")
		}
	})
}

func TestConfigLoader(t *testing.T) {
	ctx := context.Background()

	// Set up test environment variables
	testEnv := map[string]string{
		"DB_HOST":          "test-host",
		"DB_PORT":          "5432",
		"DB_NAME":          "test-db",
		"DB_USER":          "test-user",
		"DB_PASSWORD":      "test-pass",
		"REDIS_ADDR":       "test-redis:6379",
		"REDIS_PASSWORD":   "redis-pass",
		"CLAUDE_API_KEY":   "sk-ant-test",
		"CLAUDE_MODEL":     "claude-3-haiku-20240307",
		"MIMIR_ENDPOINT":   "http://test-mimir:9009",
		"JWT_SECRET":       "test-jwt-secret-with-sufficient-length-32chars",
		"PORT":             "8080",
		"GIN_MODE":         "debug",
		"RATE_LIMIT":       "50",
		"DISCOVERY_ENABLED": "true",
		"ALLOW_ANONYMOUS":  "false",
	}

	for k, v := range testEnv {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range testEnv {
			os.Unsetenv(k)
		}
	}()

	loader := NewLoader(NewEnvProvider())

	t.Run("loads all configuration sections", func(t *testing.T) {
		cfg, err := loader.Load(ctx)
		if err != nil {
			t.Fatalf("unexpected error loading config: %v", err)
		}

		// Verify database config
		if cfg.Database.Host != "test-host" {
			t.Errorf("expected DB host 'test-host', got '%s'", cfg.Database.Host)
		}
		if cfg.Database.Password != "test-pass" {
			t.Errorf("expected DB password 'test-pass', got '%s'", cfg.Database.Password)
		}

		// Verify Redis config
		if cfg.Redis.Addr != "test-redis:6379" {
			t.Errorf("expected Redis addr 'test-redis:6379', got '%s'", cfg.Redis.Addr)
		}

		// Verify Claude config
		if cfg.Claude.APIKey != "sk-ant-test" {
			t.Errorf("expected Claude API key 'sk-ant-test', got '%s'", cfg.Claude.APIKey)
		}

		// Verify Auth config
		if cfg.Auth.JWTSecret != "test-jwt-secret-with-sufficient-length-32chars" {
			t.Errorf("expected JWT secret, got '%s'", cfg.Auth.JWTSecret)
		}
		if cfg.Auth.RateLimit != 50 {
			t.Errorf("expected rate limit 50, got %d", cfg.Auth.RateLimit)
		}

		// Verify Server config
		if cfg.Server.Port != "8080" {
			t.Errorf("expected port '8080', got '%s'", cfg.Server.Port)
		}
	})

	t.Run("uses default values when env vars not set", func(t *testing.T) {
		// Clear all env vars
		for k := range testEnv {
			os.Unsetenv(k)
		}

		cfg, err := loader.Load(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use defaults
		if cfg.Database.Host != "localhost" {
			t.Errorf("expected default host 'localhost', got '%s'", cfg.Database.Host)
		}
		if cfg.Server.Port != "8080" {
			t.Errorf("expected default port '8080', got '%s'", cfg.Server.Port)
		}
		if cfg.Auth.RateLimit != 100 {
			t.Errorf("expected default rate limit 100, got %d", cfg.Auth.RateLimit)
		}

		// Restore env vars for other tests
		for k, v := range testEnv {
			os.Setenv(k, v)
		}
	})

	t.Run("parses durations correctly", func(t *testing.T) {
		os.Setenv("JWT_EXPIRY", "12h")
		os.Setenv("QUERY_TIMEOUT", "45s")
		defer os.Unsetenv("JWT_EXPIRY")
		defer os.Unsetenv("QUERY_TIMEOUT")

		cfg, err := loader.Load(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Auth.JWTExpiry != 12*time.Hour {
			t.Errorf("expected JWT expiry 12h, got %v", cfg.Auth.JWTExpiry)
		}
		if cfg.Query.Timeout != 45*time.Second {
			t.Errorf("expected query timeout 45s, got %v", cfg.Query.Timeout)
		}
	})

	t.Run("parses slices correctly", func(t *testing.T) {
		os.Setenv("SERVICE_LABEL_NAMES", "service,job,app,custom")
		defer os.Unsetenv("SERVICE_LABEL_NAMES")

		cfg, err := loader.Load(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"service", "job", "app", "custom"}
		if len(cfg.Discovery.ServiceLabelNames) != len(expected) {
			t.Errorf("expected %d labels, got %d", len(expected), len(cfg.Discovery.ServiceLabelNames))
		}
	})
}
