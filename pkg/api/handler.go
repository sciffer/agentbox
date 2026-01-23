package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/orchestrator"
	"github.com/sciffer/agentbox/pkg/validator"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	orchestrator *orchestrator.Orchestrator
	validator    *validator.Validator
	logger       *logger.Logger
}

// NewHandler creates a new API handler
func NewHandler(orch *orchestrator.Orchestrator, val *validator.Validator, log *logger.Logger) *Handler {
	return &Handler{
		orchestrator: orch,
		validator:    val,
		logger:       log,
	}
}

// CreateEnvironment handles POST /environments
func (h *Handler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req models.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	
	// Validate request
	if err := h.validator.ValidateCreateRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation failed", err)
		return
	}
	
	// Get user ID from context (set by auth middleware)
	userID := getUserIDFromContext(ctx)
	
	// Create environment
	env, err := h.orchestrator.CreateEnvironment(ctx, &req, userID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to create environment", err)
		return
	}
	
	h.logger.Info("environment created",
		zap.String("environment_id", env.ID),
		zap.String("user_id", userID),
	)
	
	h.respondJSON(w, http.StatusCreated, env)
}

// GetEnvironment handles GET /environments/{id}
func (h *Handler) GetEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]
	
	env, err := h.orchestrator.GetEnvironment(ctx, envID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "environment not found", err)
		return
	}
	
	h.respondJSON(w, http.StatusOK, env)
}

// ListEnvironments handles GET /environments
func (h *Handler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse query parameters
	query := r.URL.Query()
	
	var status *models.EnvironmentStatus
	if statusStr := query.Get("status"); statusStr != "" {
		s := models.EnvironmentStatus(statusStr)
		status = &s
	}
	
	labelSelector := query.Get("label")
	
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
	
	resp, err := h.orchestrator.ListEnvironments(ctx, status, labelSelector, limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list environments", err)
		return
	}
	
	h.respondJSON(w, http.StatusOK, resp)
}

// ExecuteCommand handles POST /environments/{id}/exec
func (h *Handler) ExecuteCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]
	
	var req models.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	
	// Validate request
	if err := h.validator.ValidateExecRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation failed", err)
		return
	}
	
	// Execute command
	resp, err := h.orchestrator.ExecuteCommand(ctx, envID, req.Command, req.Timeout)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to execute command", err)
		return
	}
	
	h.respondJSON(w, http.StatusOK, resp)
}

// DeleteEnvironment handles DELETE /environments/{id}
func (h *Handler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]
	
	force := r.URL.Query().Get("force") == "true"
	
	if err := h.orchestrator.DeleteEnvironment(ctx, envID, force); err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to delete environment", err)
		return
	}
	
	h.logger.Info("environment deleted",
		zap.String("environment_id", envID),
		zap.Bool("force", force),
	)
	
	w.WriteHeader(http.StatusNoContent)
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	resp, err := h.orchestrator.GetHealthInfo(ctx)
	if err != nil {
		// If we can't get health info, return unhealthy status
		resp = &models.HealthResponse{
			Status:  "unhealthy",
			Version: "1.0.0",
			Kubernetes: models.KubernetesHealthStatus{
				Connected: false,
				Version:   "",
			},
			Capacity: models.ClusterCapacity{},
		}
		h.logger.Error("failed to get health info", zap.Error(err))
	}
	
	// Return 503 if unhealthy, 200 if healthy
	statusCode := http.StatusOK
	if resp.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	h.respondJSON(w, statusCode, resp)
}

// GetLogs handles GET /environments/{id}/logs
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]

	// Parse query parameters
	query := r.URL.Query()
	
	var tailLines *int64
	if tailStr := query.Get("tail"); tailStr != "" {
		if tail, err := strconv.ParseInt(tailStr, 10, 64); err == nil && tail > 0 {
			tailLines = &tail
		}
	}

	// TODO: Support follow parameter for streaming (requires SSE or WebSocket)
	// For now, we'll just return the logs
	follow := query.Get("follow") == "true"
	if follow {
		h.respondError(w, http.StatusNotImplemented, "streaming logs not yet implemented", fmt.Errorf("follow parameter not supported"))
		return
	}

	// Get logs
	logsResp, err := h.orchestrator.GetLogs(ctx, envID, tailLines)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}

	// Include timestamps by default (can be disabled via query param)
	includeTimestamps := query.Get("timestamps") != "false"
	if !includeTimestamps {
		// Remove timestamps from log entries
		for i := range logsResp.Logs {
			logsResp.Logs[i].Timestamp = time.Time{}
		}
	}

	h.respondJSON(w, http.StatusOK, logsResp)
}

// Helper functions

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message, zap.Error(err))
	
	errResp := models.ErrorResponse{
		Error:   message,
		Message: err.Error(),
		Code:    status,
	}
	
	h.respondJSON(w, status, errResp)
}

func getUserIDFromContext(ctx interface{}) string {
	// TODO: Extract user ID from context (set by auth middleware)
	return "anonymous"
}
