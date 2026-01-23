package api

import (
	"github.com/gorilla/mux"
	"github.com/sciffer/agentbox/pkg/proxy"
)

// NewRouter creates and configures the HTTP router
func NewRouter(handler *Handler, proxyHandler *proxy.Proxy) *mux.Router {
	r := mux.NewRouter()

	// API v1 routes
	api := r.PathPrefix("/api/v1").Subrouter()

	// Health check (no auth required)
	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Environment routes
	api.HandleFunc("/environments", handler.CreateEnvironment).Methods("POST")
	api.HandleFunc("/environments", handler.ListEnvironments).Methods("GET")
	api.HandleFunc("/environments/{id}", handler.GetEnvironment).Methods("GET")
	api.HandleFunc("/environments/{id}", handler.DeleteEnvironment).Methods("DELETE")
	api.HandleFunc("/environments/{id}/exec", handler.ExecuteCommand).Methods("POST")

	// WebSocket attachment endpoint
	api.HandleFunc("/environments/{id}/attach", handler.AttachWebSocket(proxyHandler)).Methods("GET")

	// Logs endpoint
	api.HandleFunc("/environments/{id}/logs", handler.GetLogs).Methods("GET")

	return r
}
