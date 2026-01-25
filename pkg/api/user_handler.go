package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/users"
)

// UserHandler handles user management endpoints
type UserHandler struct {
	userService *users.Service
	authService *auth.Service
	logger      *logger.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService *users.Service, authService *auth.Service, log *logger.Logger) *UserHandler {
	return &UserHandler{
		userService: userService,
		authService: authService,
		logger:      log,
	}
}

// ListUsers handles GET /api/v1/users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permissions (admin only)
	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if user.Role != "super_admin" && user.Role != "admin" {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit := 100
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	userList, err := h.userService.ListUsers(ctx, limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list users", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"users": userList,
		"total": len(userList),
	})
}

// CreateUser handles POST /api/v1/users
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permissions (admin only)
	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if user.Role != "super_admin" && user.Role != "admin" {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)

	var req users.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Validate
	if req.Username == "" {
		h.respondError(w, http.StatusBadRequest, "username is required", nil)
		return
	}

	if req.Password != "" && len(req.Password) < 8 {
		h.respondError(w, http.StatusBadRequest, "password must be at least 8 characters", nil)
		return
	}

	if req.Role == "" {
		req.Role = "user" // Default role
	}

	if req.Status == "" {
		req.Status = users.StatusActive // Default status
	}

	// Create user
	createdUser, err := h.userService.CreateUser(ctx, &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create user", err)
		return
	}

	h.logger.Info("user created",
		zap.String("username", createdUser.Username),
		zap.String("created_by", user.Username),
	)

	h.respondJSON(w, http.StatusCreated, createdUser)
}

// GetUser handles GET /api/v1/users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	userID := vars["id"]

	// Check permissions
	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	// Users can only view their own profile unless they're admin
	if user.ID != userID && user.Role != "super_admin" && user.Role != "admin" {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	targetUser, err := h.userService.GetUserByID(ctx, userID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "user not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, targetUser)
}

// Helper methods
func (h *UserHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *UserHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message, zap.Error(err))

	errMsg := message
	if err != nil {
		if status >= 400 && status < 500 {
			errMsg = err.Error()
		}
	}

	errResp := models.ErrorResponse{
		Error:   message,
		Message: errMsg,
		Code:    status,
	}

	h.respondJSON(w, status, errResp)
}
