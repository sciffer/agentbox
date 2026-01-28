package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/auth"
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

	// Limit request body size to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB limit

	var req models.CreateEnvironmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

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
// Request body must be JSON: {"command": ["cmd", "arg1", ...], "timeout": 300}
func (h *Handler) ExecuteCommand(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB limit for exec requests

	if r.Body == nil || r.ContentLength == 0 {
		h.respondError(w, http.StatusBadRequest, "request body is required (JSON: {\"command\": [\"cmd\", \"arg1\", ...], \"timeout\": 300})", nil)
		return
	}

	var req models.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body: must be JSON with \"command\" array", err)
		return
	}
	defer r.Body.Close()

	// Validate request
	if err := h.validator.ValidateExecRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	// Execute command
	resp, err := h.orchestrator.ExecuteCommand(ctx, envID, req.Command, req.Timeout)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "environment not found", err)
			return
		}
		if strings.Contains(err.Error(), "not running") {
			h.respondError(w, http.StatusBadRequest, "environment is not running", err)
			return
		}
		h.respondError(w, http.StatusInternalServerError, "failed to execute command", err)
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// SubmitExecution handles POST /environments/{id}/run
// This queues an async execution that creates a new isolated pod using the environment's configuration
// Returns immediately with execution ID for polling
func (h *Handler) SubmitExecution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64KB limit

	var req models.EphemeralExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer r.Body.Close()

	// Set environment ID from URL path
	req.EnvironmentID = envID

	// Validate required fields
	if len(req.Command) == 0 {
		h.respondError(w, http.StatusBadRequest, "command is required", nil)
		return
	}

	// Get user ID from context
	userID := getUserIDFromContext(ctx)

	// Submit execution (returns immediately)
	orchReq := &orchestrator.EphemeralExecRequest{
		EnvironmentID: envID,
		Command:       req.Command,
		Timeout:       req.Timeout,
		Env:           req.Env,
	}

	h.logger.Info("submitting execution",
		zap.String("environment_id", envID),
		zap.Strings("command", req.Command),
		zap.String("user_id", userID),
	)

	exec, err := h.orchestrator.SubmitExecution(ctx, orchReq, userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "environment not found", err)
		} else if strings.Contains(err.Error(), "not running") {
			h.respondError(w, http.StatusBadRequest, "environment is not running", err)
		} else {
			h.respondError(w, http.StatusInternalServerError, "failed to submit execution", err)
		}
		return
	}

	// Return execution status
	resp := models.ExecutionResponse{
		ID:            exec.ID,
		EnvironmentID: exec.EnvironmentID,
		Status:        exec.Status,
		CreatedAt:     exec.CreatedAt,
	}

	h.respondJSON(w, http.StatusAccepted, resp)
}

// GetExecution handles GET /executions/{id}
// Returns the current status and result of an execution
func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	execID := vars["id"]

	exec, err := h.orchestrator.GetExecution(ctx, execID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "execution not found", err)
		} else {
			h.respondError(w, http.StatusInternalServerError, "failed to get execution", err)
		}
		return
	}

	resp := models.ExecutionResponse{
		ID:            exec.ID,
		EnvironmentID: exec.EnvironmentID,
		Status:        exec.Status,
		CreatedAt:     exec.CreatedAt,
		StartedAt:     exec.StartedAt,
		CompletedAt:   exec.CompletedAt,
		ExitCode:      exec.ExitCode,
		Stdout:        exec.Stdout,
		Stderr:        exec.Stderr,
		Error:         exec.Error,
		DurationMs:    exec.DurationMs,
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// ListExecutions handles GET /environments/{id}/executions
// Returns list of executions for an environment
func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]

	// Parse limit parameter
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	resp, err := h.orchestrator.ListExecutions(ctx, envID, limit)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to list executions", err)
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// CancelExecution handles DELETE /executions/{id}
// Cancels a pending or running execution
func (h *Handler) CancelExecution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	execID := vars["id"]

	if err := h.orchestrator.CancelExecution(ctx, execID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(w, http.StatusNotFound, "execution not found", err)
		} else if strings.Contains(err.Error(), "cannot be canceled") {
			h.respondError(w, http.StatusBadRequest, err.Error(), nil)
		} else {
			h.respondError(w, http.StatusInternalServerError, "failed to cancel execution", err)
		}
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
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

	follow := query.Get("follow") == "true"
	includeTimestamps := query.Get("timestamps") != "false"

	// If follow=true, stream logs using Server-Sent Events (SSE)
	if follow {
		h.streamLogs(w, r, ctx, envID, tailLines, includeTimestamps)
		return
	}

	// Get logs (non-streaming)
	logsResp, err := h.orchestrator.GetLogs(ctx, envID, tailLines)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get logs", err)
		return
	}

	// Include timestamps by default (can be disabled via query param)
	if !includeTimestamps {
		// Remove timestamps from log entries
		for i := range logsResp.Logs {
			logsResp.Logs[i].Timestamp = time.Time{}
		}
	}

	h.respondJSON(w, http.StatusOK, logsResp)
}

