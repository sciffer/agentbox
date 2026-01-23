package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sciffertbox/pkg/proxy"
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
		if env.Status != "running" {
			h.respondError(w, http.StatusBadRequest, "environment is not running", err)
			return
		}

		// Handle WebSocket upgrade and proxy to pod
		if err := proxyHandler.HandleWebSocket(w, r, env.Namespace, "main"); err != nil {
			h.logger.Error("websocket connection failed",
				"environment_id", envID,
				"error", err,
			)
			return
		}

		h.logger.Info("websocket attached",
			"environment_id", envID,
		)
	}
}
