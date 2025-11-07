// internal/auth/handlers.go
package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/seanankenbruck/observability-ai/internal/errors"
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
	r.POST("/auth/register", ah.Register)
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

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// Register handles user registration
func (ah *AuthHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		enhancedErr := errors.NewInvalidInputError("request body", err.Error())
		c.JSON(http.StatusBadRequest, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Create user with password
	user, err := ah.authManager.CreateUserWithPassword(req.Username, req.Email, req.Password, []string{"user"})
	if err != nil {
		enhancedErr := errors.Wrap(err, errors.ErrCodeInvalidInput, "Failed to register user").
			WithDetails("A user with this username or email may already exist").
			WithSuggestion("Choose a different username or email address.").
			WithMetadata("username", req.Username)
		c.JSON(http.StatusConflict, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Create JWT token
	token, err := ah.authManager.CreateJWTToken(user)
	if err != nil {
		enhancedErr := errors.NewTokenCreationError(err)
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Create session
	session, err := ah.authManager.CreateSession(user.ID)
	if err != nil {
		enhancedErr := errors.NewSessionCreationError(err)
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
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
	c.JSON(http.StatusCreated, LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(ah.authManager.config.JWTExpiry).Format(time.RFC3339),
		User:      user,
	})
}

// Login handles user login
func (ah *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		enhancedErr := errors.NewInvalidInputError("request body", err.Error())
		c.JSON(http.StatusBadRequest, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Get user by username
	user, err := ah.authManager.GetUserByUsername(req.Username)
	if err != nil {
		enhancedErr := errors.NewInvalidCredentialsError()
		c.JSON(http.StatusUnauthorized, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Validate password
	if !ah.authManager.ValidatePassword(user, req.Password) {
		enhancedErr := errors.NewInvalidCredentialsError()
		c.JSON(http.StatusUnauthorized, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Create JWT token
	token, err := ah.authManager.CreateJWTToken(user)
	if err != nil {
		enhancedErr := errors.NewTokenCreationError(err)
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Create session
	session, err := ah.authManager.CreateSession(user.ID)
	if err != nil {
		enhancedErr := errors.NewSessionCreationError(err)
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
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
		enhancedErr := errors.NewNotAuthenticatedError()
		c.JSON(http.StatusUnauthorized, formatAuthErrorResponse(enhancedErr))
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
		enhancedErr := errors.NewInvalidInputError("request body", err.Error())
		c.JSON(http.StatusBadRequest, formatAuthErrorResponse(enhancedErr))
		return
	}

	userID, exists := GetCurrentUserID(c)
	if !exists {
		enhancedErr := errors.NewNotAuthenticatedError()
		c.JSON(http.StatusUnauthorized, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Parse expiry duration
	expiresIn, err := parseDuration(req.ExpiresIn)
	if err != nil {
		enhancedErr := errors.New(errors.ErrCodeInvalidDuration, "Invalid expiry duration format").
			WithDetails("The expires_in field must be in format: '30d', '1y', '720h', etc.").
			WithSuggestion("Use formats like '30d' for 30 days, '1y' for 1 year, or '720h' for 720 hours.")
		c.JSON(http.StatusBadRequest, formatAuthErrorResponse(enhancedErr))
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
		enhancedErr := errors.Wrap(err, errors.ErrCodeInvalidInput, "Failed to create API key").
			WithDetails("Unable to create the API key with the provided parameters").
			WithSuggestion("Ensure the API key name is unique and all parameters are valid.")
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
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
		enhancedErr := errors.NewNotAuthenticatedError()
		c.JSON(http.StatusUnauthorized, formatAuthErrorResponse(enhancedErr))
		return
	}

	keys, err := ah.authManager.ListAPIKeys(userID)
	if err != nil {
		enhancedErr := errors.Wrap(err, errors.ErrCodeDatabaseQuery, "Failed to retrieve API keys").
			WithDetails("Unable to fetch the list of API keys").
			WithSuggestion("This is an internal error. Please try again.")
		c.JSON(http.StatusInternalServerError, formatAuthErrorResponse(enhancedErr))
		return
	}

	c.JSON(http.StatusOK, gin.H{"api_keys": keys})
}

// RevokeAPIKey revokes an API key
func (ah *AuthHandlers) RevokeAPIKey(c *gin.Context) {
	keyID := c.Param("id")

	err := ah.authManager.RevokeAPIKey(keyID)
	if err != nil {
		enhancedErr := errors.New(errors.ErrCodeInvalidInput, "Failed to revoke API key").
			WithDetails("The specified API key could not be found or has already been revoked").
			WithSuggestion("Verify the API key ID is correct using the /api/v1/api-keys endpoint.").
			WithMetadata("key_id", keyID)
		c.JSON(http.StatusNotFound, formatAuthErrorResponse(enhancedErr))
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
		enhancedErr := errors.NewInvalidInputError("request body", err.Error())
		c.JSON(http.StatusBadRequest, formatAuthErrorResponse(enhancedErr))
		return
	}

	// Set default role if not provided
	if len(req.Roles) == 0 {
		req.Roles = []string{"user"}
	}

	user, err := ah.authManager.CreateUser(req.Username, req.Email, req.Roles)
	if err != nil {
		enhancedErr := errors.Wrap(err, errors.ErrCodeInvalidInput, "Failed to create user").
			WithDetails("A user with this username or email may already exist").
			WithSuggestion("Choose a different username or email address.").
			WithMetadata("username", req.Username)
		c.JSON(http.StatusConflict, formatAuthErrorResponse(enhancedErr))
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

// formatAuthErrorResponse formats an error into a user-friendly response
func formatAuthErrorResponse(err error) gin.H {
	// Check if it's an EnhancedError
	if enhancedErr, ok := err.(*errors.EnhancedError); ok {
		response := gin.H{
			"error": gin.H{
				"code":    enhancedErr.Code,
				"message": enhancedErr.Message,
			},
		}

		if enhancedErr.Details != "" {
			response["error"].(gin.H)["details"] = enhancedErr.Details
		}

		if enhancedErr.Suggestion != "" {
			response["error"].(gin.H)["suggestion"] = enhancedErr.Suggestion
		}

		if enhancedErr.Documentation != "" {
			response["error"].(gin.H)["documentation"] = enhancedErr.Documentation
		}

		if len(enhancedErr.Metadata) > 0 {
			response["error"].(gin.H)["metadata"] = enhancedErr.Metadata
		}

		return response
	}

	// Fallback for regular errors
	return gin.H{
		"error": gin.H{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		},
	}
}
