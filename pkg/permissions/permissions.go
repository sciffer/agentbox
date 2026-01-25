package permissions

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

// Permission levels
const (
	PermissionViewer = "viewer"
	PermissionEditor = "editor"
	PermissionOwner  = "owner"
)

// PermissionLevel returns the numeric level for permission comparison
func PermissionLevel(permission string) int {
	switch permission {
	case PermissionViewer:
		return 1
	case PermissionEditor:
		return 2
	case PermissionOwner:
		return 3
	default:
		return 0
	}
}

// ValidatePermission checks if a permission level is valid
func ValidatePermission(permission string) bool {
	return permission == PermissionViewer ||
		permission == PermissionEditor ||
		permission == PermissionOwner
}

// EnvironmentPermission represents a user's permission for an environment
type EnvironmentPermission struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	EnvironmentID string    `json:"environment_id"`
	Permission    string    `json:"permission"`
	GrantedBy     *string   `json:"granted_by,omitempty"`
	GrantedAt     time.Time `json:"granted_at"`
}

// APIKeyPermission represents an API key's permission for an environment
type APIKeyPermission struct {
	ID            string    `json:"id"`
	APIKeyID      string    `json:"api_key_id"`
	EnvironmentID string    `json:"environment_id"`
	Permission    string    `json:"permission"`
	CreatedAt     time.Time `json:"created_at"`
}

// Service handles permission operations
type Service struct {
	db     *database.DB
	logger *zap.Logger
}

// NewService creates a new permission service
func NewService(db *database.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// ListUserPermissions returns all environment permissions for a user
func (s *Service) ListUserPermissions(ctx context.Context, userID string) ([]*EnvironmentPermission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, environment_id, permission, granted_by, granted_at
		FROM environment_permissions
		WHERE user_id = $1
		ORDER BY granted_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*EnvironmentPermission
	for rows.Next() {
		var perm EnvironmentPermission
		var grantedBy sql.NullString

		err := rows.Scan(&perm.ID, &perm.UserID, &perm.EnvironmentID, &perm.Permission, &grantedBy, &perm.GrantedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}

		if grantedBy.Valid {
			perm.GrantedBy = &grantedBy.String
		}

		permissions = append(permissions, &perm)
	}

	return permissions, nil
}

// GetUserPermission returns a user's permission for a specific environment
func (s *Service) GetUserPermission(ctx context.Context, userID, environmentID string) (*EnvironmentPermission, error) {
	var perm EnvironmentPermission
	var grantedBy sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, environment_id, permission, granted_by, granted_at
		FROM environment_permissions
		WHERE user_id = $1 AND environment_id = $2
	`, userID, environmentID).Scan(&perm.ID, &perm.UserID, &perm.EnvironmentID, &perm.Permission, &grantedBy, &perm.GrantedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No permission found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user permission: %w", err)
	}

	if grantedBy.Valid {
		perm.GrantedBy = &grantedBy.String
	}

	return &perm, nil
}

// GrantPermission grants a user permission to an environment
func (s *Service) GrantPermission(ctx context.Context, userID, environmentID, permission, grantedByUserID string) (*EnvironmentPermission, error) {
	if !ValidatePermission(permission) {
		return nil, fmt.Errorf("invalid permission level: %s", permission)
	}

	id := uuid.New().String()

	var grantedBy sql.NullString
	if grantedByUserID != "" {
		grantedBy = sql.NullString{String: grantedByUserID, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO environment_permissions (id, user_id, environment_id, permission, granted_by, granted_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id, environment_id) DO UPDATE SET
			permission = EXCLUDED.permission,
			granted_by = EXCLUDED.granted_by,
			granted_at = CURRENT_TIMESTAMP
	`, id, userID, environmentID, permission, grantedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to grant permission: %w", err)
	}

	s.logger.Info("permission granted",
		zap.String("user_id", userID),
		zap.String("environment_id", environmentID),
		zap.String("permission", permission),
	)

	return s.GetUserPermission(ctx, userID, environmentID)
}

// UpdatePermission updates a user's permission level for an environment
func (s *Service) UpdatePermission(ctx context.Context, userID, environmentID, permission string) (*EnvironmentPermission, error) {
	if !ValidatePermission(permission) {
		return nil, fmt.Errorf("invalid permission level: %s", permission)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE environment_permissions
		SET permission = $1
		WHERE user_id = $2 AND environment_id = $3
	`, permission, userID, environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to update permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("permission not found")
	}

	s.logger.Info("permission updated",
		zap.String("user_id", userID),
		zap.String("environment_id", environmentID),
		zap.String("permission", permission),
	)

	return s.GetUserPermission(ctx, userID, environmentID)
}

// RevokePermission removes a user's permission for an environment
func (s *Service) RevokePermission(ctx context.Context, userID, environmentID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM environment_permissions
		WHERE user_id = $1 AND environment_id = $2
	`, userID, environmentID)
	if err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("permission not found")
	}

	s.logger.Info("permission revoked",
		zap.String("user_id", userID),
		zap.String("environment_id", environmentID),
	)

	return nil
}

