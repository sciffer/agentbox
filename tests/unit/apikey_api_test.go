package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/users"
)

// === API Key Management API Tests ===

func TestListAPIKeysAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	t.Run("list empty API keys", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			APIKeys []*auth.APIKeyInfo `json:"api_keys"`
		}
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		// Should be empty or nil (no keys created yet)
		assert.True(t, resp.APIKeys == nil || len(resp.APIKeys) == 0)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestCreateAPIKeyAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	t.Run("create API key with description", func(t *testing.T) {
		createReq := struct {
			Description string `json:"description"`
			ExpiresIn   *int   `json:"expires_in,omitempty"`
		}{
			Description: "Test API key",
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var apiKey auth.APIKeyResponse
		err := json.NewDecoder(rr.Body).Decode(&apiKey)
		require.NoError(t, err)

		assert.NotEmpty(t, apiKey.ID)
		assert.NotEmpty(t, apiKey.Key)       // Full key only returned on creation
		assert.NotEmpty(t, apiKey.KeyPrefix) // e.g., "ak_live_"
		assert.Equal(t, "Test API key", apiKey.Description)
	})

	t.Run("create API key with expiration", func(t *testing.T) {
		expiresIn := 30 // 30 days
		createReq := struct {
			Description string `json:"description"`
			ExpiresIn   *int   `json:"expires_in,omitempty"`
		}{
			Description: "Expiring API key",
			ExpiresIn:   &expiresIn,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var apiKey auth.APIKeyResponse
		err := json.NewDecoder(rr.Body).Decode(&apiKey)
		require.NoError(t, err)

		assert.NotNil(t, apiKey.ExpiresAt)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		createReq := struct {
			Description string `json:"description"`
		}{
			Description: "Test API key",
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestRevokeAPIKeyAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	createUserForTest(t, userService, "otheruser", "password123", users.RoleUser)

	userToken := getTokenForUser(t, router, "testuser", "password123")
	otherUserToken := getTokenForUser(t, router, "otheruser", "password123")

	// Create an API key first
	createReq := struct {
		Description string `json:"description"`
	}{
		Description: "Key to revoke",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var createdKey auth.APIKeyResponse
	err := json.NewDecoder(rr.Body).Decode(&createdKey)
	require.NoError(t, err)

	t.Run("revoke own API key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+createdKey.ID, nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("revoke non-existent key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("cannot revoke other user key", func(t *testing.T) {
		// Create another key
		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var newKey auth.APIKeyResponse
		err := json.NewDecoder(rr.Body).Decode(&newKey)
		require.NoError(t, err)

		// Try to revoke with different user's token
		req = httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/"+newKey.ID, nil)
		req.Header.Set("Authorization", "Bearer "+otherUserToken)
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/api-keys/some-id", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestAPIKeyAuthenticationRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	// Create an API key
	createReq := struct {
		Description string `json:"description"`
	}{
		Description: "Auth test key",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var createdKey auth.APIKeyResponse
	err := json.NewDecoder(rr.Body).Decode(&createdKey)
	require.NoError(t, err)

	t.Run("authenticate with API key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
		req.Header.Set("X-API-Key", createdKey.Key)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid API key fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
		req.Header.Set("X-API-Key", "invalid-api-key")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestListAPIKeysAfterCreationRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	createUserForTest(t, userService, "testuser", "password123", users.RoleUser)
	token := getTokenForUser(t, router, "testuser", "password123")

	// Create a few API keys
	for i := 0; i < 3; i++ {
		createReq := struct {
			Description string `json:"description"`
		}{
			Description: "Test key",
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)
	}

	// List API keys
	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		APIKeys []*auth.APIKeyInfo `json:"api_keys"`
	}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, 3, len(resp.APIKeys))

	// Verify keys don't contain the actual key value (only on creation)
	for _, key := range resp.APIKeys {
		assert.NotEmpty(t, key.ID)
		assert.NotEmpty(t, key.KeyPrefix)
		assert.NotEmpty(t, key.CreatedAt)
	}
}
