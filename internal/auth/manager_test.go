// internal/auth/manager_test.go
package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAuthManager tests creation of auth manager
func TestNewAuthManager(t *testing.T) {
	tests := []struct {
		name           string
		config         AuthConfig
		expectedExpiry time.Duration
	}{
		{
			name: "default configuration",
			config: AuthConfig{
				JWTSecret: "test-secret",
			},
			expectedExpiry: 24 * time.Hour,
		},
		{
			name: "custom configuration",
			config: AuthConfig{
				JWTSecret:     "custom-secret",
				JWTExpiry:     2 * time.Hour,
				SessionExpiry: 48 * time.Hour,
				RateLimit:     200,
			},
			expectedExpiry: 2 * time.Hour,
		},
		{
			name:           "empty configuration uses defaults",
			config:         AuthConfig{},
			expectedExpiry: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewTestAuthManager(tt.config)
			require.NotNil(t, am)
			assert.NotEmpty(t, am.config.JWTSecret)
			assert.Equal(t, tt.expectedExpiry, am.config.JWTExpiry)

			// Verify default admin user was created
			adminUser, err := am.GetUserByUsername("admin")
			require.NoError(t, err)
			assert.Equal(t, "admin", adminUser.Username)
			assert.Contains(t, adminUser.Roles, "admin")
			assert.True(t, adminUser.Active)
		})
	}
}

// TestCreateUser tests user creation
func TestCreateUser(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		email       string
		roles       []string
		wantErr     bool
		errContains string
	}{
		{
			name:     "create regular user",
			username: "testuser",
			email:    "test@example.com",
			roles:    []string{"user"},
			wantErr:  false,
		},
		{
			name:     "create user with multiple roles",
			username: "poweruser",
			email:    "power@example.com",
			roles:    []string{"user", "developer", "reviewer"},
			wantErr:  false,
		},
		{
			name:     "create user with empty roles",
			username: "basicuser",
			email:    "basic@example.com",
			roles:    []string{},
			wantErr:  false,
		},
		{
			name:        "duplicate username fails",
			username:    "admin", // Already exists
			email:       "admin2@example.com",
			roles:       []string{"user"},
			wantErr:     true,
			errContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

			user, err := am.CreateUser(tt.username, tt.email, tt.roles)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.NotEmpty(t, user.ID)
				assert.Equal(t, tt.username, user.Username)
				assert.Equal(t, tt.email, user.Email)
				assert.Equal(t, tt.roles, user.Roles)
				assert.True(t, user.Active)

				// Verify user can be retrieved
				retrievedUser, err := am.GetUser(user.ID)
				require.NoError(t, err)
				assert.Equal(t, user.ID, retrievedUser.ID)

				// Verify user can be retrieved by username
				userByName, err := am.GetUserByUsername(tt.username)
				require.NoError(t, err)
				assert.Equal(t, user.ID, userByName.ID)
			}
		})
	}
}

// TestGetUser tests user retrieval
func TestGetUser(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	// Create test user
	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		userID      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "get existing user",
			userID:  user.ID,
			wantErr: false,
		},
		{
			name:        "get non-existent user",
			userID:      "non-existent-id",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedUser, err := am.GetUser(tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.userID, retrievedUser.ID)
			}
		})
	}
}

// TestCreateAPIKey tests API key creation
func TestCreateAPIKey(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	// Create test user
	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		userID      string
		keyName     string
		permissions []string
		rateLimit   int
		expiresIn   time.Duration
		wantErr     bool
		errContains string
	}{
		{
			name:        "create basic API key",
			userID:      user.ID,
			keyName:     "test-key",
			permissions: []string{"read"},
			rateLimit:   100,
			expiresIn:   30 * 24 * time.Hour,
			wantErr:     false,
		},
		{
			name:        "create API key with multiple permissions",
			userID:      user.ID,
			keyName:     "admin-key",
			permissions: []string{"read", "write", "delete"},
			rateLimit:   1000,
			expiresIn:   90 * 24 * time.Hour,
			wantErr:     false,
		},
		{
			name:        "create API key for non-existent user",
			userID:      "non-existent",
			keyName:     "invalid-key",
			permissions: []string{"read"},
			rateLimit:   100,
			expiresIn:   30 * 24 * time.Hour,
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiKey, err := am.CreateAPIKey(tt.userID, tt.keyName, tt.permissions, tt.rateLimit, tt.expiresIn)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, apiKey)
				assert.NotEmpty(t, apiKey.ID)
				assert.NotEmpty(t, apiKey.Key)
				assert.NotEmpty(t, apiKey.HashedKey)
				assert.Equal(t, tt.keyName, apiKey.Name)
				assert.Equal(t, tt.userID, apiKey.UserID)
				assert.Equal(t, tt.permissions, apiKey.Permissions)
				assert.Equal(t, tt.rateLimit, apiKey.RateLimit)
				assert.True(t, apiKey.Active)
				assert.WithinDuration(t, time.Now().Add(tt.expiresIn), apiKey.ExpiresAt, time.Second)

				// Verify key has "oai_" prefix
				assert.Contains(t, apiKey.Key, "oai_")
			}
		})
	}
}

