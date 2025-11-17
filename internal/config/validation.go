package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation error(s):\n", len(e)))
	for i, err := range e {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// HasErrors returns true if there are any validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validate performs comprehensive validation on the configuration
func (c *Config) Validate() error {
	var errors ValidationErrors

	// Validate Database config
	errors = append(errors, c.validateDatabase()...)

	// Validate Redis config
	errors = append(errors, c.validateRedis()...)

	// Validate Claude config
	errors = append(errors, c.validateClaude()...)

	// Validate Mimir config
	errors = append(errors, c.validateMimir()...)

	// Validate Auth config
	errors = append(errors, c.validateAuth()...)

	// Validate Server config
	errors = append(errors, c.validateServer()...)

	// Validate Query config
	errors = append(errors, c.validateQuery()...)

	if errors.HasErrors() {
		return errors
	}

	return nil
}

func (c *Config) validateDatabase() []ValidationError {
	var errors []ValidationError

	if c.Database.Host == "" {
		errors = append(errors, ValidationError{
			Field:   "Database.Host",
			Message: "database host is required",
		})
	}

	if c.Database.Port == "" {
		errors = append(errors, ValidationError{
			Field:   "Database.Port",
			Message: "database port is required",
		})
	}

	if c.Database.Database == "" {
		errors = append(errors, ValidationError{
			Field:   "Database.Database",
			Message: "database name is required",
		})
	}

	if c.Database.Username == "" {
		errors = append(errors, ValidationError{
			Field:   "Database.Username",
			Message: "database username is required",
		})
	}

	return errors
}

func (c *Config) validateRedis() []ValidationError {
	var errors []ValidationError

	if c.Redis.Addr == "" {
		errors = append(errors, ValidationError{
			Field:   "Redis.Addr",
			Message: "redis address is required",
		})
	}

	return errors
}

func (c *Config) validateClaude() []ValidationError {
	var errors []ValidationError

	if c.Claude.APIKey == "" {
		errors = append(errors, ValidationError{
			Field:   "Claude.APIKey",
			Message: "Claude API key is required",
		})
	}

	if c.Claude.Model == "" {
		errors = append(errors, ValidationError{
			Field:   "Claude.Model",
			Message: "Claude model is required",
		})
	}

	return errors
}

func (c *Config) validateMimir() []ValidationError {
	var errors []ValidationError

	if c.Mimir.Endpoint == "" {
		errors = append(errors, ValidationError{
			Field:   "Mimir.Endpoint",
			Message: "Mimir endpoint is required",
		})
	}

	// Validate auth configuration based on auth type
	switch c.Mimir.AuthType {
	case "basic":
		if c.Mimir.Username == "" || c.Mimir.Password == "" {
			errors = append(errors, ValidationError{
				Field:   "Mimir.Auth",
				Message: "basic auth requires both username and password",
			})
		}
	case "bearer":
		if c.Mimir.BearerToken == "" {
			errors = append(errors, ValidationError{
				Field:   "Mimir.BearerToken",
				Message: "bearer auth requires a token",
			})
		}
	case "none":
		// No auth required
	default:
		errors = append(errors, ValidationError{
			Field:   "Mimir.AuthType",
			Message: fmt.Sprintf("invalid auth type: %s (must be 'none', 'basic', or 'bearer')", c.Mimir.AuthType),
		})
	}

	return errors
}

func (c *Config) validateAuth() []ValidationError {
	var errors []ValidationError

	if c.Auth.JWTSecret == "" {
		errors = append(errors, ValidationError{
			Field:   "Auth.JWTSecret",
			Message: "JWT secret is required",
		})
	}

	if c.Auth.JWTExpiry <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Auth.JWTExpiry",
			Message: "JWT expiry must be positive",
		})
	}

	if c.Auth.SessionExpiry <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Auth.SessionExpiry",
			Message: "session expiry must be positive",
		})
	}

	if c.Auth.RateLimit < 0 {
		errors = append(errors, ValidationError{
			Field:   "Auth.RateLimit",
			Message: "rate limit must be non-negative",
		})
	}

	return errors
}

func (c *Config) validateServer() []ValidationError {
	var errors []ValidationError

	if c.Server.Port == "" {
		errors = append(errors, ValidationError{
			Field:   "Server.Port",
			Message: "server port is required",
		})
	}

	// Validate GinMode
	validModes := []string{"debug", "release", "test"}
	isValid := false
	for _, mode := range validModes {
		if c.Server.GinMode == mode {
			isValid = true
			break
		}
	}
	if !isValid {
		errors = append(errors, ValidationError{
			Field:   "Server.GinMode",
			Message: fmt.Sprintf("invalid gin mode: %s (must be 'debug', 'release', or 'test')", c.Server.GinMode),
		})
	}

	return errors
}

