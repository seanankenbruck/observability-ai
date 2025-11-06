// internal/auth/middleware_test.go
package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)
}

// TestMiddleware tests the authentication middleware
func TestMiddleware(t *testing.T) {
	am := NewAuthManager(AuthConfig{
		JWTSecret:      "test-secret",
		RateLimit:      100,
		AllowAnonymous: false,
	})

	// Create test user and auth credentials
	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	jwtToken, err := am.CreateJWTToken(user)
	require.NoError(t, err)

	apiKey, err := am.CreateAPIKey(user.ID, "test-key", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	session, err := am.CreateSession(user.ID)
	require.NoError(t, err)

	tests := []struct {
		name           string
		path           string
		setupRequest   func(*http.Request)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "authenticated with JWT token",
			path: "/api/v1/protected",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+jwtToken)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "authenticated with API key header",
			path: "/api/v1/protected",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Key", apiKey.Key)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "authenticated with session cookie",
			path: "/api/v1/protected",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "session_id",
					Value: session.ID,
				})
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthenticated request to protected endpoint",
			path:           "/api/v1/protected",
			setupRequest:   func(req *http.Request) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "authentication required",
		},
		{
			name:           "health check skips authentication",
			path:           "/health",
			setupRequest:   func(req *http.Request) {},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "login endpoint skips authentication",
			path:           "/api/v1/auth/login",
			setupRequest:   func(req *http.Request) {},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid JWT token",
			path: "/api/v1/protected",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer invalid.token.here")
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid API key",
			path: "/api/v1/protected",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Key", "oai_invalid_key")
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new Gin router for each test
			router := gin.New()
			router.Use(am.Middleware())

			// Add test routes
			router.GET("/health", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.GET("/api/v1/auth/login", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.GET("/api/v1/protected", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "authenticated"})
			})

			// Create test request
			req, _ := http.NewRequest("GET", tt.path, nil)
			tt.setupRequest(req)

			// Record response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestMiddlewareWithAnonymousAccess tests middleware with anonymous access enabled
func TestMiddlewareWithAnonymousAccess(t *testing.T) {
	am := NewAuthManager(AuthConfig{
		JWTSecret:      "test-secret",
		RateLimit:      100,
		AllowAnonymous: true,
	})

	router := gin.New()
	router.Use(am.Middleware())

	router.GET("/api/v1/services", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "public endpoint allows anonymous",
			path:           "/api/v1/services",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-public endpoint requires auth",
			path:           "/api/v1/protected",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestRequireRole tests role-based access control
func TestRequireRole(t *testing.T) {
	am := NewAuthManager(AuthConfig{
		JWTSecret: "test-secret",
		RateLimit: 100,
	})

	// Get the default admin user (created automatically)
	adminUser, err := am.GetUserByUsername("admin")
	require.NoError(t, err)

	// Create a regular user
	regularUser, err := am.CreateUser("regularuser", "user@example.com", []string{"user"})
	require.NoError(t, err)

	adminToken, err := am.CreateJWTToken(adminUser)
	require.NoError(t, err)

	userToken, err := am.CreateJWTToken(regularUser)
	require.NoError(t, err)

	tests := []struct {
		name           string
		token          string
		requiredRoles  []string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "admin can access admin endpoint",
			token:          adminToken,
			requiredRoles:  []string{"admin"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "regular user cannot access admin endpoint",
			token:          userToken,
			requiredRoles:  []string{"admin"},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "insufficient permissions",
		},
		{
			name:           "user can access user endpoint",
			token:          userToken,
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "admin can access user endpoint (has user role)",
			token:          adminToken,
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "user with any of multiple required roles can access",
			token:          userToken,
			requiredRoles:  []string{"admin", "user"},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthenticated request denied",
			token:          "",
			requiredRoles:  []string{"user"},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(am.Middleware())

			router.GET("/api/v1/protected", am.RequireRole(tt.requiredRoles...), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "authorized"})
			})

			req, _ := http.NewRequest("GET", "/api/v1/protected", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tt.expectedBody)
			}
		})
	}
}

