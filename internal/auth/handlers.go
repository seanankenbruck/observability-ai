// internal/auth/handlers.go
package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthHandlers provides HTTP handlers for authentication endpoints
type AuthHandlers struct {
	authManager *AuthManager
}

// NewAuthHandlers creates new auth handlers
func NewAuthHandlers(authManager *AuthManager) *AuthHandlers {
	return &AuthHandlers{
		authManager: authManager,
	}
}

// SetupRoutes sets up authentication routes
func (ah *AuthHandlers) SetupRoutes(r *gin.RouterGroup) {
	// Auth endpoints
	r.POST("/auth/login", ah.Login)
	r.POST("/auth/logout", ah.Logout)
	r.GET("/auth/me", ah.authManager.Middleware(), ah.GetCurrentUser)
	r.GET("/auth/status", ah.GetAuthStatus)

	// API key endpoints (require authentication)
	r.GET("/api-keys", ah.authManager.Middleware(), ah.ListAPIKeys)
	r.POST("/api-keys", ah.authManager.Middleware(), ah.CreateAPIKey)
	r.DELETE("/api-keys/:id", ah.authManager.Middleware(), ah.RevokeAPIKey)

	// Admin endpoints (require admin role)
	admin := r.Group("/admin")
	admin.Use(ah.authManager.Middleware(), ah.authManager.RequireRole("admin"))
	{
		admin.GET("/users", ah.ListUsers)
		admin.POST("/users", ah.CreateUser)
		admin.GET("/rate-limit-stats", ah.GetRateLimitStats)
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	User      *User  `json:"user"`
}

// Login handles user login
func (ah *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For development: accept any password for admin user
	// In production, you'd validate against a password hash
	user, err := ah.authManager.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Create JWT token
	token, err := ah.authManager.CreateJWTToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	// Create session
	session, err := ah.authManager.CreateSession(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	// Set session cookie
	c.SetCookie(
		"session_id",
		session.ID,
		int(ah.authManager.config.SessionExpiry.Seconds()),
		"/",
		"",
		false, // secure (set to true in production with HTTPS)
		true,  // httpOnly
	)

	// Return response
	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(ah.authManager.config.JWTExpiry).Format(time.RFC3339),
		User:      user,
	})
}

// Logout handles user logout
func (ah *AuthHandlers) Logout(c *gin.Context) {
	// Get session ID from cookie
	sessionID, err := c.Cookie("session_id")
	if err == nil {
		// Revoke session
		ah.authManager.RevokeSession(sessionID)
	}

	// Clear cookie
	c.SetCookie("session_id", "", -1, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// GetCurrentUser returns the current authenticated user
func (ah *AuthHandlers) GetCurrentUser(c *gin.Context) {
	user, exists := GetCurrentUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetAuthStatus returns authentication status and configuration
func (ah *AuthHandlers) GetAuthStatus(c *gin.Context) {
	status := gin.H{
		"authentication_enabled": true,
		"allow_anonymous":        ah.authManager.config.AllowAnonymous,
		"rate_limit":             ah.authManager.config.RateLimit,
		"jwt_expiry":             ah.authManager.config.JWTExpiry.String(),
		"session_expiry":         ah.authManager.config.SessionExpiry.String(),
	}

	// Check if user is authenticated
	if user, exists := GetCurrentUser(c); exists {
		status["authenticated"] = true
		status["user"] = user
	} else {
		status["authenticated"] = false
	}

	c.JSON(http.StatusOK, status)
}

// CreateAPIKeyRequest represents a request to create an API key
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions"`
	RateLimit   int      `json:"rate_limit"`
	ExpiresIn   string   `json:"expires_in"` // e.g., "30d", "1y", "720h"
}

// CreateAPIKeyResponse represents the response with a new API key
type CreateAPIKeyResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"` // Only shown once!
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAPIKey creates a new API key for the current user
func (ah *AuthHandlers) CreateAPIKey(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	// Parse expiry duration
	expiresIn, err := parseDuration(req.ExpiresIn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expiry duration"})
		return
	}

	// Set default rate limit if not provided
	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = ah.authManager.config.RateLimit
	}

	// Create API key
	apiKey, err := ah.authManager.CreateAPIKey(
		userID,
		req.Name,
		req.Permissions,
		rateLimit,
		expiresIn,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the key (only time it's shown in plaintext!)
	c.JSON(http.StatusCreated, CreateAPIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       apiKey.Key, // Important: only shown once!
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	})
}

// ListAPIKeys returns all API keys for the current user
func (ah *AuthHandlers) ListAPIKeys(c *gin.Context) {
	userID, exists := GetCurrentUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}

	keys, err := ah.authManager.ListAPIKeys(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

// RevokeAPIKey revokes an API key
func (ah *AuthHandlers) RevokeAPIKey(c *gin.Context) {
	keyID := c.Param("id")

	err := ah.authManager.RevokeAPIKey(keyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// CreateUserRequest represents a request to create a user
type CreateUserRequest struct {
	Username string   `json:"username" binding:"required"`
	Email    string   `json:"email" binding:"required"`
	Roles    []string `json:"roles"`
}

// CreateUser creates a new user (admin only)
func (ah *AuthHandlers) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default role if not provided
	if len(req.Roles) == 0 {
		req.Roles = []string{"user"}
	}

	user, err := ah.authManager.CreateUser(req.Username, req.Email, req.Roles)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// ListUsers returns all users (admin only)
func (ah *AuthHandlers) ListUsers(c *gin.Context) {
	users := ah.authManager.ListUsers()
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetRateLimitStats returns rate limiting statistics (admin only)
func (ah *AuthHandlers) GetRateLimitStats(c *gin.Context) {
	stats := GetRateLimitStats()
	c.JSON(http.StatusOK, stats)
}

// Helper functions

// parseDuration parses duration strings like "30d", "1y", "720h"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 30 * 24 * time.Hour, nil // Default: 30 days
	}

	// Handle special cases
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "w") {
		weeks, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "y") {
		years, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err != nil {
			return 0, err
		}
		return time.Duration(years) * 365 * 24 * time.Hour, nil
	}

	// Use standard time.ParseDuration for other formats (e.g., "720h", "48h")
	return time.ParseDuration(s)
}
