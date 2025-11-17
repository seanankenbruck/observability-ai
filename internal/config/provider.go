package config

import (
	"context"
	"fmt"
)

// SecretProvider defines the interface for retrieving secrets from various sources
type SecretProvider interface {
	// GetSecret retrieves a secret value by key
	GetSecret(ctx context.Context, key string) (string, error)

	// Name returns the provider name for logging/debugging
	Name() string

	// IsAvailable checks if this provider is available/configured
	IsAvailable(ctx context.Context) bool
}

// ChainProvider chains multiple providers with fallback logic
type ChainProvider struct {
	providers []SecretProvider
}

// NewChainProvider creates a new chain provider with the given providers
// Providers are tried in order until one succeeds
func NewChainProvider(providers ...SecretProvider) *ChainProvider {
	return &ChainProvider{
		providers: providers,
	}
}

// GetSecret tries each provider in order until one succeeds
func (c *ChainProvider) GetSecret(ctx context.Context, key string) (string, error) {
	var lastErr error

	for _, provider := range c.providers {
		if !provider.IsAvailable(ctx) {
			continue
		}

		value, err := provider.GetSecret(ctx, key)
		if err == nil && value != "" {
			return value, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return "", fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return "", fmt.Errorf("no available provider found for key: %s", key)
}

// Name returns the chain provider name
func (c *ChainProvider) Name() string {
	return "chain"
}

// IsAvailable checks if any provider in the chain is available
func (c *ChainProvider) IsAvailable(ctx context.Context) bool {
	for _, provider := range c.providers {
		if provider.IsAvailable(ctx) {
			return true
		}
	}
	return false
}