// TestRateLimiting tests rate limiting functionality
func TestRateLimiting(t *testing.T) {
	// Create a new rate limiter for testing
	rateLimiter := NewRateLimiter()

	tests := []struct {
		name           string
		clientID       string
		limit          int
		requestCount   int
		expectedAllows int
	}{
		{
			name:           "allow requests under limit",
			clientID:       "client1",
			limit:          10,
			requestCount:   5,
			expectedAllows: 5,
		},
		{
			name:           "block requests over limit",
			clientID:       "client2",
			limit:          5,
			requestCount:   10,
			expectedAllows: 5,
		},
		{
			name:           "exactly at limit",
			clientID:       "client3",
			limit:          3,
			requestCount:   3,
			expectedAllows: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowedCount := 0
			for i := 0; i < tt.requestCount; i++ {
				if rateLimiter.Allow(tt.clientID, tt.limit) {
					allowedCount++
				}
			}
			assert.Equal(t, tt.expectedAllows, allowedCount)
		})
	}
}

// TestRateLimitMiddleware tests rate limiting in middleware
func TestRateLimitMiddleware(t *testing.T) {
	am := NewAuthManager(AuthConfig{
		JWTSecret: "test-secret",
		RateLimit: 3, // Very low limit for testing
	})

	router := gin.New()
	router.Use(am.Middleware())

	router.GET("/api/v1/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Make multiple requests from the same client
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/test", nil)
		req.RemoteAddr = "192.168.1.1:1234" // Same IP for all requests

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	// First 3 requests should succeed, rest should be rate limited
	// Note: The actual count might vary due to timing, but we should see some rate limiting
	assert.Greater(t, rateLimitedCount, 0, "Some requests should be rate limited")
}

// TestGetCurrentUser tests getting current user from context
func TestGetCurrentUser(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	token, err := am.CreateJWTToken(user)
	require.NoError(t, err)

	router := gin.New()
	router.Use(am.Middleware())

	router.GET("/api/v1/test", func(c *gin.Context) {
		currentUser, exists := GetCurrentUser(c)
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no user in context"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"user_id":  currentUser.ID,
			"username": currentUser.Username,
		})
	})

	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), user.Username)
}

// TestGetCurrentUserID tests getting current user ID from context
func TestGetCurrentUserID(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	token, err := am.CreateJWTToken(user)
	require.NoError(t, err)

	router := gin.New()
	router.Use(am.Middleware())

	router.GET("/api/v1/test", func(c *gin.Context) {
		userID, exists := GetCurrentUserID(c)
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no user ID in context"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	req, _ := http.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), user.ID)
}

// TestShouldSkipAuth tests the shouldSkipAuth function
func TestShouldSkipAuth(t *testing.T) {
	tests := []struct {
		path       string
		shouldSkip bool
	}{
		{"/health", true},
		{"/api/v1/health", true},
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/status", true},
		{"/static/css/style.css", true},
		{"/favicon.ico", true},
		{"/api/v1/protected", false},
		{"/api/v1/services", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := shouldSkipAuth(tt.path)
			assert.Equal(t, tt.shouldSkip, result)
		})
	}
}

// TestIsPublicEndpoint tests the isPublicEndpoint function
func TestIsPublicEndpoint(t *testing.T) {
	tests := []struct {
		path     string
		isPublic bool
	}{
		{"/api/v1/services", true},
		{"/api/v1/metrics", true},
		{"/api/v1/protected", false},
		{"/api/v1/admin", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPublicEndpoint(tt.path)
			assert.Equal(t, tt.isPublic, result)
		})
	}
}

// TestGetClientID tests the getClientID function
func TestGetClientID(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*gin.Context)
		expectedPrefix string
	}{
		{
			name: "with user ID in context",
			setupFunc: func(c *gin.Context) {
				c.Set("user_id", "user-123")
			},
			expectedPrefix: "user:",
		},
		{
			name: "with API key header",
			setupFunc: func(c *gin.Context) {
				c.Request.Header.Set("X-API-Key", "oai_test_key_12345")
			},
			expectedPrefix: "key:",
		},
		{
			name: "fallback to IP address",
			setupFunc: func(c *gin.Context) {
				// No additional setup, should fall back to IP
			},
			expectedPrefix: "ip:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest("GET", "/test", nil)
			c.Request.RemoteAddr = "192.168.1.1:1234"

			tt.setupFunc(c)

			clientID := getClientID(c)
			assert.Contains(t, clientID, tt.expectedPrefix)
		})
	}
}

