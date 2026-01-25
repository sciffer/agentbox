package auth

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/users"
)

// ContextKey is the type for context keys
type ContextKey string

// UserContextKey is the context key for user
const UserContextKey ContextKey = "user"

// Middleware provides authentication middleware for HTTP handlers
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for X-API-Key header first (common pattern for API key auth)
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			user, err := s.ValidateAPIKey(r.Context(), apiKey)
			if err != nil {
				s.logger.Debug("API key authentication failed", zap.Error(err))
				s.respondUnauthorized(w, "invalid API key")
				return
			}

			// API key valid, set user in context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.respondUnauthorized(w, "missing authorization header")
			return
		}

		// Extract token/key
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			s.respondUnauthorized(w, "invalid authorization header format")
			return
		}

		token := parts[1]

		// Try JWT first
		user, err := s.ValidateJWT(r.Context(), token)
		if err == nil {
			// JWT valid, set user in context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try API key via Authorization header (Bearer <api-key>)
		user, err = s.ValidateAPIKey(r.Context(), token)
		if err != nil {
			s.logger.Debug("authentication failed", zap.Error(err))
			s.respondUnauthorized(w, "invalid token or API key")
			return
		}

		// API key valid, set user in context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts the user from the request context
func GetUserFromContext(ctx context.Context) (*users.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*users.User)
	return user, ok
}

// respondUnauthorized sends an unauthorized response
func (s *Service) respondUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if _, err := w.Write([]byte(`{"error":"unauthorized","message":"` + message + `"}`)); err != nil {
		s.logger.Warn("failed to write unauthorized response", zap.Error(err))
	}
}
