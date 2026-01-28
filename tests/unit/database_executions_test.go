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

func setupDBForExecutions(t *testing.T) *database.DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-exec-*.db")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()); tmpFile.Close() })

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	t.Cleanup(func() { os.Unsetenv("AGENTBOX_DB_PATH") })

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='executions'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "executions table should exist")
	return db
}

// ensureEnvironment creates a minimal environment so executions can reference it (FK).
func ensureEnvironmentForExecutions(t *testing.T, db *database.DB, ctx context.Context, envID string) {
	t.Helper()
	env := &models.Environment{
		ID:        envID,
		Name:      "env-for-exec",
		Status:    models.StatusRunning,
		Image:     "busybox",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		Namespace: "ns-" + envID,
		Resources: models.ResourceSpec{CPU: "100m", Memory: "128Mi", Storage: "1Gi"},
	}
	err := db.SaveEnvironment(ctx, env)
	require.NoError(t, err)
}

func TestDatabaseSaveAndGetExecution(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-1")

	exec := &models.Execution{
		ID:            "exec-1",
		EnvironmentID: "env-1",
		UserID:        "user-1",
		Command:       []string{"echo", "hello"},
		Status:        models.ExecutionStatusCompleted,
		PodName:       "exec-pod-1",
		Namespace:     "ns-env-1",
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
		Stdout:        "hello",
		ExitCode:      intPtr(0),
	}

	err := db.SaveExecution(ctx, exec)
	require.NoError(t, err)

	got, err := db.GetExecution(ctx, "exec-1")
	require.NoError(t, err)
	assert.Equal(t, exec.ID, got.ID)
	assert.Equal(t, exec.EnvironmentID, got.EnvironmentID)
	assert.Equal(t, exec.Status, got.Status)
	assert.Equal(t, exec.Command, got.Command)
	assert.Equal(t, exec.Stdout, got.Stdout)
	assert.Equal(t, *exec.ExitCode, *got.ExitCode)
}

func TestDatabaseGetExecutionNotFound(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()

	_, err := db.GetExecution(ctx, "non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDatabaseSaveExecutionWithOptionalFields(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-1")

	dur := int64(150)
	exec := &models.Execution{
		ID:            "exec-2",
		EnvironmentID: "env-1",
		UserID:        "user-1",
		Command:       []string{"sh", "-c", "exit 1"},
		Env:           map[string]string{"FOO": "bar"},
		Status:        models.ExecutionStatusFailed,
		PodName:       "exec-pod-2",
		Namespace:     "ns-env-1",
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
		Stderr:        "error",
		Error:         "command failed",
		ExitCode:      intPtr(1),
		DurationMs:    &dur,
	}

	err := db.SaveExecution(ctx, exec)
	require.NoError(t, err)

	got, err := db.GetExecution(ctx, "exec-2")
	require.NoError(t, err)
	assert.Equal(t, models.ExecutionStatusFailed, got.Status)
	assert.Equal(t, "error", got.Stderr)
	assert.Equal(t, "command failed", got.Error)
	assert.NotNil(t, got.Env)
	assert.Equal(t, "bar", got.Env["FOO"])
	assert.NotNil(t, got.DurationMs)
	assert.Equal(t, int64(150), *got.DurationMs)
}

func TestDatabaseListExecutions(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-list")

	now := time.Now().UTC().Truncate(time.Millisecond)
	ids := []string{"list-exec-a", "list-exec-b", "list-exec-c"}
	for i, id := range ids {
		exec := &models.Execution{
			ID:            id,
			EnvironmentID: "env-list",
			UserID:        "user-1",
			Command:       []string{"true"},
			Status:        models.ExecutionStatusCompleted,
			CreatedAt:     now.Add(time.Duration(i) * time.Second),
		}
		err := db.SaveExecution(ctx, exec)
		require.NoError(t, err)
	}

	list, err := db.ListExecutions(ctx, "env-list", 10)
	require.NoError(t, err)
	assert.Len(t, list, 3)
	// Order is created_at DESC (newest first)
	assert.Equal(t, "list-exec-c", list[0].ID)
}

func TestDatabaseListExecutionsOtherEnv(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-a")
	ensureEnvironmentForExecutions(t, db, ctx, "env-b")

	exec := &models.Execution{
		ID:            "exec-other",
		EnvironmentID: "env-a",
		UserID:        "user-1",
		Command:       []string{"true"},
		Status:        models.ExecutionStatusCompleted,
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
	}
	err := db.SaveExecution(ctx, exec)
	require.NoError(t, err)

	list, err := db.ListExecutions(ctx, "env-b", 10)
	require.NoError(t, err)
	assert.Len(t, list, 0)

	list, err = db.ListExecutions(ctx, "env-a", 10)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "exec-other", list[0].ID)
}

func TestDatabaseDeleteExecution(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-1")

	exec := &models.Execution{
		ID:            "exec-delete",
		EnvironmentID: "env-1",
		UserID:        "user-1",
		Command:       []string{"true"},
		Status:        models.ExecutionStatusCompleted,
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
	}
	err := db.SaveExecution(ctx, exec)
	require.NoError(t, err)

	_, err = db.GetExecution(ctx, "exec-delete")
	require.NoError(t, err)

	err = db.DeleteExecution(ctx, "exec-delete")
	require.NoError(t, err)

	_, err = db.GetExecution(ctx, "exec-delete")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDatabaseLoadAllExecutions(t *testing.T) {
	db := setupDBForExecutions(t)
	ctx := context.Background()
	ensureEnvironmentForExecutions(t, db, ctx, "env-1")

	exec := &models.Execution{
		ID:            "exec-load",
		EnvironmentID: "env-1",
		UserID:        "user-1",
		Command:       []string{"true"},
		Status:        models.ExecutionStatusPending,
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
	}
	err := db.SaveExecution(ctx, exec)
	require.NoError(t, err)

	all, err := db.LoadAllExecutions(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 1)
}

func intPtr(i int) *int {
	return &i
}