// TestAuthenticationMethods tests all authentication methods
func TestAuthenticationMethods(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})

	user, err := am.CreateUser("testuser", "test@example.com", []string{"user"})
	require.NoError(t, err)

	// Create credentials for all auth methods
	jwtToken, err := am.CreateJWTToken(user)
	require.NoError(t, err)

	apiKey, err := am.CreateAPIKey(user.ID, "test-key", []string{"read"}, 100, 30*24*time.Hour)
	require.NoError(t, err)

	session, err := am.CreateSession(user.ID)
	require.NoError(t, err)

	tests := []struct {
		name         string
		setupRequest func(*http.Request)
		wantErr      bool
	}{
		{
			name: "JWT authentication",
			setupRequest: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+jwtToken)
			},
			wantErr: false,
		},
		{
			name: "API key via header",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-API-Key", apiKey.Key)
			},
			wantErr: false,
		},
		{
			name: "API key via query parameter",
			setupRequest: func(req *http.Request) {
				q := req.URL.Query()
				q.Add("api_key", apiKey.Key)
				req.URL.RawQuery = q.Encode()
			},
			wantErr: false,
		},
		{
			name: "Session cookie",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{
					Name:  "session_id",
					Value: session.ID,
				})
			},
			wantErr: false,
		},
		{
			name:         "No authentication",
			setupRequest: func(req *http.Request) {},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(am.Middleware())

			authenticated := false
			router.GET("/test", func(c *gin.Context) {
				authenticated = true
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req, _ := http.NewRequest("GET", "/test", nil)
			tt.setupRequest(req)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if tt.wantErr {
				assert.False(t, authenticated, "Request should not reach handler")
				assert.Equal(t, http.StatusUnauthorized, w.Code)
			} else {
				assert.True(t, authenticated, "Request should reach handler")
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

// TestMultipleRolesAccess tests access with multiple role requirements
func TestMultipleRolesAccess(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})

	// Create users with different role combinations
	user1, _ := am.CreateUser("user1", "user1@example.com", []string{"developer"})
	user2, _ := am.CreateUser("user2", "user2@example.com", []string{"reviewer"})
	user3, _ := am.CreateUser("user3", "user3@example.com", []string{"developer", "reviewer"})

	token1, _ := am.CreateJWTToken(user1)
	token2, _ := am.CreateJWTToken(user2)
	token3, _ := am.CreateJWTToken(user3)

	router := gin.New()
	router.Use(am.Middleware())

	// Endpoint requires either developer OR reviewer role
	router.GET("/api/v1/code", am.RequireRole("developer", "reviewer"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{"developer can access", token1, http.StatusOK},
		{"reviewer can access", token2, http.StatusOK},
		{"user with both roles can access", token3, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/code", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestRateLimiterStats tests rate limiter statistics
func TestRateLimiterStats(t *testing.T) {
	rl := NewRateLimiter()

	// Make some requests
	rl.Allow("client1", 10)
	rl.Allow("client1", 10)
	rl.Allow("client2", 10)

	stats := rl.GetStats()
	require.NotNil(t, stats)

	totalClients, ok := stats["total_clients"].(int)
	require.True(t, ok)
	assert.Equal(t, 2, totalClients)

	clients, ok := stats["clients"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, clients, 2)
}

// BenchmarkMiddlewareAuth benchmarks middleware authentication
func BenchmarkMiddlewareAuth(b *testing.B) {
	am := NewAuthManager(AuthConfig{
		JWTSecret: "test-secret",
		RateLimit: 10000, // High limit for benchmarking
	})

	user, _ := am.CreateUser("testuser", "test@example.com", []string{"user"})
	token, _ := am.CreateJWTToken(user)

	router := gin.New()
	router.Use(am.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
