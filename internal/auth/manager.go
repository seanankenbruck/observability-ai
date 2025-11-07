// internal/auth/manager.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID           string            `json:"id"`
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	PasswordHash string            `json:"-"` // Never expose password hash in JSON
	Roles        []string          `json:"roles"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Active       bool              `json:"active"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key,omitempty"` // Plaintext (only shown once)
	HashedKey   string    `json:"-"`             // Stored hash
	UserID      string    `json:"user_id"`
	Permissions []string  `json:"permissions"`
	RateLimit   int       `json:"rate_limit"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	Active      bool      `json:"active"`
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeen  time.Time `json:"last_seen"`
	Active    bool      `json:"active"`
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret      string
	JWTExpiry      time.Duration
	SessionExpiry  time.Duration
	RateLimit      int
	AllowAnonymous bool
}

// AuthManager handles authentication and user management
type AuthManager struct {
	config   AuthConfig
	users    map[string]*User          // userID -> User
	apiKeys  map[string]*APIKey        // hashedKey -> APIKey
	sessions map[string]*Session       // sessionID -> Session
	userByUsername map[string]*User   // username -> User
	mu       sync.RWMutex
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(config AuthConfig) *AuthManager {
	// Set defaults
	if config.JWTExpiry == 0 {
		config.JWTExpiry = 24 * time.Hour
	}
	if config.SessionExpiry == 0 {
		config.SessionExpiry = 7 * 24 * time.Hour
	}
	if config.RateLimit == 0 {
		config.RateLimit = 100
	}
	if config.JWTSecret == "" {
		config.JWTSecret = generateRandomString(32)
	}

	am := &AuthManager{
		config:         config,
		users:          make(map[string]*User),
		apiKeys:        make(map[string]*APIKey),
		sessions:       make(map[string]*Session),
		userByUsername: make(map[string]*User),
	}

	// Create default admin user
	adminUser, _ := am.CreateUser("admin", "admin@example.com", []string{"admin", "user"})
	if adminUser != nil {
		fmt.Printf("Created default admin user (ID: %s)\n", adminUser.ID)
	}

	return am
}

// CreateUser creates a new user (without password - used for admin creation)
func (am *AuthManager) CreateUser(username, email string, roles []string) (*User, error) {
	return am.CreateUserWithPassword(username, email, "", roles)
}

// CreateUserWithPassword creates a new user with a password
func (am *AuthManager) CreateUserWithPassword(username, email, password string, roles []string) (*User, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if user already exists
	if _, exists := am.userByUsername[username]; exists {
		return nil, fmt.Errorf("user already exists: %s", username)
	}

	// Hash password if provided
	var passwordHash string
	if password != "" {
		hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		passwordHash = string(hashedBytes)
	}

	user := &User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Roles:        roles,
		Metadata:     make(map[string]string),
		Active:       true,
	}

	am.users[user.ID] = user
	am.userByUsername[username] = user

	return user, nil
}

// ValidatePassword checks if the provided password matches the user's password hash
func (am *AuthManager) ValidatePassword(user *User, password string) bool {
	if user.PasswordHash == "" {
		// No password set - for backward compatibility with admin user
		return true
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// GetUser retrieves a user by ID
func (am *AuthManager) GetUser(userID string) (*User, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	return user, nil
}

// GetUserByUsername retrieves a user by username
func (am *AuthManager) GetUserByUsername(username string) (*User, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	user, exists := am.userByUsername[username]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return user, nil
}

// CreateAPIKey creates a new API key for a user
func (am *AuthManager) CreateAPIKey(userID, name string, permissions []string, rateLimit int, expiresIn time.Duration) (*APIKey, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Verify user exists
	if _, exists := am.users[userID]; !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	// Generate API key
	key := generateAPIKey()
	hashedKey := hashAPIKey(key)

	apiKey := &APIKey{
		ID:          uuid.New().String(),
		Name:        name,
		Key:         key, // Store plaintext for initial response
		HashedKey:   hashedKey,
		UserID:      userID,
		Permissions: permissions,
		RateLimit:   rateLimit,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(expiresIn),
		Active:      true,
	}

	am.apiKeys[hashedKey] = apiKey

	return apiKey, nil
}

// ValidateAPIKey validates an API key and returns the associated user
func (am *AuthManager) ValidateAPIKey(key string) (*User, *APIKey, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	hashedKey := hashAPIKey(key)
	apiKey, exists := am.apiKeys[hashedKey]
	if !exists {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	if !apiKey.Active {
		return nil, nil, fmt.Errorf("API key is inactive")
	}

	if time.Now().After(apiKey.ExpiresAt) {
		return nil, nil, fmt.Errorf("API key has expired")
	}

	// Get associated user
	user, exists := am.users[apiKey.UserID]
	if !exists {
		return nil, nil, fmt.Errorf("user not found for API key")
	}

	if !user.Active {
		return nil, nil, fmt.Errorf("user is inactive")
	}

	// Update last used timestamp
	apiKey.LastUsedAt = time.Now()

	return user, apiKey, nil
}

// CreateJWTToken creates a JWT token for a user
func (am *AuthManager) CreateJWTToken(user *User) (string, error) {
	expiresAt := time.Now().Add(am.config.JWTExpiry)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Roles:    user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "observability-ai",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(am.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateJWTToken validates a JWT token and returns the claims
func (am *AuthManager) ValidateJWTToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Verify user still exists and is active
	am.mu.RLock()
	user, exists := am.users[claims.UserID]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	if !user.Active {
		return nil, fmt.Errorf("user is inactive")
	}

	return claims, nil
}

// CreateSession creates a new session for a user
func (am *AuthManager) CreateSession(userID string) (*Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Verify user exists
	if _, exists := am.users[userID]; !exists {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	session := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(am.config.SessionExpiry),
		LastSeen:  time.Now(),
		Active:    true,
	}

	am.sessions[session.ID] = session

	return session, nil
}

// ValidateSession validates a session and returns the associated user
func (am *AuthManager) ValidateSession(sessionID string) (*User, *Session, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	session, exists := am.sessions[sessionID]
	if !exists {
		return nil, nil, fmt.Errorf("session not found")
	}

	if !session.Active {
		return nil, nil, fmt.Errorf("session is inactive")
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, nil, fmt.Errorf("session has expired")
	}

	// Get associated user
	user, exists := am.users[session.UserID]
	if !exists {
		return nil, nil, fmt.Errorf("user not found for session")
	}

	if !user.Active {
		return nil, nil, fmt.Errorf("user is inactive")
	}

	// Update last seen timestamp
	session.LastSeen = time.Now()

	return user, session, nil
}

// RevokeAPIKey revokes an API key
func (am *AuthManager) RevokeAPIKey(keyID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Find the API key by ID
	for _, apiKey := range am.apiKeys {
		if apiKey.ID == keyID {
			apiKey.Active = false
			return nil
		}
	}

	return fmt.Errorf("API key not found: %s", keyID)
}

// RevokeSession revokes a session
func (am *AuthManager) RevokeSession(sessionID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	session, exists := am.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.Active = false
	return nil
}

// CleanupExpired removes expired sessions and API keys
func (am *AuthManager) CleanupExpired() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()

	// Cleanup expired sessions
	for id, session := range am.sessions {
		if now.After(session.ExpiresAt) {
			delete(am.sessions, id)
		}
	}

	// Cleanup expired API keys
	for hash, apiKey := range am.apiKeys {
		if now.After(apiKey.ExpiresAt) {
			delete(am.apiKeys, hash)
		}
	}
}

// ListAPIKeys returns all API keys for a user
func (am *AuthManager) ListAPIKeys(userID string) ([]*APIKey, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var keys []*APIKey
	for _, apiKey := range am.apiKeys {
		if apiKey.UserID == userID {
			// Create a copy without the plaintext key
			keyCopy := *apiKey
			keyCopy.Key = "" // Don't expose the actual key
			keys = append(keys, &keyCopy)
		}
	}

	return keys, nil
}

// ListUsers returns all users (admin only)
func (am *AuthManager) ListUsers() []*User {
	am.mu.RLock()
	defer am.mu.RUnlock()

	users := make([]*User, 0, len(am.users))
	for _, user := range am.users {
		users = append(users, user)
	}

	return users
}

// Helper functions

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

// generateAPIKey generates a new API key with "oai_" prefix
func generateAPIKey() string {
	return "oai_" + generateRandomString(32)
}

// hashAPIKey hashes an API key using SHA256
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