func (c *Config) validateQuery() []ValidationError {
	var errors []ValidationError

	if c.Query.MaxResultSamples <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.MaxResultSamples",
			Message: "max result samples must be positive",
		})
	}

	if c.Query.MaxResultTimepoints <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.MaxResultTimepoints",
			Message: "max result timepoints must be positive",
		})
	}

	if c.Query.Timeout <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.Timeout",
			Message: "query timeout must be positive",
		})
	}

	if c.Query.CacheTTL < 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.CacheTTL",
			Message: "cache TTL must be non-negative",
		})
	}

	if c.Query.MaxQueryLength <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.MaxQueryLength",
			Message: "max query length must be positive",
		})
	}

	if c.Query.MaxNestingDepth <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.MaxNestingDepth",
			Message: "max nesting depth must be positive",
		})
	}

	if c.Query.MaxTimeRangeDays <= 0 {
		errors = append(errors, ValidationError{
			Field:   "Query.MaxTimeRangeDays",
			Message: "max time range days must be positive",
		})
	}

	return errors
}

// ValidateProduction performs additional validation for production environments
// It checks for insecure default values that should not be used in production
func (c *Config) ValidateProduction() error {
	var errors ValidationErrors

	// Check for insecure database passwords
	if c.Database.Password == "" || c.Database.Password == "changeme" {
		errors = append(errors, ValidationError{
			Field:   "Database.Password",
			Message: "production deployment must not use default or empty database password",
		})
	}

	// Check for insecure Redis passwords
	if c.Redis.Password == "" || c.Redis.Password == "changeme" {
		errors = append(errors, ValidationError{
			Field:   "Redis.Password",
			Message: "production deployment must not use default or empty Redis password",
		})
	}

	// Check for insecure JWT secrets
	insecureJWTSecrets := []string{
		"",
		"your-secret-key-change-in-production",
		"change-this-in-production",
		"secret",
		"jwt-secret",
	}
	for _, insecure := range insecureJWTSecrets {
		if c.Auth.JWTSecret == insecure {
			errors = append(errors, ValidationError{
				Field:   "Auth.JWTSecret",
				Message: "production deployment must not use default or insecure JWT secret",
			})
			break
		}
	}

	// Check JWT secret length (should be at least 32 characters)
	if len(c.Auth.JWTSecret) < 32 {
		errors = append(errors, ValidationError{
			Field:   "Auth.JWTSecret",
			Message: "JWT secret should be at least 32 characters for production use",
		})
	}

	// Check for placeholder Claude API key
	if c.Claude.APIKey == "your-api-key-here" || c.Claude.APIKey == "" {
		errors = append(errors, ValidationError{
			Field:   "Claude.APIKey",
			Message: "production deployment requires a valid Claude API key",
		})
	}

	// Ensure Gin is in release mode for production
	if c.Server.GinMode != "release" {
		errors = append(errors, ValidationError{
			Field:   "Server.GinMode",
			Message: "production deployment should use 'release' mode",
		})
	}

	// Ensure anonymous access is disabled in production
	if c.Auth.AllowAnonymous {
		errors = append(errors, ValidationError{
			Field:   "Auth.AllowAnonymous",
			Message: "production deployment should not allow anonymous access",
		})
	}

	// Ensure safety checks are enabled in production
	if !c.Query.EnableSafetyChecks {
		errors = append(errors, ValidationError{
			Field:   "Query.EnableSafetyChecks",
			Message: "production deployment should have safety checks enabled",
		})
	}

	if errors.HasErrors() {
		return errors
	}

	return nil
}

// IsProduction determines if the current environment is production
// based on the GinMode setting
func (c *Config) IsProduction() bool {
	return c.Server.GinMode == "release"
}

// ValidateWithContext validates configuration and runs production checks if appropriate
func (c *Config) ValidateWithContext() error {
	// Always run basic validation
	if err := c.Validate(); err != nil {
		return err
	}

	// Run production validation if in production mode
	if c.IsProduction() {
		if err := c.ValidateProduction(); err != nil {
			return fmt.Errorf("production validation failed: %w", err)
		}
	}

	return nil
}
