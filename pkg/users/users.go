package users

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
)

// User status constants
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// User role constants
const (
	RoleUser       = "user"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// Service handles user operations
type Service struct {
	db     *database.DB
	logger *zap.Logger
}

// NewService creates a new user service
func NewService(db *database.DB, logger *zap.Logger) *Service {
	return &Service{
		db:     db,
		logger: logger,
	}
}

// User represents a user
type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     *string    `json:"email,omitempty"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	GoogleID  *string    `json:"google_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// CreateUserRequest is the request to create a user
type CreateUserRequest struct {
	Username string
	Email    string
	Password string
	Role     string
	Status   string
}

// EnsureDefaultAdmin ensures the default admin user exists
func (s *Service) EnsureDefaultAdmin(ctx context.Context) error {
	adminUsername := os.Getenv("AGENTBOX_ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "admin"
	}

	adminPassword := os.Getenv("AGENTBOX_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin" // Default, must be changed
	}

	adminEmail := os.Getenv("AGENTBOX_ADMIN_EMAIL")

	// Check if admin exists
	exists, err := s.userExists(ctx, adminUsername)
	if err != nil {
		return fmt.Errorf("failed to check if admin exists: %w", err)
	}

	if !exists {
		s.logger.Info("creating default admin user", zap.String("username", adminUsername))
		_, err := s.CreateUser(ctx, &CreateUserRequest{
			Username: adminUsername,
			Email:    adminEmail,
			Password: adminPassword,
			Role:     RoleSuperAdmin,
			Status:   StatusActive,
		})
		if err != nil {
			return fmt.Errorf("failed to create default admin: %w", err)
		}
		s.logger.Info("default admin user created", zap.String("username", adminUsername))
	}

	return nil
}

// userExists checks if a user exists
func (s *Service) userExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = $1", username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateUser creates a new user
func (s *Service) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	id := uuid.New().String()

	var passwordHash sql.NullString
	if req.Password != "" {
		hash, err := hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		passwordHash = sql.NullString{String: hash, Valid: true}
	}

	var email sql.NullString
	if req.Email != "" {
		email = sql.NullString{String: req.Email, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, email, password_hash, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, id, req.Username, email, passwordHash, req.Role, req.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	var dbUser database.User
	var email, googleID sql.NullString
	var lastLogin sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, role, status, google_id, created_at, updated_at, last_login
		FROM users
		WHERE id = $1
	`, id).Scan(
		&dbUser.ID, &dbUser.Username, &email, &dbUser.PasswordHash,
		&dbUser.Role, &dbUser.Status, &googleID, &dbUser.CreatedAt,
		&dbUser.UpdatedAt, &lastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user := &User{
		ID:        dbUser.ID,
		Username:  dbUser.Username,
		Role:      dbUser.Role,
		Status:    dbUser.Status,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}

	if email.Valid {
		user.Email = &email.String
	}
	if googleID.Valid {
		user.GoogleID = &googleID.String
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetUserByUsername retrieves a user by username
func (s *Service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var dbUser database.User
	var email, googleID sql.NullString
	var lastLogin sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, role, status, google_id, created_at, updated_at, last_login
		FROM users
		WHERE username = $1
	`, username).Scan(
		&dbUser.ID, &dbUser.Username, &email, &dbUser.PasswordHash,
		&dbUser.Role, &dbUser.Status, &googleID, &dbUser.CreatedAt,
		&dbUser.UpdatedAt, &lastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user := &User{
		ID:        dbUser.ID,
		Username:  dbUser.Username,
		Role:      dbUser.Role,
		Status:    dbUser.Status,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}

	if email.Valid {
		user.Email = &email.String
	}
	if googleID.Valid {
		user.GoogleID = &googleID.String
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetUserWithPassword retrieves a user with password hash for authentication
func (s *Service) GetUserWithPassword(ctx context.Context, username string) (*User, string, error) {
	var dbUser database.User
	var email, googleID, passwordHash sql.NullString
	var lastLogin sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, role, status, google_id, created_at, updated_at, last_login
		FROM users
		WHERE username = $1
	`, username).Scan(
		&dbUser.ID, &dbUser.Username, &email, &passwordHash,
		&dbUser.Role, &dbUser.Status, &googleID, &dbUser.CreatedAt,
		&dbUser.UpdatedAt, &lastLogin,
	)
	if err == sql.ErrNoRows {
		return nil, "", fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	user := &User{
		ID:        dbUser.ID,
		Username:  dbUser.Username,
		Role:      dbUser.Role,
		Status:    dbUser.Status,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
	}

	if email.Valid {
		user.Email = &email.String
	}
	if googleID.Valid {
		user.GoogleID = &googleID.String
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	hash := ""
	if passwordHash.Valid {
		hash = passwordHash.String
	}

	return user, hash, nil
}

// UpdateLastLogin updates the user's last login timestamp
func (s *Service) UpdateLastLogin(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET last_login = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, userID)
	return err
}

// ListUsers lists all users with optional filtering
func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, email, role, status, google_id, created_at, updated_at, last_login
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var dbUser database.User
		var email, googleID sql.NullString
		var lastLogin sql.NullTime

		err := rows.Scan(
			&dbUser.ID, &dbUser.Username, &email, &dbUser.Role,
			&dbUser.Status, &googleID, &dbUser.CreatedAt,
			&dbUser.UpdatedAt, &lastLogin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user := &User{
			ID:        dbUser.ID,
			Username:  dbUser.Username,
			Role:      dbUser.Role,
			Status:    dbUser.Status,
			CreatedAt: dbUser.CreatedAt,
			UpdatedAt: dbUser.UpdatedAt,
		}

		if email.Valid {
			user.Email = &email.String
		}
		if googleID.Valid {
			user.GoogleID = &googleID.String
		}
		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// UpdatePassword updates a user's password
func (s *Service) UpdatePassword(ctx context.Context, userID, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`, hash, userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateUserRequest is the request to update a user
type UpdateUserRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
	Role     *string `json:"role,omitempty"`
	Status   *string `json:"status,omitempty"`
	Password *string `json:"password,omitempty"`
}

// UpdateUser updates a user's information
func (s *Service) UpdateUser(ctx context.Context, userID string, req *UpdateUserRequest) (*User, error) {
	// Verify user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Username != nil {
		updates = append(updates, fmt.Sprintf("username = $%d", argIdx))
		args = append(args, *req.Username)
		argIdx++
	}

	if req.Email != nil {
		if *req.Email == "" {
			updates = append(updates, fmt.Sprintf("email = $%d", argIdx))
			args = append(args, sql.NullString{})
		} else {
			updates = append(updates, fmt.Sprintf("email = $%d", argIdx))
			args = append(args, *req.Email)
		}
		argIdx++
	}

	if req.Role != nil {
		// Validate role
		if *req.Role != RoleUser && *req.Role != RoleAdmin && *req.Role != RoleSuperAdmin {
			return nil, fmt.Errorf("invalid role: %s", *req.Role)
		}
		updates = append(updates, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *req.Role)
		argIdx++
	}

	if req.Status != nil {
		// Validate status
		if *req.Status != StatusActive && *req.Status != StatusInactive {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}
		updates = append(updates, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *req.Status)
		argIdx++
	}

	if req.Password != nil && *req.Password != "" {
		hash, err := hashPassword(*req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		updates = append(updates, fmt.Sprintf("password_hash = $%d", argIdx))
		args = append(args, hash)
		argIdx++
	}

	if len(updates) == 0 {
		return s.GetUserByID(ctx, userID)
	}

	// Always update the updated_at timestamp
	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	// Build and execute query
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		joinStrings(updates, ", "), argIdx)
	args = append(args, userID)

	_, err = s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return s.GetUserByID(ctx, userID)
}

// DeleteUser deletes a user by ID
// Note: This will cascade delete all related records (API keys, permissions)
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	// Verify user exists
	_, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	s.logger.Info("user deleted", zap.String("user_id", userID))
	return nil
}

// GetUserCount returns the total number of users
func (s *Service) GetUserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// joinStrings joins strings with a separator (simple helper to avoid importing strings package)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
