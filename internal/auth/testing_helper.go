// internal/auth/testing_helper.go
package auth

import (
	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/session"
)

// NewTestAuthManager creates an auth manager for testing with a mock Redis client
func NewTestAuthManager(config AuthConfig) *AuthManager {
	// Create a mock Redis client (uses miniredis or real Redis for integration tests)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests to avoid conflicts
	})

	// Create session manager
	sessionManager := session.NewManager(rdb, config.SessionExpiry)

	// Create and return auth manager
	return NewAuthManager(config, sessionManager)
}
