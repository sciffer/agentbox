package unit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/models"
)

func setupDBForEnvironments(t *testing.T) *database.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-env-*.db")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()); tmpFile.Close() })

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	t.Cleanup(func() { os.Unsetenv("AGENTBOX_DB_PATH") })

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	// Ensure environments table exists (migration 3)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='environments'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "environments table should exist")
	return db
}

func TestDatabaseSaveAndGetEnvironment(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	env := &models.Environment{
		ID:        "env-1",
		Name:      "test-env",
		Status:    models.StatusRunning,
		Image:     "python:3.11-slim",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-env-1",
		UserID:    "user-1",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)

	got, err := db.GetEnvironment(ctx, "env-1")
	require.NoError(t, err)
	assert.Equal(t, env.ID, got.ID)
	assert.Equal(t, env.Name, got.Name)
	assert.Equal(t, env.Status, got.Status)
	assert.Equal(t, env.Image, got.Image)
	assert.Equal(t, env.Namespace, got.Namespace)
	assert.Equal(t, env.Resources.CPU, got.Resources.CPU)
}

func TestDatabaseGetEnvironmentNotFound(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	_, err := db.GetEnvironment(ctx, "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDatabaseSaveEnvironmentWithOptionalFields(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	env := &models.Environment{
		ID:        "env-2",
		Name:      "env-with-opts",
		Status:    models.StatusPending,
		Image:     "node:18",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-env-2",
		Resources: models.ResourceSpec{CPU: "1", Memory: "1Gi", Storage: "1Gi"},
		Env:       map[string]string{"KEY": "value"},
		Command:   []string{"node", "--version"},
		Labels:    map[string]string{"team": "backend"},
		Pool:      &models.PoolConfig{Enabled: true, Size: 2},
	}

	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)

	got, err := db.GetEnvironment(ctx, "env-2")
	require.NoError(t, err)
	assert.Equal(t, "env-with-opts", got.Name)
	assert.NotNil(t, got.Env)
	assert.Equal(t, "value", got.Env["KEY"])
	assert.Equal(t, []string{"node", "--version"}, got.Command)
	assert.NotNil(t, got.Pool)
	assert.True(t, got.Pool.Enabled)
	assert.Equal(t, 2, got.Pool.Size)
}

func TestDatabaseListEnvironments(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	ids := []string{"list-env-a", "list-env-b", "list-env-c"}
	for i, id := range ids {
		env := &models.Environment{
			ID:        id,
			Name:      "env-" + id,
			Status:    models.StatusRunning,
			Image:     "busybox",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			Namespace: "ns-" + id,
			Resources: models.ResourceSpec{CPU: "100m", Memory: "128Mi", Storage: "1Gi"},
		}
		err := db.SaveEnvironment(ctx, env)
		require.NoError(t, err)
	}

	list, err := db.ListEnvironments(ctx, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)

	// Order is created_at DESC (newest first)
	assert.Equal(t, "list-env-c", list[0].ID)
}

func TestDatabaseDeleteEnvironment(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	env := &models.Environment{
		ID:        "env-delete",
		Name:      "to-delete",
		Status:    models.StatusTerminated,
		Image:     "busybox",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-delete",
		Resources: models.ResourceSpec{CPU: "100m", Memory: "128Mi", Storage: "1Gi"},
	}
	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)

	_, err = db.GetEnvironment(ctx, "env-delete")
	require.NoError(t, err)

	err = db.DeleteEnvironment(ctx, "env-delete")
	require.NoError(t, err)

	_, err = db.GetEnvironment(ctx, "env-delete")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDatabaseUpdateEnvironmentStatus(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	env := &models.Environment{
		ID:        "env-status",
		Name:      "status-test",
		Status:    models.StatusPending,
		Image:     "busybox",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-status",
		Resources: models.ResourceSpec{CPU: "100m", Memory: "128Mi", Storage: "1Gi"},
	}
	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)

	started := time.Now().UTC().Truncate(time.Millisecond)
	err = db.UpdateEnvironmentStatus(ctx, "env-status", models.StatusRunning, &started)
	require.NoError(t, err)

	got, err := db.GetEnvironment(ctx, "env-status")
	require.NoError(t, err)
	assert.Equal(t, models.StatusRunning, got.Status)
	assert.NotNil(t, got.StartedAt)
}

func TestDatabaseLoadAllEnvironments(t *testing.T) {
	db := setupDBForEnvironments(t)
	ctx := context.Background()

	env := &models.Environment{
		ID:        "env-load",
		Name:      "load-all",
		Status:    models.StatusRunning,
		Image:     "busybox",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-load",
		Resources: models.ResourceSpec{CPU: "100m", Memory: "128Mi", Storage: "1Gi"},
	}
	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)

	all, err := db.LoadAllEnvironments(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 1)
}
