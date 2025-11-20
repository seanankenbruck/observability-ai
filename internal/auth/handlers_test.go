// internal/auth/handlers_test.go
package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/seanankenbruck/observability-ai/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestRouter creates a test router with auth routes
func setupTestRouter(authManager *AuthManager) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	handlers := NewAuthHandlers(authManager)
	api := r.Group("/api/v1")
	handlers.SetupRoutes(api)

	return r
}

// TestNewAuthHandlers tests handler creation
func TestNewAuthHandlers(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	handlers := NewAuthHandlers(am)

	require.NotNil(t, handlers)
	assert.NotNil(t, handlers.authManager)
}

// TestSetupRoutes tests route registration
func TestSetupRoutes(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	// Verify routes are registered by checking they don't return 404
	routes := r.Routes()

	expectedRoutes := []string{
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"GET /api/v1/auth/me",
		"GET /api/v1/auth/status",
		"GET /api/v1/api-keys",
		"POST /api/v1/api-keys",
		"DELETE /api/v1/api-keys/:id",
		"GET /api/v1/admin/users",
		"POST /api/v1/admin/users",
		"GET /api/v1/admin/rate-limit-stats",
	}

	routeMap := make(map[string]bool)
	for _, route := range routes {
		key := route.Method + " " + route.Path
		routeMap[key] = true
	}

	for _, expectedRoute := range expectedRoutes {
		assert.True(t, routeMap[expectedRoute], "Route %s should be registered", expectedRoute)
	}
}