// streamLogs streams logs using Server-Sent Events (SSE)
func (h *Handler) streamLogs(w http.ResponseWriter, r *http.Request, ctx context.Context, envID string, tailLines *int64, includeTimestamps bool) {
	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create a context that can be canceled when client disconnects
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitor client disconnect
	go func() {
		<-r.Context().Done()
		cancel()
	}()

	// Get log stream from orchestrator
	logsStream, err := h.orchestrator.StreamLogs(streamCtx, envID, tailLines, true)
	if err != nil {
		h.logger.Error("failed to stream logs", zap.String("environment_id", envID), zap.Error(err))
		// Send error as SSE event
		errorJSON, marshalErr := json.Marshal(map[string]string{"error": fmt.Sprintf("failed to stream logs: %v", err)})
		if marshalErr == nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errorJSON))
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}
	defer logsStream.Close()

	// Create a flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream logs line by line
	scanner := bufio.NewScanner(logsStream)
	now := time.Now()

	for scanner.Scan() {
		// Check if context was canceled (client disconnected)
		select {
		case <-streamCtx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Create log entry
		logEntry := models.LogEntry{
			Timestamp: now,
			Stream:    "stdout",
			Message:   line,
		}

		// Format as JSON
		logJSON, err := json.Marshal(logEntry)
		if err != nil {
			h.logger.Warn("failed to marshal log entry", zap.Error(err))
			continue
		}

		// Send as SSE event
		if !includeTimestamps {
			// Create log entry without timestamp
			logEntry = models.LogEntry{
				Timestamp: time.Time{},
				Stream:    "stdout",
				Message:   line,
			}
			logJSON, err = json.Marshal(logEntry)
			if err != nil {
				h.logger.Warn("failed to marshal log entry", zap.Error(err))
				continue
			}
		}
		fmt.Fprintf(w, "data: %s\n\n", string(logJSON))

		flusher.Flush()
		now = time.Now() // Update timestamp for next line
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		h.logger.Error("error reading log stream", zap.Error(err))
		errorJSON, marshalErr := json.Marshal(map[string]string{"error": fmt.Sprintf("error reading logs: %v", err)})
		if marshalErr == nil {
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errorJSON))
			flusher.Flush()
		}
	}
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

	// For 4xx include error details; for 5xx include error details to help debugging (e.g. exec failures)
	errMsg := message
	if err != nil {
		errMsg = err.Error()
	}

	errResp := models.ErrorResponse{
		Error:   message,
		Message: errMsg,
		Code:    status,
	}

	h.respondJSON(w, status, errResp)
}

func getUserIDFromContext(ctx context.Context) string {
	// Extract user ID from context (set by auth middleware)
	// Try to get user from auth context first
	if user, ok := auth.GetUserFromContext(ctx); ok && user != nil {
		return user.ID
	}
	return "anonymous"
}

// GetPoolStatus handles GET /pool/status
// Returns the current standby pod pool status
func (h *Handler) GetPoolStatus(w http.ResponseWriter, r *http.Request) {
	status := h.orchestrator.GetPoolStatus()

	resp := map[string]interface{}{
		"pools": status,
		"total": func() int {
			total := 0
			for _, count := range status {
				total += count
			}
			return total
		}(),
	}

	h.respondJSON(w, http.StatusOK, resp)
}
