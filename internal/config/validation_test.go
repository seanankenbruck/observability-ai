package config

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	t.Run("valid config passes validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
			},
			Redis: RedisConfig{
				Addr: "localhost:6379",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:     "test-secret-key",
				JWTExpiry:     24 * time.Hour,
				SessionExpiry: 7 * 24 * time.Hour,
				RateLimit:     100,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "debug",
			},
			Query: QueryConfig{
				MaxResultSamples:     10,
				MaxResultTimepoints:  50,
				Timeout:              30 * time.Second,
				CacheTTL:             5 * time.Minute,
				MaxQueryLength:       500,
				MaxNestingDepth:      3,
				MaxTimeRangeDays:     7,
				EnableSafetyChecks:   true,
				ForbiddenMetricNames: []string{".*_secret.*"},
			},
		}

		err := cfg.Validate()
		if err != nil {
			t.Errorf("expected no validation errors, got: %v", err)
		}
	})

	t.Run("missing database host fails validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
			},
			Redis: RedisConfig{Addr: "localhost:6379"},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:     "test-secret",
				JWTExpiry:     24 * time.Hour,
				SessionExpiry: 7 * 24 * time.Hour,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "debug",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
			},
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("expected validation error for missing database host")
		}
		if !strings.Contains(err.Error(), "Database.Host") {
			t.Errorf("expected error about Database.Host, got: %v", err)
		}
	})

	t.Run("missing Claude API key fails validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
			},
			Redis: RedisConfig{Addr: "localhost:6379"},
			Claude: ClaudeConfig{
				Model: "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:     "test-secret",
				JWTExpiry:     24 * time.Hour,
				SessionExpiry: 7 * 24 * time.Hour,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "debug",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
			},
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("expected validation error for missing Claude API key")
		}
		if !strings.Contains(err.Error(), "Claude.APIKey") {
			t.Errorf("expected error about Claude.APIKey, got: %v", err)
		}
	})

	t.Run("invalid gin mode fails validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
			},
			Redis: RedisConfig{Addr: "localhost:6379"},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:     "test-secret",
				JWTExpiry:     24 * time.Hour,
				SessionExpiry: 7 * 24 * time.Hour,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "invalid-mode",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
			},
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("expected validation error for invalid gin mode")
		}
		if !strings.Contains(err.Error(), "Server.GinMode") {
			t.Errorf("expected error about Server.GinMode, got: %v", err)
		}
	})

	t.Run("invalid mimir auth type fails validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
			},
			Redis: RedisConfig{Addr: "localhost:6379"},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "invalid",
			},
			Auth: AuthConfig{
				JWTSecret:     "test-secret",
				JWTExpiry:     24 * time.Hour,
				SessionExpiry: 7 * 24 * time.Hour,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "debug",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
			},
		}

		err := cfg.Validate()
		if err == nil {
			t.Error("expected validation error for invalid mimir auth type")
		}
		if !strings.Contains(err.Error(), "Mimir.AuthType") {
			t.Errorf("expected error about Mimir.AuthType, got: %v", err)
		}
	})
}

