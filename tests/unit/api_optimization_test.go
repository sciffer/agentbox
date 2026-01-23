package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sciffer/agentbox/pkg/models"
)

func TestCreateEnvironmentLargeBody(t *testing.T) {
	_, router := setupAPITest(t)

	// Create a request with very large body (should be rejected or return 400)
	// Note: MaxBytesReader returns 400 Bad Request, not 413
	largeBody := make([]byte, 2*1024*1024) // 2MB
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// MaxBytesReader returns 400 Bad Request when limit exceeded
	assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusRequestEntityTooLarge, 
		"Expected 400 or 413, got %d", rr.Code)
}

func TestListEnvironmentsInvalidPagination(t *testing.T) {
	_, router := setupAPITest(t)

	tests := []struct {
		name   string
		url    string
		status int
	}{
		{"negative limit", "/api/v1/environments?limit=-1", http.StatusOK}, // Should default to 100
		{"negative offset", "/api/v1/environments?offset=-1", http.StatusOK}, // Should default to 0
		{"invalid limit", "/api/v1/environments?limit=abc", http.StatusOK}, // Should default to 100
		{"invalid offset", "/api/v1/environments?offset=xyz", http.StatusOK}, // Should default to 0
		{"very large limit", "/api/v1/environments?limit=999999", http.StatusOK}, // Should cap
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			assert.Equal(t, tt.status, rr.Code)
		})
	}
}

func TestGetLogsInvalidTail(t *testing.T) {
	// This test is covered in api_test.go TestGetLogsAPI
	// Skipping to avoid duplication
	t.Skip("Covered in TestGetLogsAPI")
}

func TestHealthCheckErrorHandling(t *testing.T) {
	_, mockK8s, router := setupAPITestWithMock(t)

	// Make health check fail
	if mockK8s != nil {
		mockK8s.SetHealthCheckError(true)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should return 503 Service Unavailable
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	var resp models.HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", resp.Status)
	assert.False(t, resp.Kubernetes.Connected)
}