// TestRegister tests user registration with session creation
func TestRegister(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful registration",
			requestBody: RegisterRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LoginResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotNil(t, response.User)
				assert.Equal(t, "newuser", response.User.Username)
				assert.NotEmpty(t, response.ExpiresAt)
				assert.Contains(t, response.Message, "Registration successful")

				// Check session cookie is set
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "session_id" {
						found = true
						assert.NotEmpty(t, cookie.Value)
						assert.True(t, cookie.HttpOnly)
						break
					}
				}
				assert.True(t, found, "session_id cookie should be set")
			},
		},
		{
			name: "missing required fields",
			requestBody: map[string]string{
				"username": "testuser",
				// missing email and password
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "duplicate username",
			requestBody: RegisterRequest{
				Username: "admin", // Default admin user exists
				Email:    "admin2@example.com",
				Password: "password123",
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "invalid email format",
			requestBody: RegisterRequest{
				Username: "testuser",
				Email:    "invalid-email",
				Password: "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "password too short",
			requestBody: RegisterRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "short",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
			r := setupTestRouter(am)

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestLogin tests user login with session cookie
func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func(*AuthManager) *User
		requestBody    interface{}
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			setupUser: func(am *AuthManager) *User {
				user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
				return user
			},
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "password123",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LoginResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotNil(t, response.User)
				assert.Equal(t, "testuser", response.User.Username)
				assert.NotEmpty(t, response.ExpiresAt)
				assert.Contains(t, response.Message, "Login successful")

				// Check session cookie is set
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "session_id" {
						found = true
						assert.NotEmpty(t, cookie.Value)
						assert.True(t, cookie.HttpOnly)
						break
					}
				}
				assert.True(t, found, "session_id cookie should be set")
			},
		},
		{
			name: "invalid credentials - wrong password",
			setupUser: func(am *AuthManager) *User {
				user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
				return user
			},
			requestBody: LoginRequest{
				Username: "testuser",
				Password: "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "user not found",
			setupUser: func(am *AuthManager) *User {
				return nil
			},
			requestBody: LoginRequest{
				Username: "nonexistent",
				Password: "password123",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "missing required fields",
			setupUser: func(am *AuthManager) *User {
				return nil
			},
			requestBody: map[string]string{
				"username": "testuser",
				// missing password
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
			r := setupTestRouter(am)

			if tt.setupUser != nil {
				tt.setupUser(am)
			}

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestLogout tests user logout with session revocation
func TestLogout(t *testing.T) {
	tests := []struct {
		name           string
		setupSession   func(*AuthManager) string
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful logout with session",
			setupSession: func(am *AuthManager) string {
				user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
				session, _ := am.CreateSession(user.ID)
				return session.ID
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["message"], "logged out")

				// Check cookie is cleared
				cookies := w.Result().Cookies()
				for _, cookie := range cookies {
					if cookie.Name == "session_id" {
						assert.Equal(t, "", cookie.Value)
						assert.Equal(t, -1, cookie.MaxAge)
					}
				}
			},
		},
		{
			name: "logout without session",
			setupSession: func(am *AuthManager) string {
				return ""
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["message"], "logged out")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
			r := setupTestRouter(am)

			sessionID := ""
			if tt.setupSession != nil {
				sessionID = tt.setupSession(am)
			}

			req, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
			if sessionID != "" {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestGetCurrentUserHandler tests retrieving the current authenticated user handler
func TestGetCurrentUserHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
	session, _ := am.CreateSession(user.ID)

	tests := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "authenticated with session cookie",
			setupRequest: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response User
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "testuser", response.Username)
			},
		},
		{
			name: "not authenticated",
			setupRequest: func(req *http.Request) {
				// No session cookie
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
			if tt.setupRequest != nil {
				tt.setupRequest(req)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestGetAuthStatus tests retrieving authentication status
func TestGetAuthStatus(t *testing.T) {
	am := NewAuthManager(AuthConfig{
		JWTSecret:      "test-secret",
		AllowAnonymous: false,
		RateLimit:      100,
	})
	r := setupTestRouter(am)

	tests := []struct {
		name          string
		setupContext  func(*gin.Context)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "status endpoint returns config and auth state",
			setupContext: func(c *gin.Context) {
				// Don't set up any auth - this endpoint is public
			},
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, true, response["authentication_enabled"])
				assert.Equal(t, false, response["allow_anonymous"])
				assert.Equal(t, float64(100), response["rate_limit"])
				assert.NotEmpty(t, response["jwt_expiry"])
				assert.NotEmpty(t, response["session_expiry"])

				// Since no user is in context, should be false
				assert.Equal(t, false, response["authenticated"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/auth/status", nil)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestCreateAPIKeyHandler tests API key creation handler
func TestCreateAPIKeyHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
	session, _ := am.CreateSession(user.ID)

	tests := []struct {
		name           string
		requestBody    interface{}
		authenticated  bool
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful API key creation",
			requestBody: CreateAPIKeyRequest{
				Name:        "test-key",
				Permissions: []string{"read", "write"},
				RateLimit:   50,
				ExpiresIn:   "30d",
			},
			authenticated:  true,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response CreateAPIKeyResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.NotEmpty(t, response.ID)
				assert.Equal(t, "test-key", response.Name)
				assert.NotEmpty(t, response.Key)
				assert.False(t, response.ExpiresAt.IsZero())
			},
		},
		{
			name: "not authenticated",
			requestBody: CreateAPIKeyRequest{
				Name: "test-key",
			},
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name:           "missing required name field",
			requestBody:    map[string]string{},
			authenticated:  true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
		{
			name: "invalid expiry duration",
			requestBody: CreateAPIKeyRequest{
				Name:      "test-key",
				ExpiresIn: "invalid",
			},
			authenticated:  true,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/api-keys", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.authenticated {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestListAPIKeysHandler tests listing API keys handler
func TestListAPIKeysHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
	session, _ := am.CreateSession(user.ID)

	// Create some API keys
	am.CreateAPIKey(user.ID, "key1", []string{"read"}, 100, 30*24*time.Hour)
	am.CreateAPIKey(user.ID, "key2", []string{"write"}, 100, 30*24*time.Hour)

	tests := []struct {
		name           string
		authenticated  bool
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "list API keys successfully",
			authenticated:  true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				keys := response["api_keys"].([]interface{})
				assert.GreaterOrEqual(t, len(keys), 2)
			},
		},
		{
			name:           "not authenticated",
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/api-keys", nil)

			if tt.authenticated {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestRevokeAPIKeyHandler tests revoking an API key handler
func TestRevokeAPIKeyHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	user, _ := am.CreateUserWithPassword("testuser", "test@example.com", "password123", []string{"user"})
	session, _ := am.CreateSession(user.ID)

	apiKey, _ := am.CreateAPIKey(user.ID, "test-key", []string{"read"}, 100, 30*24*time.Hour)

	tests := []struct {
		name           string
		keyID          string
		authenticated  bool
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "revoke API key successfully",
			keyID:          apiKey.ID,
			authenticated:  true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["message"], "revoked")
			},
		},
		{
			name:           "not authenticated",
			keyID:          apiKey.ID,
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "API key not found",
			keyID:          "nonexistent",
			authenticated:  true,
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/api/v1/api-keys/"+tt.keyID, nil)

			if tt.authenticated {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: session.ID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestCreateUserHandler tests admin user creation endpoint handler
func TestCreateUserHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	// Create admin user
	adminUser, _ := am.CreateUserWithPassword("adminuser", "admin@example.com", "password123", []string{"admin", "user"})
	adminSession, _ := am.CreateSession(adminUser.ID)

	// Create regular user
	regularUser, _ := am.CreateUserWithPassword("regularuser", "regular@example.com", "password123", []string{"user"})
	regularSession, _ := am.CreateSession(regularUser.ID)

	tests := []struct {
		name           string
		requestBody    interface{}
		sessionID      string
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "admin creates user successfully",
			requestBody: CreateUserRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
				Roles:    []string{"user"},
			},
			sessionID:      adminSession.ID,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response User
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "newuser", response.Username)
			},
		},
		{
			name: "regular user cannot create user",
			requestBody: CreateUserRequest{
				Username: "anotheruser",
				Email:    "another@example.com",
			},
			sessionID:      regularSession.ID,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not authenticated",
			requestBody:    CreateUserRequest{Username: "user", Email: "user@example.com"},
			sessionID:      "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/admin/users", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.sessionID != "" {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: tt.sessionID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestListUsersHandler tests admin user listing endpoint handler
func TestListUsersHandler(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	// Create admin user
	adminUser, _ := am.CreateUserWithPassword("adminuser", "admin@example.com", "password123", []string{"admin", "user"})
	adminSession, _ := am.CreateSession(adminUser.ID)

	// Create regular user
	regularUser, _ := am.CreateUserWithPassword("regularuser", "regular@example.com", "password123", []string{"user"})
	regularSession, _ := am.CreateSession(regularUser.ID)

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "admin lists users successfully",
			sessionID:      adminSession.ID,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				users := response["users"].([]interface{})
				assert.GreaterOrEqual(t, len(users), 2)
			},
		},
		{
			name:           "regular user cannot list users",
			sessionID:      regularSession.ID,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not authenticated",
			sessionID:      "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/admin/users", nil)

			if tt.sessionID != "" {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: tt.sessionID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestGetRateLimitStats tests rate limit stats endpoint
func TestGetRateLimitStats(t *testing.T) {
	am := NewAuthManager(AuthConfig{JWTSecret: "test-secret"})
	r := setupTestRouter(am)

	// Create admin user
	adminUser, _ := am.CreateUserWithPassword("adminuser", "admin@example.com", "password123", []string{"admin", "user"})
	adminSession, _ := am.CreateSession(adminUser.ID)

	tests := []struct {
		name           string
		sessionID      string
		expectedStatus int
	}{
		{
			name:           "admin gets rate limit stats",
			sessionID:      adminSession.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not authenticated",
			sessionID:      "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/admin/rate-limit-stats", nil)

			if tt.sessionID != "" {
				req.AddCookie(&http.Cookie{Name: "session_id", Value: tt.sessionID})
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestParseDuration tests the parseDuration helper function
func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "empty string returns default",
			input:    "",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "days format",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "weeks format",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "years format",
			input:    "1y",
			expected: 365 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "hours format",
			input:    "48h",
			expected: 48 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "invalid number",
			input:    "xd",
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestFormatAuthErrorResponse tests error response formatting
func TestFormatAuthErrorResponse(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		check func(*testing.T, gin.H)
	}{
		{
			name: "enhanced error with all fields",
			err: errors.New(errors.ErrCodeInvalidInput, "Test error").
				WithDetails("Test details").
				WithSuggestion("Test suggestion").
				WithMetadata("key", "value"),
			check: func(t *testing.T, response gin.H) {
				errorObj := response["error"].(gin.H)
				assert.Equal(t, errors.ErrCodeInvalidInput, errorObj["code"])
				assert.Equal(t, "Test error", errorObj["message"])
				assert.Equal(t, "Test details", errorObj["details"])
				assert.Equal(t, "Test suggestion", errorObj["suggestion"])
				assert.NotNil(t, errorObj["metadata"])
			},
		},
		{
			name: "enhanced error with minimal fields",
			err:  errors.New(errors.ErrCodeNotAuthenticated, "Auth required"),
			check: func(t *testing.T, response gin.H) {
				errorObj := response["error"].(gin.H)
				assert.Equal(t, errors.ErrCodeNotAuthenticated, errorObj["code"])
				assert.Equal(t, "Auth required", errorObj["message"])
			},
		},
		{
			name: "regular error fallback",
			err:  assert.AnError,
			check: func(t *testing.T, response gin.H) {
				errorObj := response["error"].(gin.H)
				assert.Equal(t, "INTERNAL_ERROR", errorObj["code"])
				assert.NotEmpty(t, errorObj["message"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := formatAuthErrorResponse(tt.err)
			assert.NotNil(t, response)
			assert.Contains(t, response, "error")

			if tt.check != nil {
				tt.check(t, response)
			}
		})
	}
}
