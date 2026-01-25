package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/api"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

// setupAuthAPITest initializes the handler for simple auth handler tests
func setupAuthAPITest(t *testing.T) (*api.AuthHandler, *auth.Service, *users.Service) {
	tmpFile, err := os.CreateTemp("", "test-api-auth-*.db")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	os.Setenv("AGENTBOX_JWT_SECRET", "test-secret-key-min-32-chars-for-safety")
	t.Cleanup(func() {
		os.Unsetenv("AGENTBOX_DB_PATH")
		os.Unsetenv("AGENTBOX_JWT_SECRET")
	})

	zapLogger := zap.NewNop()
	db, err := database.NewDB(zapLogger)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	userService := users.NewService(db, zapLogger)
	authService := auth.NewService(db, userService, zapLogger)
	log, _ := logger.NewDevelopment()
	authHandler := api.NewAuthHandler(authService, userService, log)

	return authHandler, authService, userService
}

// setupFullAPITest creates a full router with auth, user, and API key handlers
func setupFullAPITest(t *testing.T) (*mux.Router, *database.DB, *auth.Service, *users.Service) {
	tmpFile, err := os.CreateTemp("", "test-full-api-*.db")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	os.Setenv("AGENTBOX_JWT_SECRET", "test-secret-key-min-32-chars-for-safety")
	os.Setenv("AGENTBOX_JWT_EXPIRY", "1h")
	t.Cleanup(func() {
		os.Unsetenv("AGENTBOX_DB_PATH")
		os.Unsetenv("AGENTBOX_JWT_SECRET")
		os.Unsetenv("AGENTBOX_JWT_EXPIRY")
	})

	zapLogger := zap.NewNop()
	db, err := database.NewDB(zapLogger)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	log, _ := logger.NewDevelopment()
	userService := users.NewService(db, zapLogger)
	authService := auth.NewService(db, userService, zapLogger)

	// Create handlers
	authHandler := api.NewAuthHandler(authService, userService, log)
	userHandler := api.NewUserHandler(userService, authService, log)
	apiKeyHandler := api.NewAPIKeyHandler(authService, log)

	// Create router with auth routes
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Public auth routes (no auth required)
	apiRouter.HandleFunc("/auth/login", authHandler.Login).Methods("POST")
	apiRouter.HandleFunc("/auth/logout", authHandler.Logout).Methods("POST")

	// Auth routes that need user context (protected)
	protectedAuth := apiRouter.PathPrefix("/auth").Subrouter()
	protectedAuth.Use(authService.Middleware)
	protectedAuth.HandleFunc("/me", authHandler.GetMe).Methods("GET")
	protectedAuth.HandleFunc("/change-password", authHandler.ChangePassword).Methods("POST")

	// Protected routes (with middleware)
	protected := apiRouter.PathPrefix("").Subrouter()
	protected.Use(authService.Middleware)

	// User management routes
	protected.HandleFunc("/users", userHandler.ListUsers).Methods("GET")
	protected.HandleFunc("/users", userHandler.CreateUser).Methods("POST")
	protected.HandleFunc("/users/{id}", userHandler.GetUser).Methods("GET")

	// API key routes
	protected.HandleFunc("/api-keys", apiKeyHandler.ListAPIKeys).Methods("GET")
	protected.HandleFunc("/api-keys", apiKeyHandler.CreateAPIKey).Methods("POST")
	protected.HandleFunc("/api-keys/{id}", apiKeyHandler.RevokeAPIKey).Methods("DELETE")

	return router, db, authService, userService
}

// createUserForTest creates a test user and returns the user object
func createUserForTest(t *testing.T, userService *users.Service, username, password, role string) *users.User {
	ctx := context.Background()
	user, err := userService.CreateUser(ctx, &users.CreateUserRequest{
		Username: username,
		Password: password,
		Role:     role,
		Status:   users.StatusActive,
	})
	require.NoError(t, err)
	return user
}

// getTokenForUser logs in a user and returns the JWT token
func getTokenForUser(t *testing.T, router *mux.Router, username, password string) string {
	loginReq := auth.LoginRequest{
		Username: username,
		Password: password,
	}
	body, _ := json.Marshal(loginReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "login failed: %s", rr.Body.String())

	var resp auth.LoginResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	return resp.Token
}

// === Basic Auth Handler Tests ===

func TestLoginAPI(t *testing.T) {
	authHandler, _, userService := setupAuthAPITest(t)
	ctx := context.Background()

	// Create user
	_, err := userService.CreateUser(ctx, &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	})
	require.NoError(t, err)

	// Login request
	reqBody := map[string]string{
		"username": "testuser",
		"password": "password123",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	authHandler.Login(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["token"])
	assert.NotNil(t, resp["user"])
}

func TestLoginAPIInvalidCredentials(t *testing.T) {
	authHandler, _, userService := setupAuthAPITest(t)
	ctx := context.Background()

	// Create user
	_, err := userService.CreateUser(ctx, &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	})
	require.NoError(t, err)

	// Login with wrong password
	reqBody := map[string]string{
		"username": "testuser",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	authHandler.Login(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestGetMeAPI(t *testing.T) {
	authHandler, authService, userService := setupAuthAPITest(t)
	ctx := context.Background()

	// Create user
	user, err := userService.CreateUser(ctx, &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	})
	require.NoError(t, err)

	// Login to get token
	loginReq := &auth.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	loginResp, err := authService.Login(ctx, loginReq)
	require.NoError(t, err)

	// Get /auth/me
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rr := httptest.NewRecorder()

	// Create a handler that includes auth middleware
	// For testing, we'll call GetMe directly with a context that has the user
	// In real usage, middleware would set this
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, user))
	authHandler.GetMe(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "testuser", resp["username"])
}

// === Full Router Auth Tests ===

func TestLoginAPIWithRouter(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)

	tests := []struct {
		name           string
		username       string
		password       string
		expectedStatus int
	}{
		{
			name:           "valid credentials",
			username:       "testuser",
			password:       "password123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid password",
			username:       "testuser",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "non-existent user",
			username:       "nonexistent",
			password:       "password123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty username",
			username:       "",
			password:       "password123",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty password",
			username:       "testuser",
			password:       "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginReq := auth.LoginRequest{
				Username: tt.username,
				Password: tt.password,
			}
			body, _ := json.Marshal(loginReq)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp auth.LoginResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)

				assert.NotEmpty(t, resp.Token)
				assert.NotNil(t, resp.User)
				assert.Equal(t, tt.username, resp.User.Username)
			}
		})
	}
}

func TestLogoutAPIWithRouter(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestGetMeAPIWithRouter(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	t.Run("authenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var user users.User
		err := json.NewDecoder(rr.Body).Decode(&user)
		require.NoError(t, err)

		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, users.RoleUser, user.Role)
	})

	t.Run("unauthenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestChangePasswordAPIWithRouter(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	t.Run("valid password change", func(t *testing.T) {
		reqBody := map[string]string{
			"current_password": "password123",
			"new_password":     "newpassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Verify new password works
		newToken := getTokenForUser(t, router, "testuser", "newpassword123")
		assert.NotEmpty(t, newToken)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		reqBody := map[string]string{
			"current_password": "password123",
			"new_password":     "newpassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
