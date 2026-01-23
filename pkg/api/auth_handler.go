package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/users"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *auth.Service
	userService *users.Service
	logger      *logger.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *auth.Service, userService *users.Service, log *logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userService: userService,
		logger:      log,
	}
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // ctx is used in authService.Login

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024) // 4KB limit

	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.Username == "" || req.Password == "" {
		h.respondError(w, http.StatusBadRequest, "username and password are required", nil)
		return
	}

	// Authenticate
	resp, err := h.authService.Login(ctx, &req)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "authentication failed", err)
		return
	}

	h.logger.Info("user logged in",
		zap.String("username", resp.User.Username),
		zap.String("user_id", resp.User.ID),
	)

	h.respondJSON(w, http.StatusOK, resp)
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// For JWT, logout is handled client-side by discarding the token
	// For API keys, they can be revoked via the API key management endpoint
	w.WriteHeader(http.StatusNoContent)
}

// GetMe handles GET /api/v1/auth/me
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	h.respondJSON(w, http.StatusOK, user)
}

// ChangePassword handles POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Validate
	if req.CurrentPassword == "" || req.NewPassword == "" {
		h.respondError(w, http.StatusBadRequest, "current_password and new_password are required", nil)
		return
	}

	if len(req.NewPassword) < 8 {
		h.respondError(w, http.StatusBadRequest, "new password must be at least 8 characters", nil)
		return
	}

	// Verify current password
	_, passwordHash, err := h.userService.GetUserWithPassword(ctx, user.Username)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to verify password", err)
		return
	}

	if !users.VerifyPassword(passwordHash, req.CurrentPassword) {
		h.respondError(w, http.StatusUnauthorized, "current password is incorrect", nil)
		return
	}

	// Update password
	if err := h.userService.UpdatePassword(ctx, user.ID, req.NewPassword); err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to update password", err)
		return
	}

	h.respondJSON(w, http.StatusOK, models.ErrorResponse{
		Error:   "success",
		Message: "password changed successfully",
		Code:    http.StatusOK,
	})
}

// Helper methods (reuse from handler.go)
func (h *AuthHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *AuthHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
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
