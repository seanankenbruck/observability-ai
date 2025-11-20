package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	sessionPrefix = "session:"
	sessionIDLen  = 32
)

// Session represents user session data
type Session struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Roles     []string  `json:"roles"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Manager handles session storage and retrieval
type Manager struct {
	redis  *redis.Client
	expiry time.Duration
}

// NewManager creates a new session manager
func NewManager(redisClient *redis.Client, expiry time.Duration) *Manager {
	return &Manager{
		redis:  redisClient,
		expiry: expiry,
	}
}

// Create creates a new session and returns the session ID
func (m *Manager) Create(ctx context.Context, userID, username, token string, roles []string) (string, error) {
	// Generate session ID
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Create session data
	session := Session{
		UserID:    userID,
		Username:  username,
		Roles:     roles,
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.expiry),
	}

	// Serialize session
	data, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	// Store in Redis
	key := sessionPrefix + sessionID
	if err := m.redis.Set(ctx, key, data, m.expiry).Err(); err != nil {
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	return sessionID, nil
}

// Get retrieves a session by ID
func (m *Manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	key := sessionPrefix + sessionID
	data, err := m.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		m.Delete(ctx, sessionID)
		return nil, fmt.Errorf("session expired")
	}

	return &session, nil
}

// Delete removes a session
func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	key := sessionPrefix + sessionID
	return m.redis.Del(ctx, key).Err()
}

// Refresh extends the session expiry
func (m *Manager) Refresh(ctx context.Context, sessionID string) error {
	key := sessionPrefix + sessionID
	return m.redis.Expire(ctx, key, m.expiry).Err()
}

// generateSessionID generates a cryptographically secure random session ID
func generateSessionID() (string, error) {
	b := make([]byte, sessionIDLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
