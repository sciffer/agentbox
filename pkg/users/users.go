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
			Role:     "super_admin",
			Status:   "active",
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

