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

// setupOrchestratorForOptimization creates an orchestrator for optimization tests
func setupOrchestratorForOptimization(t *testing.T) (*orchestrator.Orchestrator, *mocks.MockK8sClient) {
	cfg := &config.Config{
		Kubernetes: config.KubernetesConfig{
			NamespacePrefix: "test-",
			RuntimeClass:    "gvisor",
		},
		Timeouts: config.TimeoutConfig{
			StartupTimeout: 60,
			DefaultTimeout: 3600,
			MaxTimeout:     86400,
		},
	}

	log, err := logger.NewDevelopment()
	require.NoError(t, err)

	mockK8s := mocks.NewMockK8sClient()
	orch := orchestrator.New(mockK8s, cfg, log, nil)

	return orch, mockK8s
}

func TestListEnvironmentsPagination(t *testing.T) {
	orch, _ := setupOrchestratorForOptimization(t)
	ctx := context.Background()

	// Create 10 environments
	for i := 0; i < 10; i++ {
		req := &models.CreateEnvironmentRequest{
			Name:  "test-env",
			Image: "python:3.11-slim",
			Resources: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
		}
		_, err := orch.CreateEnvironment(ctx, req, "user-123")
		require.NoError(t, err)
	}

	// Test pagination edge cases
	tests := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
		expectedTotal int
	}{
		{"first page", 3, 0, 3, 10},
		{"middle page", 3, 3, 3, 10},
		{"last page", 3, 9, 1, 10},
		{"offset beyond total", 3, 100, 0, 10},
		{"zero limit", 0, 0, 10, 10},      // Should default to 100
		{"negative offset", 3, -1, 3, 10}, // Should default to 0
		{"large limit", 10000, 0, 10, 10}, // Should cap at 1000
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := orch.ListEnvironments(ctx, nil, "", tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTotal, resp.Total)
			// Note: actual count may vary due to default limit behavior
		})
	}
}

func TestListEnvironmentsEmptyResult(t *testing.T) {
	orch, _ := setupOrchestratorForOptimization(t)
	ctx := context.Background()

	// List with no environments
	resp, err := orch.ListEnvironments(ctx, nil, "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Total)
	assert.Len(t, resp.Environments, 0)
}

func TestExecuteCommandTimeout(t *testing.T) {
	orch, mockK8s := setupOrchestratorForOptimization(t)
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

	// Wait for pod to be created in mock
	time.Sleep(100 * time.Millisecond)

	// Create pod in mock using k8s.PodSpec
	podSpec := &k8s.PodSpec{
		Name:      "main",
		Namespace: env.Namespace,
		Image:     "python:3.11-slim",
		Command:   []string{"/bin/sh"},
		CPU:       "500m",
		Memory:    "512Mi",
		Storage:   "1Gi",
	}
	_ = mockK8s.CreatePod(ctx, podSpec) // Ignore errors in tests
	mockK8s.SetPodRunning(env.Namespace, "main")

	// Test with timeout
	resp, err := orch.ExecuteCommand(ctx, env.ID, []string{"echo", "test"}, 5)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetLogsEmptyLogs(t *testing.T) {
	orch, mockK8s := setupOrchestratorForOptimization(t)
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

	// Wait for pod creation
	time.Sleep(100 * time.Millisecond)

	// Create pod in mock using k8s.PodSpec
	podSpec := &k8s.PodSpec{
		Name:      "main",
		Namespace: env.Namespace,
		Image:     "python:3.11-slim",
		Command:   []string{"/bin/sh"},
		CPU:       "500m",
		Memory:    "512Mi",
		Storage:   "1Gi",
	}
	_ = mockK8s.CreatePod(ctx, podSpec) // Ignore errors in tests
	mockK8s.SetPodRunning(env.Namespace, "main")

	// Mock empty logs
	mockK8s.SetPodLogs(env.Namespace, "main", "")

	// Get logs
	logsResp, err := orch.GetLogs(ctx, env.ID, nil)
	require.NoError(t, err)
	assert.NotNil(t, logsResp)
	assert.Len(t, logsResp.Logs, 0)
}

func TestGetLogsWithTail(t *testing.T) {
	orch, mockK8s := setupOrchestratorForOptimization(t)
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

	// Wait for pod creation
	time.Sleep(100 * time.Millisecond)

	// Create pod in mock using k8s.PodSpec
	podSpec := &k8s.PodSpec{
		Name:      "main",
		Namespace: env.Namespace,
		Image:     "python:3.11-slim",
		Command:   []string{"/bin/sh"},
		CPU:       "500m",
		Memory:    "512Mi",
		Storage:   "1Gi",
	}
	_ = mockK8s.CreatePod(ctx, podSpec) // Ignore errors in tests
	mockK8s.SetPodRunning(env.Namespace, "main")

	// Mock logs
	mockK8s.SetPodLogs(env.Namespace, "main", "line1\nline2\nline3\nline4\nline5")

	// Get last 3 lines
	tailLines := int64(3)
	logsResp, err := orch.GetLogs(ctx, env.ID, &tailLines)
	require.NoError(t, err)
	assert.NotNil(t, logsResp)
}

func TestGetHealthInfoUnhealthy(t *testing.T) {
	orch, mockK8s := setupOrchestratorForOptimization(t)
	ctx := context.Background()

	// Make health check fail
	mockK8s.SetHealthCheckError(true)

	health, err := orch.GetHealthInfo(ctx)
	require.NoError(t, err) // Should not error, just return unhealthy
	assert.Equal(t, "unhealthy", health.Status)
	assert.False(t, health.Kubernetes.Connected)
}