// TestValidateAPIKey tests API key validation
func TestValidateAPIKey(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	// Create test user and API key
	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	validKey, err := am.CreateAPIKey(user.ID, "test-key", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	// Create expired API key
	expiredKey, err := am.CreateAPIKey(user.ID, "expired-key", []string{"read"}, 100, -1*time.Hour)
	require.NoError(t, err)

	tests := []struct {
		name        string
		key         string
		wantErr     bool
		errContains string
		setupFunc   func()
	}{
		{
			name:    "validate valid API key",
			key:     validKey.Key,
			wantErr: false,
		},
		{
			name:        "validate expired API key",
			key:         expiredKey.Key,
			wantErr:     true,
			errContains: "expired",
		},
		{
			name:        "validate invalid API key",
			key:         "oai_invalid_key_12345",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:    "validate revoked API key",
			key:     validKey.Key,
			wantErr: true,
			setupFunc: func() {
				am.RevokeAPIKey(validKey.ID)
			},
			errContains: "inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			validatedUser, apiKey, err := am.ValidateAPIKey(tt.key)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, validatedUser)
				require.NotNil(t, apiKey)
				assert.Equal(t, user.ID, validatedUser.ID)
				assert.NotZero(t, apiKey.LastUsedAt)
			}
		})
	}
}

// TestCreateJWTToken tests JWT token creation
func TestCreateJWTToken(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{
		JWTSecret: "test-secret",
		JWTExpiry: 1 * time.Hour,
	})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user", "admin"})
	require.NoError(t, err)

	token, err := am.CreateJWTToken(user)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should be a valid JWT format (3 parts separated by dots)
	// We'll validate this in the validation test
}

// TestValidateJWTToken tests JWT token validation
func TestValidateJWTToken(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{
		JWTSecret: "test-secret",
		JWTExpiry: 1 * time.Hour,
	})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user", "admin"})
	require.NoError(t, err)

	validToken, err := am.CreateJWTToken(user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		wantErr     bool
		errContains string
		setupFunc   func()
	}{
		{
			name:    "validate valid token",
			token:   validToken,
			wantErr: false,
		},
		{
			name:        "validate invalid token",
			token:       "invalid.token.here",
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name:    "validate token for inactive user",
			token:   validToken,
			wantErr: true,
			setupFunc: func() {
				user.Active = false
			},
			errContains: "inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			claims, err := am.ValidateJWTToken(tt.token)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, claims)
				assert.Equal(t, user.ID, claims.UserID)
				assert.Equal(t, user.Username, claims.Username)
				assert.Equal(t, user.Roles, claims.Roles)
			}
		})
	}
}

// TestCreateSession tests session creation
func TestCreateSession(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{
		JWTSecret:     "test-secret",
		SessionExpiry: 7 * 24 * time.Hour,
	})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		userID      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "create session for valid user",
			userID:  user.ID,
			wantErr: false,
		},
		{
			name:        "create session for non-existent user",
			userID:      "non-existent",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID, err := am.CreateSession(tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, sessionID)

				// Validate the session was created correctly by retrieving it
				user, err := am.ValidateSession(sessionID)
				require.NoError(t, err)
				assert.Equal(t, tt.userID, user.ID)
			}
		})
	}
}

// TestValidateSession tests session validation
func TestValidateSession(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{
		JWTSecret:     "test-secret",
		SessionExpiry: 7 * 24 * time.Hour,
	})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	validSessionID, err := am.CreateSession(user.ID)
	require.NoError(t, err)

	tests := []struct {
		name        string
		sessionID   string
		wantErr     bool
		errContains string
		setupFunc   func()
	}{
		{
			name:      "validate valid session",
			sessionID: validSessionID,
			wantErr:   false,
		},
		{
			name:        "validate non-existent session",
			sessionID:   "non-existent",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:      "validate revoked session",
			sessionID: validSessionID,
			wantErr:   true,
			setupFunc: func() {
				am.RevokeSession(validSessionID)
			},
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			validatedUser, err := am.ValidateSession(tt.sessionID)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, validatedUser)
				assert.Equal(t, user.ID, validatedUser.ID)
			}
		})
	}
}

