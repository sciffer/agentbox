package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/api"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

func setupAuthAPITest(t *testing.T) (*api.AuthHandler, *auth.Service, *users.Service) {
	tmpFile, err := os.CreateTemp("", "test-api-auth-*.db")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	os.Setenv("AGENTBOX_JWT_SECRET", "test-secret-key")
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
