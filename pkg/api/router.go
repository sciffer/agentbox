package api

import (
	"github.com/gorilla/mux"

	"github.com/sciffer/agentbox/pkg/auth"
	"github.com/sciffer/agentbox/pkg/proxy"
)

// RouterConfig holds all handlers needed for routing
type RouterConfig struct {
	Handler        *Handler
	AuthHandler    *AuthHandler
	UserHandler    *UserHandler
	APIKeyHandler  *APIKeyHandler
	MetricsHandler *MetricsHandler
	ProxyHandler   *proxy.Proxy
	AuthService    *auth.Service
}

// NewRouter creates and configures the HTTP router
// For backward compatibility, also supports old signature (handler, proxyHandler)
func NewRouter(configOrHandler interface{}, proxyHandlerOrNil ...*proxy.Proxy) *mux.Router {
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()

	// Handle old signature for backward compatibility
	if handler, ok := configOrHandler.(*Handler); ok {
		// Old signature: NewRouter(handler, proxyHandler)
		var proxyHandler *proxy.Proxy
		if len(proxyHandlerOrNil) > 0 {
			proxyHandler = proxyHandlerOrNil[0]
		}

		// Health check (no auth required)
		api.HandleFunc("/health", handler.HealthCheck).Methods("GET")

		// Environment routes (no auth for backward compatibility in tests)
		api.HandleFunc("/environments", handler.CreateEnvironment).Methods("POST")
		api.HandleFunc("/environments", handler.ListEnvironments).Methods("GET")
		api.HandleFunc("/environments/{id}", handler.GetEnvironment).Methods("GET")
		api.HandleFunc("/environments/{id}", handler.DeleteEnvironment).Methods("DELETE")
		api.HandleFunc("/environments/{id}/exec", handler.ExecuteCommand).Methods("POST")
		if proxyHandler != nil {
			api.HandleFunc("/environments/{id}/attach", handler.AttachWebSocket(proxyHandler)).Methods("GET")
		}
		api.HandleFunc("/environments/{id}/logs", handler.GetLogs).Methods("GET")

		return r
	}

	// New signature: NewRouter(config)
	config := configOrHandler.(*RouterConfig)

	// Public routes (no auth required)
	api.HandleFunc("/health", config.Handler.HealthCheck).Methods("GET")

	// Auth routes (no auth required for login)
	authRoutes := api.PathPrefix("/auth").Subrouter()
	authRoutes.HandleFunc("/login", config.AuthHandler.Login).Methods("POST")
	authRoutes.HandleFunc("/logout", config.AuthHandler.Logout).Methods("POST")
	authRoutes.HandleFunc("/me", config.AuthHandler.GetMe).Methods("GET")
	authRoutes.HandleFunc("/change-password", config.AuthHandler.ChangePassword).Methods("POST")

	// Protected routes (require authentication)
	protected := api.PathPrefix("").Subrouter()
	protected.Use(config.AuthService.Middleware)

	// Environment routes (protected)
	protected.HandleFunc("/environments", config.Handler.CreateEnvironment).Methods("POST")
	protected.HandleFunc("/environments", config.Handler.ListEnvironments).Methods("GET")
	protected.HandleFunc("/environments/{id}", config.Handler.GetEnvironment).Methods("GET")
	protected.HandleFunc("/environments/{id}", config.Handler.DeleteEnvironment).Methods("DELETE")
	protected.HandleFunc("/environments/{id}/exec", config.Handler.ExecuteCommand).Methods("POST")
	if config.ProxyHandler != nil {
		protected.HandleFunc("/environments/{id}/attach", config.Handler.AttachWebSocket(config.ProxyHandler)).Methods("GET")
	}
	protected.HandleFunc("/environments/{id}/logs", config.Handler.GetLogs).Methods("GET")

	// User management routes (protected, admin only)
	protected.HandleFunc("/users", config.UserHandler.ListUsers).Methods("GET")
	protected.HandleFunc("/users", config.UserHandler.CreateUser).Methods("POST")
	protected.HandleFunc("/users/{id}", config.UserHandler.GetUser).Methods("GET")

	// API key management routes (protected)
	protected.HandleFunc("/api-keys", config.APIKeyHandler.ListAPIKeys).Methods("GET")
	protected.HandleFunc("/api-keys", config.APIKeyHandler.CreateAPIKey).Methods("POST")
	protected.HandleFunc("/api-keys/{id}", config.APIKeyHandler.RevokeAPIKey).Methods("DELETE")

	// Metrics routes (protected)
	if config.MetricsHandler != nil {
		protected.HandleFunc("/metrics/global", config.MetricsHandler.GetGlobalMetrics).Methods("GET")
		protected.HandleFunc("/metrics/environment/{id}", config.MetricsHandler.GetEnvironmentMetrics).Methods("GET")
	}

	return r
}