// TestRevokeAPIKey tests API key revocation
func TestRevokeAPIKey(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	apiKey, err := am.CreateAPIKey(user.ID, "test-key", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	// Revoke the key
	err = am.RevokeAPIKey(apiKey.ID)
	require.NoError(t, err)

	// Verify key is revoked
	_, _, err = am.ValidateAPIKey(apiKey.Key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inactive")

	// Try to revoke non-existent key
	err = am.RevokeAPIKey("non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRevokeSession tests session revocation
func TestRevokeSession(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	sessionID, err := am.CreateSession(user.ID)
	require.NoError(t, err)

	// Revoke the session
	err = am.RevokeSession(sessionID)
	require.NoError(t, err)

	// Verify session is revoked
	_, err = am.ValidateSession(sessionID)
	require.Error(t, err)

	// Try to revoke non-existent session - this should not error in Redis (delete is idempotent)
	err = am.RevokeSession("non-existent")
	require.NoError(t, err)
}

// TestCleanupExpired tests cleanup of expired sessions and API keys
func TestCleanupExpired(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	// Create expired API key
	expiredKey, err := am.CreateAPIKey(user.ID, "expired-key", []string{"read"}, 100, -1*time.Hour)
	require.NoError(t, err)

	// Create valid API key
	validKey, err := am.CreateAPIKey(user.ID, "valid-key", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	// Note: Sessions are now managed in Redis with TTL auto-expiration
	// We only test API key cleanup here

	// Run cleanup
	am.CleanupExpired()

	// Verify expired API key is removed, valid key remains
	am.mu.RLock()
	_, expiredKeyExists := am.apiKeys[hashAPIKey(expiredKey.Key)]
	_, validKeyExists := am.apiKeys[hashAPIKey(validKey.Key)]
	am.mu.RUnlock()

	assert.False(t, expiredKeyExists, "Expired API key should be removed")
	assert.True(t, validKeyExists, "Valid API key should remain")
}

// TestListAPIKeys tests listing API keys for a user
func TestListAPIKeys(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user1, err := am.CreateUser("user1", "user1@example.com", []string{"user"})
	require.NoError(t, err)

	user2, err := am.CreateUser("user2", "user2@example.com", []string{"user"})
	require.NoError(t, err)

	// Create keys for user1
	_, err = am.CreateAPIKey(user1.ID, "key1", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)
	_, err = am.CreateAPIKey(user1.ID, "key2", []string{"write"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	// Create key for user2
	_, err = am.CreateAPIKey(user2.ID, "key3", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	// List keys for user1
	keys, err := am.ListAPIKeys(user1.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	// Verify plaintext keys are not exposed
	for _, key := range keys {
		assert.Empty(t, key.Key, "Plaintext key should not be exposed in list")
		assert.NotEmpty(t, key.HashedKey)
	}

	// List keys for user2
	keys, err = am.ListAPIKeys(user2.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

// TestListUsers tests listing all users
func TestListUsers(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	// Create additional users (admin already exists)
	_, err := am.CreateUser("user1", "user1@example.com", []string{"user"})
	require.NoError(t, err)
	_, err = am.CreateUser("user2", "user2@example.com", []string{"user"})
	require.NoError(t, err)

	users := am.ListUsers()
	assert.GreaterOrEqual(t, len(users), 3) // At least admin + 2 created users
}

// TestHashAPIKey tests API key hashing consistency
func TestHashAPIKey(t *testing.T) {
	key := "oai_test_key_12345"

	hash1 := hashAPIKey(key)
	hash2 := hashAPIKey(key)

	assert.Equal(t, hash1, hash2, "Hash should be deterministic")
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA256 produces 64 hex characters
}

// TestConcurrentAccess tests concurrent access to auth manager
func TestConcurrentAccess(t *testing.T) {
	am := NewTestAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	// Create multiple API keys concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := am.CreateAPIKey(user.ID, "key-"+string(rune(i)), []string{"read"}, 100, 30*24*time.Hour)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all keys were created
	keys, err := am.ListAPIKeys(user.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 10)
}
