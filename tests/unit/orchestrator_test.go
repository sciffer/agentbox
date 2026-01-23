package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sciffertbox/internal/config"
	"github.com/sciffertbox/internal/logger"
	"github.com/sciffertbox/pkg/models"
	"github.com/sciffertbox/pkg/orchestrator"
	"github.com/sciffertbox/tests/mocks"
)

func setupOrchestrator(t *testing.T) (*orchestrator.Orchestrator, *mocks.MockK8sClient) {
	cfg := &config.Config{
		Kubernetes: config.KubernetesConfig{
			NamespacePrefix: "test-",
			RuntimeClass:    "gvisor",
		},
		Timeouts: config.TimeoutConfig{
			StartupTimeout: 60,
		},
	}

	log, err := logger.NewDevelopment()
	require.NoError(t, err)

	mockK8s := mocks.NewMockK8sClient()
	orch := orchestrator.New(mockK8s, cfg, log)

	return orch, mockK8s
}

func TestCreateEnvironment(t *testing.T) {
	orch, mockK8s := setupOrchestrator(t)
	ctx := context.Background()

	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
		Env: map[string]string{
			"TEST_VAR": "value",
		},
		Labels: map[string]string{
			"team": "test",
		},
	}

	env, err := orch.CreateEnvironment(ctx, req, "user-123")
	require.NoError(t, err)

	assert.NotEmpty(t, env.ID)
	assert.Equal(t, "test-env", env.Name)
	assert.Equal(t, models.StatusPending, env.Status)
	assert.Equal(t, "python:3.11-slim", env.Image)
	assert.NotEmpty(t, env.Namespace)
	assert.Equal(t, "user-123", env.UserID)
	assert.NotZero(t, env.CreatedAt)

	// Verify namespace was created
	time.Sleep(100 * time.Millisecond) // Give goroutine time to execute
	assert.True(t, mockK8s.NamespaceExists(env.Namespace))
}

func TestGetEnvironment(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	// Create environment first
	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	created, err := orch.CreateEnvironment(ctx, req, "user-123")
	require.NoError(t, err)

	// Get environment
	retrieved, err := orch.GetEnvironment(ctx, created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Name, retrieved.Name)

	// Try to get non-existent environment
	_, err = orch.GetEnvironment(ctx, "non-existent")
	assert.Error(t, err)
}

func TestListEnvironments(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	// Create multiple environments
	for i := 0; i < 5; i++ {
		req := &models.CreateEnvironmentRequest{
			Name:  "test-env",
			Image: "python:3.11-slim",
			Resources: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
			Labels: map[string]string{
				"index": string(rune('0' + i)),
			},
		}
		_, err := orch.CreateEnvironment(ctx, req, "user-123")
		require.NoError(t, err)
	}

	// List all environments
	resp, err := orch.ListEnvironments(ctx, nil, "", 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Total)
	assert.Len(t, resp.Environments, 5)

	// Test pagination
	resp, err = orch.ListEnvironments(ctx, nil, "", 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Total)
	assert.Len(t, resp.Environments, 2)

	resp, err = orch.ListEnvironments(ctx, nil, "", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Total)
	assert.Len(t, resp.Environments, 2)

	// Filter by status
	status := models.StatusPending
	resp, err = orch.ListEnvironments(ctx, &status, "", 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Total) // All should be pending initially
}

func TestDeleteEnvironment(t *testing.T) {
	orch, mockK8s := setupOrchestrator(t)
	ctx := context.Background()

	// Create environment
	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	env, err := orch.CreateEnvironment(ctx, req, "user-123")
	require.NoError(t, err)

	// Delete environment
	err = orch.DeleteEnvironment(ctx, env.ID, false)
	require.NoError(t, err)

	// Verify namespace was deleted
	assert.False(t, mockK8s.NamespaceExists(env.Namespace))

	// Verify environment is removed from memory
	_, err = orch.GetEnvironment(ctx, env.ID)
	assert.Error(t, err)

	// Try to delete non-existent environment
	err = orch.DeleteEnvironment(ctx, "non-existent", false)
	assert.Error(t, err)
}

func TestExecuteCommand(t *testing.T) {
	orch, mockK8s := setupOrchestrator(t)
	ctx := context.Background()

	// Create environment
	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	env, err := orch.CreateEnvironment(ctx, req, "user-123")
	require.NoError(t, err)

	// Wait for environment to be running
	time.Sleep(100 * time.Millisecond)
	mockK8s.SetPodRunning(env.Namespace, "main")

	// Update status to running
	retrieved, _ := orch.GetEnvironment(ctx, env.ID)
	retrieved.Status = models.StatusRunning

	// Execute command
	resp, err := orch.ExecuteCommand(ctx, env.ID, []string{"echo", "hello"}, 30)
	require.NoError(t, err)

	assert.NotNil(t, resp)
	assert.Equal(t, 0, resp.ExitCode)

	// Try to execute in non-existent environment
	_, err = orch.ExecuteCommand(ctx, "non-existent", []string{"ls"}, 30)
	assert.Error(t, err)
}

func TestEnvironmentIDGeneration(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	// Create multiple environments and verify unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		env, err := orch.CreateEnvironment(ctx, req, "user-123")
		require.NoError(t, err)

		assert.NotEmpty(t, env.ID)
		assert.False(t, ids[env.ID], "ID should be unique")
		ids[env.ID] = true
	}
}

func TestNamespaceGeneration(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	req := &models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	env, err := orch.CreateEnvironment(ctx, req, "user-123")
	require.NoError(t, err)

	// Namespace should start with prefix and contain env ID
	assert.Contains(t, env.Namespace, "test-")
	assert.Contains(t, env.Namespace, env.ID)
	assert.LessOrEqual(t, len(env.Namespace), 63) // K8s limit
}
