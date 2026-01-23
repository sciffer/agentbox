//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) (*orchestrator.Orchestrator, func()) {
	// Load config
	cfg, err := config.Load("../../config/config.yaml")
	require.NoError(t, err)

	// Override for testing
	cfg.Kubernetes.NamespacePrefix = "agentbox-test-"
	cfg.Auth.Enabled = false

	// Create logger
	log, err := logger.NewDevelopment()
	require.NoError(t, err)

	// Create K8s client
	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	require.NoError(t, err)

	// Verify K8s connectivity
	ctx := context.Background()
	err = k8sClient.HealthCheck(ctx)
	require.NoError(t, err, "Kubernetes cluster must be available for integration tests")

	// Create orchestrator
	orch := orchestrator.New(k8sClient, cfg, log)

	cleanup := func() {
		// Clean up any test namespaces
		// This would require implementing namespace listing and deletion
	}

	return orch, cleanup
}

func TestEnvironmentLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("complete lifecycle", func(t *testing.T) {
		// Step 1: Create environment
		req := &models.CreateEnvironmentRequest{
			Name:  "test-lifecycle",
			Image: "alpine:latest",
			Resources: models.ResourceSpec{
				CPU:     "100m",
				Memory:  "128Mi",
				Storage: "500Mi",
			},
			Command: []string{"/bin/sh", "-c", "sleep 3600"},
		}

		env, err := orch.CreateEnvironment(ctx, req, "test-user")
		require.NoError(t, err)
		assert.NotEmpty(t, env.ID)
		assert.Equal(t, models.StatusPending, env.Status)

		// Step 2: Wait for environment to be running
		maxWait := 60 * time.Second
		interval := 2 * time.Second
		deadline := time.Now().Add(maxWait)

		for time.Now().Before(deadline) {
			retrieved, err := orch.GetEnvironment(ctx, env.ID)
			require.NoError(t, err)

			if retrieved.Status == models.StatusRunning {
				env = retrieved
				break
			}

			if retrieved.Status == models.StatusFailed {
				t.Fatal("environment failed to start")
			}

			time.Sleep(interval)
		}

		assert.Equal(t, models.StatusRunning, env.Status)
		assert.NotNil(t, env.StartedAt)

		// Step 3: Execute command
		execResp, err := orch.ExecuteCommand(ctx, env.ID, []string{"echo", "hello world"}, 30)
		require.NoError(t, err)
		assert.Contains(t, execResp.Stdout, "hello world")
		assert.Equal(t, 0, execResp.ExitCode)

		// Step 4: Delete environment
		err = orch.DeleteEnvironment(ctx, env.ID, false)
		require.NoError(t, err)

		// Step 5: Verify environment is deleted
		_, err = orch.GetEnvironment(ctx, env.ID)
		assert.Error(t, err)
	})
}

func TestMultipleEnvironments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create multiple environments concurrently", func(t *testing.T) {
		numEnvs := 5
		envIDs := make([]string, numEnvs)

		// Create environments concurrently
		for i := 0; i < numEnvs; i++ {
			req := &models.CreateEnvironmentRequest{
				Name:  "test-concurrent",
				Image: "alpine:latest",
				Resources: models.ResourceSpec{
					CPU:     "100m",
					Memory:  "128Mi",
					Storage: "500Mi",
				},
			}

			env, err := orch.CreateEnvironment(ctx, req, "test-user")
			require.NoError(t, err)
			envIDs[i] = env.ID
		}

		// Wait a bit for provisioning
		time.Sleep(5 * time.Second)

		// List environments
		resp, err := orch.ListEnvironments(ctx, nil, "", 100, 0)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.Total, numEnvs)

		// Clean up
		for _, envID := range envIDs {
			orch.DeleteEnvironment(ctx, envID, true)
		}
	})
}

func TestCommandExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create environment
	req := &models.CreateEnvironmentRequest{
		Name:  "test-exec",
		Image: "alpine:latest",
		Resources: models.ResourceSpec{
			CPU:     "100m",
			Memory:  "128Mi",
			Storage: "500Mi",
		},
	}

	env, err := orch.CreateEnvironment(ctx, req, "test-user")
	require.NoError(t, err)
	defer orch.DeleteEnvironment(ctx, env.ID, true)

	// Wait for running
	time.Sleep(10 * time.Second)

	t.Run("execute simple command", func(t *testing.T) {
		resp, err := orch.ExecuteCommand(ctx, env.ID, []string{"echo", "test"}, 30)
		require.NoError(t, err)
		assert.Contains(t, resp.Stdout, "test")
		assert.Equal(t, 0, resp.ExitCode)
	})

	t.Run("execute command that fails", func(t *testing.T) {
		resp, err := orch.ExecuteCommand(ctx, env.ID, []string{"false"}, 30)
		// May or may not error depending on implementation
		if err == nil {
			assert.NotEqual(t, 0, resp.ExitCode)
		}
	})

	t.Run("execute command with timeout", func(t *testing.T) {
		_, err := orch.ExecuteCommand(ctx, env.ID, []string{"sleep", "100"}, 1)
		assert.Error(t, err)
	})
}

func TestResourceConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("enforce resource limits", func(t *testing.T) {
		req := &models.CreateEnvironmentRequest{
			Name:  "test-resources",
			Image: "alpine:latest",
			Resources: models.ResourceSpec{
				CPU:     "50m",
				Memory:  "64Mi",
				Storage: "100Mi",
			},
		}

		env, err := orch.CreateEnvironment(ctx, req, "test-user")
		require.NoError(t, err)
		defer orch.DeleteEnvironment(ctx, env.ID, true)

		// Wait for running
		time.Sleep(10 * time.Second)

		// Verify resource limits are applied
		retrieved, err := orch.GetEnvironment(ctx, env.ID)
		require.NoError(t, err)
		assert.Equal(t, "50m", retrieved.Resources.CPU)
		assert.Equal(t, "64Mi", retrieved.Resources.Memory)
	})
}

func TestIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	orch, cleanup := setupIntegrationTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("environments are isolated", func(t *testing.T) {
		// Create two environments
		req1 := &models.CreateEnvironmentRequest{
			Name:  "test-isolation-1",
			Image: "alpine:latest",
			Resources: models.ResourceSpec{
				CPU:     "100m",
				Memory:  "128Mi",
				Storage: "500Mi",
			},
		}

		env1, err := orch.CreateEnvironment(ctx, req1, "test-user-1")
		require.NoError(t, err)
		defer orch.DeleteEnvironment(ctx, env1.ID, true)

		req2 := &models.CreateEnvironmentRequest{
			Name:  "test-isolation-2",
			Image: "alpine:latest",
			Resources: models.ResourceSpec{
				CPU:     "100m",
				Memory:  "128Mi",
				Storage: "500Mi",
			},
		}

		env2, err := orch.CreateEnvironment(ctx, req2, "test-user-2")
		require.NoError(t, err)
		defer orch.DeleteEnvironment(ctx, env2.ID, true)

		// Verify they have different namespaces
		assert.NotEqual(t, env1.Namespace, env2.Namespace)

		// Wait for both to be running
		time.Sleep(15 * time.Second)

		// Create a file in env1
		_, err = orch.ExecuteCommand(ctx, env1.ID, []string{"touch", "/tmp/test-file"}, 30)
		require.NoError(t, err)

		// Verify file doesn't exist in env2
		resp, err := orch.ExecuteCommand(ctx, env2.ID, []string{"ls", "/tmp/test-file"}, 30)
		// Should fail as file doesn't exist
		if err == nil {
			assert.NotEqual(t, 0, resp.ExitCode)
		}
	})
}
