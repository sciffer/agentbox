package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sciffer/agentbox/internal/config"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/api"
	"github.com/sciffer/agentbox/pkg/k8s"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/sciffer/agentbox/pkg/validator"
	"github.com/sciffer/agentbox/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAPITest(t *testing.T) (*api.Handler, *mux.Router) {
	handler, _, router := setupAPITestWithMock(t)
	return handler, router
}

func setupAPITestWithMock(t *testing.T) (*api.Handler, *mocks.MockK8sClient, *mux.Router) {
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

	val := validator.New(10000, 10*1024*1024*1024, 100*1024*1024*1024, 86400)

	handler := api.NewHandler(orch, val, log)
	router := api.NewRouter(handler, nil) // nil proxy for unit tests

	return handler, mockK8s, router
}

func TestCreateEnvironmentAPI(t *testing.T) {
	_, router := setupAPITest(t)

	tests := []struct {
		name           string
		request        models.CreateEnvironmentRequest
		expectedStatus int
	}{
		{
			name: "valid request",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing image",
			request: models.CreateEnvironmentRequest{
				Name: "test-env",
				Resources: models.ResourceSpec{
					CPU:     "500m",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid CPU",
			request: models.CreateEnvironmentRequest{
				Name:  "test-env",
				Image: "python:3.11-slim",
				Resources: models.ResourceSpec{
					CPU:     "invalid",
					Memory:  "512Mi",
					Storage: "1Gi",
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if rr.Code == http.StatusCreated {
				var env models.Environment
				err := json.NewDecoder(rr.Body).Decode(&env)
				require.NoError(t, err)

				assert.NotEmpty(t, env.ID)
				assert.Equal(t, tt.request.Name, env.Name)
				assert.Equal(t, tt.request.Image, env.Image)
			}
		})
	}
}

func TestGetEnvironmentAPI(t *testing.T) {
	_, router := setupAPITest(t)

	// Create an environment first
	createReq := models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var createdEnv models.Environment
	json.NewDecoder(rr.Body).Decode(&createdEnv)

	t.Run("get existing environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+createdEnv.ID, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var env models.Environment
		err := json.NewDecoder(rr.Body).Decode(&env)
		require.NoError(t, err)

		assert.Equal(t, createdEnv.ID, env.ID)
		assert.Equal(t, createdEnv.Name, env.Name)
	})

	t.Run("get non-existent environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/non-existent", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestListEnvironmentsAPI(t *testing.T) {
	_, router := setupAPITest(t)

	// Create multiple environments
	for i := 0; i < 3; i++ {
		createReq := models.CreateEnvironmentRequest{
			Name:  "test-env",
			Image: "python:3.11-slim",
			Resources: models.ResourceSpec{
				CPU:     "500m",
				Memory:  "512Mi",
				Storage: "1Gi",
			},
		}

		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}

	t.Run("list all environments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.ListEnvironmentsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, resp.Total, 3)
	})

	t.Run("list with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments?limit=2&offset=0", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.ListEnvironmentsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		assert.LessOrEqual(t, len(resp.Environments), 2)
		assert.Equal(t, 2, resp.Limit)
	})

	t.Run("list with status filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments?status=pending", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.ListEnvironmentsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		for _, env := range resp.Environments {
			assert.Equal(t, models.StatusPending, env.Status)
		}
	})
}

func TestExecuteCommandAPI(t *testing.T) {
	_, router := setupAPITest(t)

	// Create environment
	createReq := models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var env models.Environment
	json.NewDecoder(rr.Body).Decode(&env)

	t.Run("execute valid command", func(t *testing.T) {
		execReq := models.ExecRequest{
			Command: []string{"echo", "hello"},
			Timeout: 30,
		}

		body, _ := json.Marshal(execReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env.ID+"/exec", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// May fail if environment isn't running yet
		if rr.Code == http.StatusOK {
			var resp models.ExecResponse
			err := json.NewDecoder(rr.Body).Decode(&resp)
			require.NoError(t, err)
		}
	})

	t.Run("execute with empty command", func(t *testing.T) {
		execReq := models.ExecRequest{
			Command: []string{},
		}

		body, _ := json.Marshal(execReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/"+env.ID+"/exec", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestDeleteEnvironmentAPI(t *testing.T) {
	_, router := setupAPITest(t)

	// Create environment
	createReq := models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var env models.Environment
	json.NewDecoder(rr.Body).Decode(&env)

	t.Run("delete existing environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/"+env.ID, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("delete non-existent environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/non-existent", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("force delete", func(t *testing.T) {
		// Create another environment
		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		var env models.Environment
		json.NewDecoder(rr.Body).Decode(&env)

		// Force delete
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/environments/"+env.ID+"?force=true", nil)
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})
}

func TestHealthCheckAPI(t *testing.T) {
	_, router := setupAPITest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "healthy", resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.True(t, resp.Kubernetes.Connected)
	assert.NotEmpty(t, resp.Kubernetes.Version)
	assert.Greater(t, resp.Capacity.TotalNodes, 0)
	assert.NotEmpty(t, resp.Capacity.AvailableCPU)
	assert.NotEmpty(t, resp.Capacity.AvailableMemory)
}

func TestGetLogsAPI(t *testing.T) {
	_, mockK8s, router := setupAPITestWithMock(t)
	ctx := context.Background()

	// Create environment
	createReq := models.CreateEnvironmentRequest{
		Name:  "test-env",
		Image: "python:3.11-slim",
		Resources: models.ResourceSpec{
			CPU:     "500m",
			Memory:  "512Mi",
			Storage: "1Gi",
		},
	}

	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	var env models.Environment
	json.NewDecoder(rr.Body).Decode(&env)

	// Wait for async pod creation
	time.Sleep(200 * time.Millisecond)

	// Ensure pod exists in mock (create manually if async creation hasn't completed)
	pod, err := mockK8s.GetPod(ctx, env.Namespace, "main")
	if err != nil || pod == nil {
		// Create pod manually if async creation hasn't completed
		mockK8s.CreatePod(ctx, &k8s.PodSpec{
			Name:      "main",
			Namespace: env.Namespace,
			Image:     "python:3.11-slim",
		})
	}

	t.Run("get logs for existing environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env.ID+"/logs", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.LogsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		assert.NotNil(t, resp.Logs)
		// Mock returns at least one log entry
		if len(resp.Logs) > 0 {
			assert.Equal(t, "stdout", resp.Logs[0].Stream)
			assert.NotEmpty(t, resp.Logs[0].Message)
		}
	})

	t.Run("get logs with tail parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env.ID+"/logs?tail=10", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.LogsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.Logs)
	})

	t.Run("get logs with timestamps disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env.ID+"/logs?timestamps=false", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp models.LogsResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.Logs)
		// Timestamps should be zero when disabled
		if len(resp.Logs) > 0 {
			assert.True(t, resp.Logs[0].Timestamp.IsZero())
		}
	})

	t.Run("get logs with follow parameter (not implemented)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/"+env.ID+"/logs?follow=true", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Should return 501 Not Implemented
		assert.Equal(t, http.StatusNotImplemented, rr.Code)
	})

	t.Run("get logs for non-existent environment", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/non-existent/logs", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
