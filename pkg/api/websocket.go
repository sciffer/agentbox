package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/models"
	"github.com/sciffer/agentbox/pkg/proxy"
)

// AttachWebSocket handles WebSocket attachment to an environment
func (h *Handler) AttachWebSocket(proxyHandler *proxy.Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		vars := mux.Vars(r)
		envID := vars["id"]

		// Get environment
		env, err := h.orchestrator.GetEnvironment(ctx, envID)
		if err != nil {
			h.respondError(w, http.StatusNotFound, "environment not found", err)
			return
		}

		// Check if environment is running
		if env.Status != models.StatusRunning {
			h.respondError(w, http.StatusBadRequest, "environment is not running", fmt.Errorf("environment status is %s", env.Status))
			return
		}

		// Handle WebSocket upgrade and proxy to pod
		if err := proxyHandler.HandleWebSocket(w, r, env.Namespace, "main"); err != nil {
			h.logger.Error("websocket connection failed",
				zap.String("environment_id", envID),
				zap.Error(err),
			)
			return
		}

		h.logger.Info("websocket attached",
			zap.String("environment_id", envID),
		)
	}
}
