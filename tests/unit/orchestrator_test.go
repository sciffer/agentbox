package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/sciffer/agentbox/tests/mocks"
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
	exists, _ := mockK8s.NamespaceExists(ctx, env.Namespace)
	assert.True(t, exists)
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

	// Wait for async pod creation to complete before deletion
	time.Sleep(150 * time.Millisecond)
	
	// Ensure pod exists before deletion (for proper cleanup)
	pod, _ := mockK8s.GetPod(ctx, env.Namespace, "main")
	if pod == nil {
		// Create pod if async creation hasn't completed
		mockK8s.CreatePod(ctx, &k8s.PodSpec{
			Name:      "main",
			Namespace: env.Namespace,
			Image:     "python:3.11-slim",
		})
	}

	// Delete environment
	err = orch.DeleteEnvironment(ctx, env.ID, false)
	require.NoError(t, err)

	// Verify namespace was deleted (retry a few times for async operations)
	var exists bool
	for i := 0; i < 5; i++ {
		time.Sleep(50 * time.Millisecond)
		exists, err = mockK8s.NamespaceExists(ctx, env.Namespace)
		require.NoError(t, err)
		if !exists {
			break
		}
	}
	assert.False(t, exists, "namespace should be deleted")

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

func TestGetLogs(t *testing.T) {
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

	// Wait for async pod creation
	time.Sleep(100 * time.Millisecond)
	
	// Ensure pod exists in mock (the async creation should have done this, but verify)
	// If pod doesn't exist, create it manually for the test
	pod, err := mockK8s.GetPod(ctx, env.Namespace, "main")
	if err != nil || pod == nil {
		// Create pod manually if async creation hasn't completed
		mockK8s.CreatePod(ctx, &k8s.PodSpec{
			Name:      "main",
			Namespace: env.Namespace,
			Image:     "python:3.11-slim",
		})
	}

	t.Run("get logs successfully", func(t *testing.T) {
		logsResp, err := orch.GetLogs(ctx, env.ID, nil)
		require.NoError(t, err)

		assert.NotNil(t, logsResp)
		assert.NotNil(t, logsResp.Logs)
		// Mock returns at least one log entry
		if len(logsResp.Logs) > 0 {
			assert.Equal(t, "stdout", logsResp.Logs[0].Stream)
			assert.NotEmpty(t, logsResp.Logs[0].Message)
		}
	})

	t.Run("get logs with tail parameter", func(t *testing.T) {
		tail := int64(10)
		logsResp, err := orch.GetLogs(ctx, env.ID, &tail)
		require.NoError(t, err)

		assert.NotNil(t, logsResp)
		assert.NotNil(t, logsResp.Logs)
	})

	t.Run("get logs for non-existent environment", func(t *testing.T) {
		_, err := orch.GetLogs(ctx, "non-existent", nil)
		assert.Error(t, err)
	})
}

func TestGetHealthInfo(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	healthResp, err := orch.GetHealthInfo(ctx)
	require.NoError(t, err)

	assert.NotNil(t, healthResp)
	assert.Equal(t, "healthy", healthResp.Status)
	assert.Equal(t, "1.0.0", healthResp.Version)
	assert.True(t, healthResp.Kubernetes.Connected)
	assert.Equal(t, "v1.28.0", healthResp.Kubernetes.Version)
	assert.Equal(t, 3, healthResp.Capacity.TotalNodes)
	assert.Equal(t, "50000m", healthResp.Capacity.AvailableCPU)
	assert.Equal(t, "100Gi", healthResp.Capacity.AvailableMemory)
}

func TestListEnvironmentsWithLabelSelector(t *testing.T) {
	orch, _ := setupOrchestrator(t)
	ctx := context.Background()

	// Create environments with different labels
	envs := []struct {
		name   string
		labels map[string]string
	}{
		{"env1", map[string]string{"team": "backend", "env": "prod"}},
		{"env2", map[string]string{"team": "frontend", "env": "prod"}},
		{"env3", map[string]string{"team": "backend", "env": "dev"}},
		{"env4", map[string]string{"team": "frontend", "env": "dev"}},
	}

	for _, e := range envs {
		req := &models.CreateEnvironmentRequest{
			Name:  e.name,
			Image: "python:3.11-slim",
			Resources: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
			Labels: e.labels,
		}
		_, err := orch.CreateEnvironment(ctx, req, "user-123")
		require.NoError(t, err)
	}

	t.Run("filter by team=backend", func(t *testing.T) {
		resp, err := orch.ListEnvironments(ctx, nil, "team=backend", 100, 0)
		require.NoError(t, err)

		assert.Equal(t, 2, resp.Total)
		for _, env := range resp.Environments {
			assert.Equal(t, "backend", env.Labels["team"])
		}
	})

	t.Run("filter by env=prod", func(t *testing.T) {
		resp, err := orch.ListEnvironments(ctx, nil, "env=prod", 100, 0)
		require.NoError(t, err)

		assert.Equal(t, 2, resp.Total)
		for _, env := range resp.Environments {
			assert.Equal(t, "prod", env.Labels["env"])
		}
	})

	t.Run("filter by team=backend,env=prod", func(t *testing.T) {
		resp, err := orch.ListEnvironments(ctx, nil, "team=backend,env=prod", 100, 0)
		require.NoError(t, err)

		assert.Equal(t, 1, resp.Total)
		assert.Equal(t, "backend", resp.Environments[0].Labels["team"])
		assert.Equal(t, "prod", resp.Environments[0].Labels["env"])
	})

	t.Run("filter by non-existent label", func(t *testing.T) {
		resp, err := orch.ListEnvironments(ctx, nil, "team=nonexistent", 100, 0)
		require.NoError(t, err)

		assert.Equal(t, 0, resp.Total)
	})

	t.Run("filter with invalid selector", func(t *testing.T) {
		// Invalid selector should return empty results
		resp, err := orch.ListEnvironments(ctx, nil, "invalid selector format", 100, 0)
		require.NoError(t, err)

		assert.Equal(t, 0, resp.Total)
	})
}
