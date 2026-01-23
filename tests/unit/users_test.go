package unit

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

func setupTestDB(t *testing.T) *database.DB {
	tmpFile, err := os.CreateTemp("", "test-users-*.db")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	t.Cleanup(func() {
		os.Unsetenv("AGENTBOX_DB_PATH")
	})

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	req := &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	}

	user, err := service.CreateUser(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "user", user.Role)
	assert.Equal(t, "active", user.Status)
	assert.NotNil(t, user.Email)
	assert.Equal(t, "test@example.com", *user.Email)
}

func TestGetUserByID(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Create user
	req := &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	}
	created, err := service.CreateUser(ctx, req)
	require.NoError(t, err)

	// Get user
	user, err := service.GetUserByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, "testuser", user.Username)
}

func TestGetUserByUsername(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Create user
	req := &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	}
	_, err := service.CreateUser(ctx, req)
	require.NoError(t, err)

	// Get user
	user, err := service.GetUserByUsername(ctx, "testuser")
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
}

func TestGetUserWithPassword(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Create user
	req := &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "password123",
		Role:     "user",
		Status:   "active",
	}
	_, err := service.CreateUser(ctx, req)
	require.NoError(t, err)

	// Get user with password
	_, hash, err := service.GetUserWithPassword(ctx, "testuser")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.True(t, users.VerifyPassword(hash, "password123"))
	assert.False(t, users.VerifyPassword(hash, "wrongpassword"))
}

func TestListUsers(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 5; i++ {
		req := &users.CreateUserRequest{
			Username: fmt.Sprintf("user%d", i),
			Email:    fmt.Sprintf("user%d@example.com", i),
			Password: "password123",
			Role:     "user",
			Status:   "active",
		}
		_, err := service.CreateUser(ctx, req)
		require.NoError(t, err)
	}

	// List users
	userList, err := service.ListUsers(ctx, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(userList), 5)
}

func TestUpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Create user
	req := &users.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "oldpassword",
		Role:     "user",
		Status:   "active",
	}
	user, err := service.CreateUser(ctx, req)
	require.NoError(t, err)

	// Update password
	err = service.UpdatePassword(ctx, user.ID, "newpassword")
	require.NoError(t, err)

	// Verify new password
	_, hash, err := service.GetUserWithPassword(ctx, "testuser")
	require.NoError(t, err)
	assert.True(t, users.VerifyPassword(hash, "newpassword"))
	assert.False(t, users.VerifyPassword(hash, "oldpassword"))
}

func TestEnsureDefaultAdmin(t *testing.T) {
	db := setupTestDB(t)
	service := users.NewService(db, zap.NewNop())
	ctx := context.Background()

	// Set env vars
	os.Setenv("AGENTBOX_ADMIN_USERNAME", "testadmin")
	os.Setenv("AGENTBOX_ADMIN_PASSWORD", "adminpass")
	defer os.Unsetenv("AGENTBOX_ADMIN_USERNAME")
	defer os.Unsetenv("AGENTBOX_ADMIN_PASSWORD")

	// Ensure default admin
	err := service.EnsureDefaultAdmin(ctx)
	require.NoError(t, err)

	// Verify admin exists
	admin, err := service.GetUserByUsername(ctx, "testadmin")
	require.NoError(t, err)
	assert.Equal(t, "super_admin", admin.Role)
	assert.Equal(t, "active", admin.Status)

	// Verify password
	_, hash, err := service.GetUserWithPassword(ctx, "testadmin")
	require.NoError(t, err)
	assert.True(t, users.VerifyPassword(hash, "adminpass"))
}