// CheckAccess verifies if a user has at least the required permission level for an environment
// Returns true if the user has access, false otherwise
// Super admins always have access
func (s *Service) CheckAccess(ctx context.Context, user *users.User, environmentID string, requiredPermission string) (bool, error) {
	// Super admins have access to everything
	if user.Role == users.RoleSuperAdmin {
		return true, nil
	}

	perm, err := s.GetUserPermission(ctx, user.ID, environmentID)
	if err != nil {
		return false, err
	}

	if perm == nil {
		return false, nil
	}

	// Check if user's permission level is >= required level
	userLevel := PermissionLevel(perm.Permission)
	requiredLevel := PermissionLevel(requiredPermission)

	return userLevel >= requiredLevel, nil
}

// ListEnvironmentPermissions returns all permissions for an environment
func (s *Service) ListEnvironmentPermissions(ctx context.Context, environmentID string) ([]*EnvironmentPermission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, environment_id, permission, granted_by, granted_at
		FROM environment_permissions
		WHERE environment_id = $1
		ORDER BY granted_at DESC
	`, environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list environment permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*EnvironmentPermission
	for rows.Next() {
		var perm EnvironmentPermission
		var grantedBy sql.NullString

		err := rows.Scan(&perm.ID, &perm.UserID, &perm.EnvironmentID, &perm.Permission, &grantedBy, &perm.GrantedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}

		if grantedBy.Valid {
			perm.GrantedBy = &grantedBy.String
		}

		permissions = append(permissions, &perm)
	}

	return permissions, nil
}

// API Key Permissions

// ListAPIKeyPermissions returns all environment permissions for an API key
func (s *Service) ListAPIKeyPermissions(ctx context.Context, apiKeyID string) ([]*APIKeyPermission, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, api_key_id, environment_id, permission, created_at
		FROM api_key_permissions
		WHERE api_key_id = $1
		ORDER BY created_at DESC
	`, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API key permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*APIKeyPermission
	for rows.Next() {
		var perm APIKeyPermission
		err := rows.Scan(&perm.ID, &perm.APIKeyID, &perm.EnvironmentID, &perm.Permission, &perm.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key permission: %w", err)
		}
		permissions = append(permissions, &perm)
	}

	return permissions, nil
}

// GrantAPIKeyPermission grants an API key permission to an environment
func (s *Service) GrantAPIKeyPermission(ctx context.Context, apiKeyID, environmentID, permission string) (*APIKeyPermission, error) {
	if !ValidatePermission(permission) {
		return nil, fmt.Errorf("invalid permission level: %s", permission)
	}

	id := uuid.New().String()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_key_permissions (id, api_key_id, environment_id, permission, created_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (api_key_id, environment_id) DO UPDATE SET
			permission = EXCLUDED.permission
	`, id, apiKeyID, environmentID, permission)
	if err != nil {
		return nil, fmt.Errorf("failed to grant API key permission: %w", err)
	}

	return s.GetAPIKeyPermission(ctx, apiKeyID, environmentID)
}

// GetAPIKeyPermission returns an API key's permission for a specific environment
func (s *Service) GetAPIKeyPermission(ctx context.Context, apiKeyID, environmentID string) (*APIKeyPermission, error) {
	var perm APIKeyPermission

	err := s.db.QueryRowContext(ctx, `
		SELECT id, api_key_id, environment_id, permission, created_at
		FROM api_key_permissions
		WHERE api_key_id = $1 AND environment_id = $2
	`, apiKeyID, environmentID).Scan(&perm.ID, &perm.APIKeyID, &perm.EnvironmentID, &perm.Permission, &perm.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get API key permission: %w", err)
	}

	return &perm, nil
}

// RevokeAPIKeyPermission removes an API key's permission for an environment
func (s *Service) RevokeAPIKeyPermission(ctx context.Context, apiKeyID, environmentID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM api_key_permissions
		WHERE api_key_id = $1 AND environment_id = $2
	`, apiKeyID, environmentID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("permission not found")
	}

	return nil
}

// SetAPIKeyPermissions sets all permissions for an API key (replaces existing)
func (s *Service) SetAPIKeyPermissions(ctx context.Context, apiKeyID string, permissions []struct {
	EnvironmentID string
	Permission    string
}) error {
	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			//nolint:errcheck // Best effort rollback on error path, error is already being returned
			tx.Rollback()
		}
	}()

	// Delete existing permissions
	_, err = tx.ExecContext(ctx, "DELETE FROM api_key_permissions WHERE api_key_id = $1", apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to delete existing permissions: %w", err)
	}

	// Insert new permissions
	for _, p := range permissions {
		if !ValidatePermission(p.Permission) {
			return fmt.Errorf("invalid permission level: %s", p.Permission)
		}

		id := uuid.New().String()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO api_key_permissions (id, api_key_id, environment_id, permission, created_at)
			VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		`, id, apiKeyID, p.EnvironmentID, p.Permission)
		if err != nil {
			return fmt.Errorf("failed to insert permission: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CheckAPIKeyAccess verifies if an API key has at least the required permission level for an environment
func (s *Service) CheckAPIKeyAccess(ctx context.Context, apiKeyID, environmentID, requiredPermission string) (bool, error) {
	perm, err := s.GetAPIKeyPermission(ctx, apiKeyID, environmentID)
	if err != nil {
		return false, err
	}

	if perm == nil {
		return false, nil
	}

	keyLevel := PermissionLevel(perm.Permission)
	requiredLevel := PermissionLevel(requiredPermission)

	return keyLevel >= requiredLevel, nil
}
