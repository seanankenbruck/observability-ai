// internal/auth/testing_helper.go
package auth

import (
	"log"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/seanankenbruck/observability-ai/internal/session"
)

// NewTestAuthManager creates an auth manager for testing with an in-memory mock Redis
func NewTestAuthManager(config AuthConfig) *AuthManager {
	// Create miniredis (in-memory Redis mock)
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatalf("Failed to start miniredis: %v", err)
	}

	// Create Redis client pointing to miniredis
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Set default session expiry if not provided
	if config.SessionExpiry == 0 {
		config.SessionExpiry = 7 * 24 * time.Hour
	}

	// Create session manager
	sessionManager := session.NewManager(rdb, config.SessionExpiry)

	// Create and return auth manager
	return NewAuthManager(config, sessionManager)
}
