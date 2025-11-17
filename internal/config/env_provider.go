package config

import (
	"context"
	"os"
)

// EnvProvider retrieves secrets from environment variables
type EnvProvider struct{}

// NewEnvProvider creates a new environment variable provider
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// GetSecret retrieves a secret from environment variables
func (e *EnvProvider) GetSecret(ctx context.Context, key string) (string, error) {
	return os.Getenv(key), nil
}

// Name returns the provider name
func (e *EnvProvider) Name() string {
	return "env"
}

// IsAvailable always returns true as env vars are always available
func (e *EnvProvider) IsAvailable(ctx context.Context) bool {
	return true
}
