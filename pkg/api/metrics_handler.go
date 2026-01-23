package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/internal/logger"
	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/metrics"
	"github.com/sciffer/agentbox/pkg/models"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct {
	db     *database.DB
	logger *logger.Logger
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(db *database.DB, log *logger.Logger) *MetricsHandler {
	return &MetricsHandler{
		db:     db,
		logger: log,
	}
}

// GetGlobalMetrics handles GET /api/v1/metrics/global
func (h *MetricsHandler) GetGlobalMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	query := r.URL.Query()
	metricType := query.Get("type") // running_sandboxes, cpu_usage, memory_usage, start_time
	startStr := query.Get("start")
	endStr := query.Get("end")

	// Default time range: last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t
		}
	}

	// Default to running_sandboxes if not specified
	if metricType == "" {
		metricType = "running_sandboxes"
	}

	// Get metrics (envID is empty for global)
	metricList, err := metrics.GetMetrics(ctx, h.db, "", metricType, startTime, endTime)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get metrics", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"metrics": metricList,
		"type":    metricType,
		"start":   startTime,
		"end":     endTime,
	})
}

// GetEnvironmentMetrics handles GET /api/v1/metrics/environment/{id}
func (h *MetricsHandler) GetEnvironmentMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	envID := vars["id"]

	// Parse query parameters
	query := r.URL.Query()
	metricType := query.Get("type")
	startStr := query.Get("start")
	endStr := query.Get("end")

	// Default time range: last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t
		}
	}

	// Default to running_sandboxes if not specified
	if metricType == "" {
		metricType = "running_sandboxes"
	}

	// Get metrics for specific environment
	metricList, err := metrics.GetMetrics(ctx, h.db, envID, metricType, startTime, endTime)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "failed to get metrics", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"environment_id": envID,
		"metrics":        metricList,
		"type":           metricType,
		"start":          startTime,
		"end":            endTime,
	})
}

// Helper methods
func (h *MetricsHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
	}
}

func (h *MetricsHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
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
