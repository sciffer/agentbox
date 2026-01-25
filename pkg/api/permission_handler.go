package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/permissions"
	"github.com/sciffer/agentbox/pkg/users"
)

// PermissionHandler handles permission-related endpoints
type PermissionHandler struct {
	permissionService *permissions.Service
	userService       *users.Service
	logger            *logger.Logger
}

// NewPermissionHandler creates a new permission handler
func NewPermissionHandler(permissionService *permissions.Service, userService *users.Service, log *logger.Logger) *PermissionHandler {
	return &PermissionHandler{
		permissionService: permissionService,
		userService:       userService,
		logger:            log,
	}
}

// ListUserPermissions handles GET /api/v1/users/{id}/permissions
func (h *PermissionHandler) ListUserPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	targetUserID := vars["id"]

	// Check permissions - users can view their own, admins can view any
	currentUser, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if currentUser.ID != targetUserID && currentUser.Role != users.RoleSuperAdmin && currentUser.Role != users.RoleAdmin {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	perms, err := h.permissionService.ListUserPermissions(ctx, targetUserID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list permissions", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"permissions": perms,
	})
}

// GrantPermissionRequest is the request body for granting permissions
type GrantPermissionRequest struct {
	EnvironmentID string `json:"environment_id"`
	Permission    string `json:"permission"`
}

// GrantPermission handles POST /api/v1/users/{id}/permissions
func (h *PermissionHandler) GrantPermission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	targetUserID := vars["id"]

	// Check permissions - only admins can grant permissions
	currentUser, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if currentUser.Role != users.RoleSuperAdmin && currentUser.Role != users.RoleAdmin {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	// Verify target user exists
	_, err := h.userService.GetUserByID(ctx, targetUserID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "user not found", err)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	var req GrantPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Validate
	if req.EnvironmentID == "" {
		h.respondError(w, http.StatusBadRequest, "environment_id is required", nil)
		return
	}

	if !permissions.ValidatePermission(req.Permission) {
		h.respondError(w, http.StatusBadRequest, "invalid permission level (must be viewer, editor, or owner)", nil)
		return
	}

	// Grant permission
	perm, err := h.permissionService.GrantPermission(ctx, targetUserID, req.EnvironmentID, req.Permission, currentUser.ID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to grant permission", err)
		return
	}

	h.logger.Info("permission granted",
		zap.String("target_user_id", targetUserID),
		zap.String("environment_id", req.EnvironmentID),
		zap.String("permission", req.Permission),
		zap.String("granted_by", currentUser.ID),
	)

	h.respondJSON(w, http.StatusCreated, perm)
}

// UpdatePermissionRequest is the request body for updating permissions
type UpdatePermissionRequest struct {
	Permission string `json:"permission"`
}

// UpdatePermission handles PUT /api/v1/users/{id}/permissions/{envId}
func (h *PermissionHandler) UpdatePermission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	targetUserID := vars["id"]
	environmentID := vars["envId"]

	// Check permissions - only admins can update permissions
	currentUser, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if currentUser.Role != users.RoleSuperAdmin && currentUser.Role != users.RoleAdmin {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	// Parse request
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)
	var req UpdatePermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	if !permissions.ValidatePermission(req.Permission) {
		h.respondError(w, http.StatusBadRequest, "invalid permission level (must be viewer, editor, or owner)", nil)
		return
	}

	// Update permission
	perm, err := h.permissionService.UpdatePermission(ctx, targetUserID, environmentID, req.Permission)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "permission not found", err)
		return
	}

	h.logger.Info("permission updated",
		zap.String("target_user_id", targetUserID),
		zap.String("environment_id", environmentID),
		zap.String("permission", req.Permission),
		zap.String("updated_by", currentUser.ID),
	)

	h.respondJSON(w, http.StatusOK, perm)
}

// RevokePermission handles DELETE /api/v1/users/{id}/permissions/{envId}
func (h *PermissionHandler) RevokePermission(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	targetUserID := vars["id"]
	environmentID := vars["envId"]

	// Check permissions - only admins can revoke permissions
	currentUser, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if currentUser.Role != users.RoleSuperAdmin && currentUser.Role != users.RoleAdmin {
		h.respondError(w, http.StatusForbidden, "insufficient permissions", nil)
		return
	}

	// Revoke permission
	err := h.permissionService.RevokePermission(ctx, targetUserID, environmentID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "permission not found", err)
		return
	}

	h.logger.Info("permission revoked",
		zap.String("target_user_id", targetUserID),
		zap.String("environment_id", environmentID),
		zap.String("revoked_by", currentUser.ID),
	)

	w.WriteHeader(http.StatusNoContent)
}

// ListAPIKeyPermissions handles GET /api/v1/api-keys/{id}/permissions
func (h *PermissionHandler) ListAPIKeyPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	apiKeyID := vars["id"]

	currentUser, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	// TODO: Verify the API key belongs to the user or user is admin
	_ = currentUser

	perms, err := h.permissionService.ListAPIKeyPermissions(ctx, apiKeyID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list API key permissions", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"permissions": perms,
	})
}

// Helper methods
func (h *PermissionHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *PermissionHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
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
