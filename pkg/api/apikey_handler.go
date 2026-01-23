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
)

// APIKeyHandler handles API key management endpoints
type APIKeyHandler struct {
	authService *auth.Service
	logger      *logger.Logger
}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler(authService *auth.Service, log *logger.Logger) *APIKeyHandler {
	return &APIKeyHandler{
		authService: authService,
		logger:      log,
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

// CreateAPIKey handles POST /api/v1/api-keys
func (h *APIKeyHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := auth.GetUserFromContext(ctx)
	if !ok {
		h.respondError(w, http.StatusUnauthorized, "not authenticated", nil)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 4*1024)

	var req struct {
		Description string `json:"description"`
		ExpiresIn   *int   `json:"expires_in"` // Days until expiration
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		exp := time.Now().Add(time.Duration(*req.ExpiresIn) * 24 * time.Hour)
		expiresAt = &exp
	}

	createReq := &auth.CreateAPIKeyRequest{
		UserID:      user.ID,
		Description: req.Description,
		ExpiresAt:   expiresAt,
	}

	apiKey, err := h.authService.CreateAPIKey(ctx, createReq)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create API key", err)
		return
	}

	h.logger.Info("API key created",
		zap.String("user_id", user.ID),
		zap.String("key_id", apiKey.ID),
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
