package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sciffer/agentbox/pkg/users"
)

// === User Management API Tests ===

func TestListUsersAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	// Create admin and regular users
	createUserForTest(t, userService, "admin", "adminpass123", users.RoleSuperAdmin)
	createUserForTest(t, userService, "user1", "password123", users.RoleUser)
	createUserForTest(t, userService, "user2", "password123", users.RoleUser)

	adminToken := getTokenForUser(t, router, "admin", "adminpass123")
	userToken := getTokenForUser(t, router, "user1", "password123")

	t.Run("admin can list users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Users []*users.User `json:"users"`
			Total int           `json:"total"`
		}
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, resp.Total, 3)
	})

	t.Run("regular user cannot list users", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("list with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users?limit=2&offset=0", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var resp struct {
			Users []*users.User `json:"users"`
			Total int           `json:"total"`
		}
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		assert.LessOrEqual(t, len(resp.Users), 2)
	})
}

func TestCreateUserAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	// Create admin user
	createUserForTest(t, userService, "admin", "adminpass123", users.RoleSuperAdmin)
	createUserForTest(t, userService, "regularuser", "password123", users.RoleUser)

	adminToken := getTokenForUser(t, router, "admin", "adminpass123")
	userToken := getTokenForUser(t, router, "regularuser", "password123")

	t.Run("admin can create user", func(t *testing.T) {
		createReq := users.CreateUserRequest{
			Username: "newuser",
			Password: "newpassword123",
			Role:     users.RoleUser,
			Status:   users.StatusActive,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var user users.User
		err := json.NewDecoder(rr.Body).Decode(&user)
		require.NoError(t, err)

		assert.Equal(t, "newuser", user.Username)
		assert.Equal(t, users.RoleUser, user.Role)
		assert.NotEmpty(t, user.ID)
	})

	t.Run("regular user cannot create user", func(t *testing.T) {
		createReq := users.CreateUserRequest{
			Username: "anotheruser",
			Password: "password123",
			Role:     users.RoleUser,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("create user with missing username", func(t *testing.T) {
		createReq := users.CreateUserRequest{
			Password: "password123",
			Role:     users.RoleUser,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("create user with short password", func(t *testing.T) {
		createReq := users.CreateUserRequest{
			Username: "shortpwuser",
			Password: "short",
			Role:     users.RoleUser,
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("create user with default role and status", func(t *testing.T) {
		createReq := struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{
			Username: "defaultsuser",
			Password: "password123",
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var user users.User
		err := json.NewDecoder(rr.Body).Decode(&user)
		require.NoError(t, err)

		// Should have default role and status
		assert.Equal(t, "user", user.Role)
		assert.Equal(t, users.StatusActive, user.Status)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		createReq := users.CreateUserRequest{
			Username: "failuser",
			Password: "password123",
		}
		body, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestGetUserAPIRoute(t *testing.T) {
	router, _, _, userService := setupFullAPITest(t)

	// Create users
	admin := createUserForTest(t, userService, "admin", "adminpass123", users.RoleSuperAdmin)
	user1 := createUserForTest(t, userService, "user1", "password123", users.RoleUser)
	createUserForTest(t, userService, "user2", "password123", users.RoleUser)

	adminToken := getTokenForUser(t, router, "admin", "adminpass123")
	user1Token := getTokenForUser(t, router, "user1", "password123")

	t.Run("admin can get any user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user1.ID, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var user users.User
		err := json.NewDecoder(rr.Body).Decode(&user)
		require.NoError(t, err)

		assert.Equal(t, user1.ID, user.ID)
		assert.Equal(t, "user1", user.Username)
	})

	t.Run("user can get own profile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user1.ID, nil)
		req.Header.Set("Authorization", "Bearer "+user1Token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("user cannot get other user profile", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+admin.ID, nil)
		req.Header.Set("Authorization", "Bearer "+user1Token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/non-existent-id", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("unauthenticated request fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user1.ID, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}
