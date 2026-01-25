package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/permissions"
	"github.com/sciffer/agentbox/pkg/users"
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	authService       *auth.Service
	permissionService *permissions.Service
	logger            *logger.Logger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(authService *auth.Service, permissionService *permissions.Service, log *logger.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		authService:       authService,
		permissionService: permissionService,
		logger:            log,
	}
}

// ListAPIKeys handles GET /api/v1/api-keys
func (h *APIKeyHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	keys, err := h.authService.ListAPIKeys(ctx, user.ID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list API keys", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"api_keys": keys,
	})
}

// CreateAPIKeyRequest is the request body for creating an API key
type CreateAPIKeyRequestBody struct {
	Description string `json:"description"`
	ExpiresIn   *int   `json:"expires_in"` // Days until expiration
	Permissions []struct {
		EnvironmentID string `json:"environment_id"`
		Permission    string `json:"permission"`
	} `json:"permissions,omitempty"`
}

// CreateAPIKey handles POST /api/v1/api-keys
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var req CreateAPIKeyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Validate permissions if provided
	var permReqs []auth.APIKeyPermissionRequest
	if len(req.Permissions) > 0 {
		// Super admins can grant any permission
		isSuperAdmin := user.Role == users.RoleSuperAdmin

		for _, p := range req.Permissions {
			// Validate permission level
			if !permissions.ValidatePermission(p.Permission) {
				h.respondError(w, http.StatusBadRequest, "invalid permission level: "+p.Permission, nil)
				return
			}

			// For non-super-admins, verify they have at least the permission they're trying to grant
			if !isSuperAdmin {
				userPerm, err := h.permissionService.GetUserPermission(ctx, user.ID, p.EnvironmentID)
				if err != nil {
					h.respondError(w, http.StatusInternalServerError, "failed to check user permissions", err)
					return
				}
				if userPerm == nil {
					h.respondError(w, http.StatusForbidden,
						"you don't have access to environment: "+p.EnvironmentID, nil)
					return
				}
				// User can only grant permissions up to their own level
				userLevel := permissions.PermissionLevel(userPerm.Permission)
				requestedLevel := permissions.PermissionLevel(p.Permission)
				if requestedLevel > userLevel {
					h.respondError(w, http.StatusForbidden,
						"cannot grant permission higher than your own for environment: "+p.EnvironmentID, nil)
					return
				}
			}

			permReqs = append(permReqs, auth.APIKeyPermissionRequest{
				EnvironmentID: p.EnvironmentID,
				Permission:    p.Permission,
			})
		}
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &exp
	}

	createReq := &auth.CreateAPIKeyRequest{
		UserID:      user.ID,
		Description: req.Description,
		ExpiresAt:   expiresAt,
		Permissions: permReqs,
	}

	apiKey, err := h.authService.CreateAPIKey(ctx, createReq)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create API key", err)
		return
	}

	// Store API key permissions
	if len(permReqs) > 0 {
		permsToStore := make([]struct {
			EnvironmentID string
			Permission    string
		}, len(permReqs))
		for i, p := range permReqs {
			permsToStore[i] = struct {
				EnvironmentID string
				Permission    string
			}{
				EnvironmentID: p.EnvironmentID,
				Permission:    p.Permission,
			}
		}
		if err := h.permissionService.SetAPIKeyPermissions(ctx, apiKey.ID, permsToStore); err != nil {
			// Log but don't fail - the key was created
			h.logger.Error("failed to store API key permissions", zap.Error(err))
		}

		// Add permissions to response
		apiKey.Permissions = make([]auth.APIKeyPermissionResponse, len(permReqs))
		for i, p := range permReqs {
			apiKey.Permissions[i] = auth.APIKeyPermissionResponse(p)
		}
	}

	h.logger.Info("API key created",
		zap.String("user_id", user.ID),
		zap.String("key_id", apiKey.ID),
		zap.Int("permissions_count", len(permReqs)),
	)

	h.respondJSON(w, http.StatusCreated, apiKey)
}

// RevokeAPIKey handles DELETE /api/v1/api-keys/{id}
func (h *APIKeyHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	keyID := vars["id"]

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	if err := h.authService.RevokeAPIKey(ctx, keyID, user.ID); err != nil {
		h.respondError(w, http.StatusNotFound, "failed to revoke API key", err)
		return
	}

	h.logger.Info("API key revoked",
		zap.String("user_id", user.ID),
		zap.String("key_id", keyID),
	)

	w.WriteHeader(http.StatusNoContent)
}

// Helper methods
func (h *APIKeyHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *APIKeyHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
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
