package unit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

func setupAuthTest(t *testing.T) (*auth.Service, *users.Service, *database.DB) {
	tmpFile, err := os.CreateTemp("", "test-auth-*.db")
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

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	userService := users.NewService(db, logger)
	authService := auth.NewService(db, userService, logger)

	return authService, userService, db
}

func TestLogin(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Login
	req := &auth.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}

	resp, err := authService.Login(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.NotNil(t, resp.User)
	assert.Equal(t, "testuser", resp.User.Username)
	assert.False(t, resp.ExpiresAt.IsZero())
}

func TestLoginInvalidCredentials(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Try login with wrong password
	req := &auth.LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}

	_, err = authService.Login(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid credentials")
}

func TestValidateJWT(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Login to get token
	req := &auth.LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	resp, err := authService.Login(ctx, req)
	require.NoError(t, err)

	// Validate token
	user, err := authService.ValidateJWT(ctx, resp.Token)
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
}

func TestValidateJWTInvalidToken(t *testing.T) {
	authService, _, _ := setupAuthTest(t)
	ctx := context.Background()

	// Try to validate invalid token
	_, err := authService.ValidateJWT(ctx, "invalid-token")
	assert.Error(t, err)
}

func TestCreateAPIKey(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Create API key
	req := &auth.CreateAPIKeyRequest{
		UserID:      user.ID,
		Description: "Test API key",
	}

	apiKey, err := authService.CreateAPIKey(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, apiKey.Key)
	assert.NotEmpty(t, apiKey.ID)
	assert.Equal(t, "Test API key", apiKey.Description)
}

func TestValidateAPIKey(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Create API key
	req := &auth.CreateAPIKeyRequest{
		UserID:      user.ID,
		Description: "Test API key",
	}
	apiKey, err := authService.CreateAPIKey(ctx, req)
	require.NoError(t, err)

	// Validate API key
	validatedUser, err := authService.ValidateAPIKey(ctx, apiKey.Key)
	require.NoError(t, err)
	assert.Equal(t, user.ID, validatedUser.ID)
	assert.Equal(t, "testuser", validatedUser.Username)
}

func TestValidateAPIKeyInvalid(t *testing.T) {
	authService, _, _ := setupAuthTest(t)
	ctx := context.Background()

	// Try to validate invalid API key
	_, err := authService.ValidateAPIKey(ctx, "invalid-api-key")
	assert.Error(t, err)
}

func TestListAPIKeys(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Create multiple API keys
	for i := 0; i < 3; i++ {
		req := &auth.CreateAPIKeyRequest{
			UserID:      user.ID,
			Description: fmt.Sprintf("Key %d", i),
		}
		_, err := authService.CreateAPIKey(ctx, req)
		require.NoError(t, err)
	}

	// List API keys
	keys, err := authService.ListAPIKeys(ctx, user.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 3)
}

func TestRevokeAPIKey(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Create API key
	req := &auth.CreateAPIKeyRequest{
		UserID:      user.ID,
		Description: "Test API key",
	}
	apiKey, err := authService.CreateAPIKey(ctx, req)
	require.NoError(t, err)

	// Validate it works
	_, err = authService.ValidateAPIKey(ctx, apiKey.Key)
	require.NoError(t, err)

	// Revoke API key
	err = authService.RevokeAPIKey(ctx, apiKey.ID, user.ID)
	require.NoError(t, err)

	// Try to validate revoked key
	_, err = authService.ValidateAPIKey(ctx, apiKey.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestAPIKeyExpiration(t *testing.T) {
	authService, userService, _ := setupAuthTest(t)
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

	// Create API key with expiration
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired
	req := &auth.CreateAPIKeyRequest{
		UserID:    user.ID,
		ExpiresAt: &expiresAt,
	}
	apiKey, err := authService.CreateAPIKey(ctx, req)
	require.NoError(t, err)

	// Try to validate expired key
	_, err = authService.ValidateAPIKey(ctx, apiKey.Key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}