func TestProductionValidation(t *testing.T) {
	t.Run("production config with secure values passes", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "prod-db.example.com",
				Port:     "5432",
				Database: "prod_db",
				Username: "prod_user",
				Password: "secure-random-password-123",
			},
			Redis: RedisConfig{
				Addr:     "prod-redis:6379",
				Password: "secure-redis-password",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-prod-key",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://prod-mimir:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:      "super-secure-jwt-secret-with-at-least-32-characters",
				JWTExpiry:      24 * time.Hour,
				SessionExpiry:  7 * 24 * time.Hour,
				AllowAnonymous: false,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "release",
			},
			Query: QueryConfig{
				MaxResultSamples:     10,
				MaxResultTimepoints:  50,
				Timeout:              30 * time.Second,
				CacheTTL:             5 * time.Minute,
				MaxQueryLength:       500,
				MaxNestingDepth:      3,
				MaxTimeRangeDays:     7,
				EnableSafetyChecks:   true,
				ForbiddenMetricNames: []string{".*_secret.*"},
			},
		}

		err := cfg.ValidateProduction()
		if err != nil {
			t.Errorf("expected no production validation errors, got: %v", err)
		}
	})

	t.Run("default database password fails production validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
				Password: "changeme",
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "secure-password",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:      "super-secure-jwt-secret-with-at-least-32-characters",
				JWTExpiry:      24 * time.Hour,
				SessionExpiry:  7 * 24 * time.Hour,
				AllowAnonymous: false,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "release",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
				EnableSafetyChecks:  true,
			},
		}

		err := cfg.ValidateProduction()
		if err == nil {
			t.Error("expected production validation error for default database password")
		}
		if !strings.Contains(err.Error(), "Database.Password") {
			t.Errorf("expected error about Database.Password, got: %v", err)
		}
	})

	t.Run("short JWT secret fails production validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
				Password: "secure-password",
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "secure-redis-pass",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:      "short",
				JWTExpiry:      24 * time.Hour,
				SessionExpiry:  7 * 24 * time.Hour,
				AllowAnonymous: false,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "release",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
				EnableSafetyChecks:  true,
			},
		}

		err := cfg.ValidateProduction()
		if err == nil {
			t.Error("expected production validation error for short JWT secret")
		}
		if !strings.Contains(err.Error(), "JWT secret should be at least 32 characters") {
			t.Errorf("expected error about JWT secret length, got: %v", err)
		}
	})

	t.Run("debug mode fails production validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
				Password: "secure-password",
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "secure-redis-pass",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:      "super-secure-jwt-secret-with-at-least-32-characters",
				JWTExpiry:      24 * time.Hour,
				SessionExpiry:  7 * 24 * time.Hour,
				AllowAnonymous: false,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "debug",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
				EnableSafetyChecks:  true,
			},
		}

		err := cfg.ValidateProduction()
		if err == nil {
			t.Error("expected production validation error for debug mode")
		}
		if !strings.Contains(err.Error(), "release") {
			t.Errorf("expected error about release mode, got: %v", err)
		}
	})

	t.Run("anonymous access enabled fails production validation", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     "5432",
				Database: "testdb",
				Username: "testuser",
				Password: "secure-password",
			},
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Password: "secure-redis-pass",
			},
			Claude: ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-3-haiku-20240307",
			},
			Mimir: MimirConfig{
				Endpoint: "http://localhost:9009",
				AuthType: "none",
			},
			Auth: AuthConfig{
				JWTSecret:      "super-secure-jwt-secret-with-at-least-32-characters",
				JWTExpiry:      24 * time.Hour,
				SessionExpiry:  7 * 24 * time.Hour,
				AllowAnonymous: true,
			},
			Server: ServerConfig{
				Port:    "8080",
				GinMode: "release",
			},
			Query: QueryConfig{
				MaxResultSamples:    10,
				MaxResultTimepoints: 50,
				Timeout:             30 * time.Second,
				MaxQueryLength:      500,
				MaxNestingDepth:     3,
				MaxTimeRangeDays:    7,
				EnableSafetyChecks:  true,
			},
		}

		err := cfg.ValidateProduction()
		if err == nil {
			t.Error("expected production validation error for anonymous access")
		}
		if !strings.Contains(err.Error(), "AllowAnonymous") {
			t.Errorf("expected error about AllowAnonymous, got: %v", err)
		}
	})
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		ginMode  string
		expected bool
	}{
		{"release mode is production", "release", true},
		{"debug mode is not production", "debug", false},
		{"test mode is not production", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					GinMode: tt.ginMode,
				},
			}

			if cfg.IsProduction() != tt.expected {
				t.Errorf("expected IsProduction() = %v, got %v", tt.expected, cfg.IsProduction())
			}
		})
	}
}
