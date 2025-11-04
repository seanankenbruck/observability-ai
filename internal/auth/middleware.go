// internal/auth/middleware.go
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware for authentication
func (am *AuthManager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip authentication for certain paths
		if shouldSkipAuth(path) {
			c.Next()
			return
		}

		// Check rate limiting
		clientID := getClientID(c)
		if !CheckRateLimit(clientID, am.config.RateLimit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Try to authenticate the request
		user, err := am.authenticateRequest(c)
		if err != nil {
			// Check if endpoint allows anonymous access
			if am.config.AllowAnonymous && isPublicEndpoint(path) {
				c.Next()
				return
			}

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("roles", user.Roles)

		c.Next()
	}
}

// RequireRole returns a middleware that checks if user has required role
func (am *AuthManager) RequireRole(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := GetCurrentUser(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, required := range requiredRoles {
			for _, userRole := range user.Roles {
				if userRole == required {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// authenticateRequest tries multiple authentication methods
func (am *AuthManager) authenticateRequest(c *gin.Context) (*User, error) {
	// Try JWT authentication
	if user, err := am.authenticateJWT(c); err == nil {
		return user, nil
	}

	// Try API key authentication
	if user, err := am.authenticateAPIKey(c); err == nil {
		return user, nil
	}

	// Try session authentication
	if user, err := am.authenticateSession(c); err == nil {
		return user, nil
	}

	return nil, http.ErrAbortHandler
}

// authenticateJWT authenticates using JWT token
func (am *AuthManager) authenticateJWT(c *gin.Context) (*User, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, http.ErrAbortHandler
	}

	// Extract Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, http.ErrAbortHandler
	}

	tokenString := parts[1]
	claims, err := am.ValidateJWTToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Get user from claims
	user, err := am.GetUser(claims.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// authenticateAPIKey authenticates using API key
func (am *AuthManager) authenticateAPIKey(c *gin.Context) (*User, error) {
	// Try X-API-Key header
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		// Try query parameter
		apiKey = c.Query("api_key")
	}

	if apiKey == "" {
		return nil, http.ErrAbortHandler
	}

	user, _, err := am.ValidateAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// authenticateSession authenticates using session cookie
func (am *AuthManager) authenticateSession(c *gin.Context) (*User, error) {
	sessionID, err := c.Cookie("session_id")
	if err != nil {
		return nil, err
	}

	user, _, err := am.ValidateSession(sessionID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Helper functions

// shouldSkipAuth checks if a path should skip authentication
func shouldSkipAuth(path string) bool {
	skipPaths := []string{
		"/health",
		"/api/v1/health",
		"/api/v1/auth/login",
		"/api/v1/auth/status",
		"/static/",
		"/favicon.ico",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// isPublicEndpoint checks if an endpoint allows anonymous access
func isPublicEndpoint(path string) bool {
	publicEndpoints := []string{
		"/api/v1/services",
		"/api/v1/metrics",
	}

	for _, publicPath := range publicEndpoints {
		if path == publicPath {
			return true
		}
	}

	return false
}

// getClientID gets a unique identifier for rate limiting
func getClientID(c *gin.Context) string {
	// Try to get user ID if authenticated
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			return "user:" + id
		}
	}

	// Try to get API key
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return "key:" + apiKey[:8] // Use first 8 chars
	}

	// Fall back to IP address
	return "ip:" + c.ClientIP()
}

// GetCurrentUser returns the current authenticated user from context
func GetCurrentUser(c *gin.Context) (*User, bool) {
	value, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	user, ok := value.(*User)
	return user, ok
}

// GetCurrentUserID returns the current user ID from context
func GetCurrentUserID(c *gin.Context) (string, bool) {
	value, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	userID, ok := value.(string)
	return userID, ok
}
