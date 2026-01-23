package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/users"
)

// Service handles authentication operations
type Service struct {
	db          *database.DB
	userService *users.Service
	jwtSecret   []byte
	logger      *zap.Logger
}

// GetUserService returns the user service (for access in handlers)
func (s *Service) GetUserService() *users.Service {
	return s.userService
}

// NewService creates a new auth service
func NewService(db *database.DB, userService *users.Service, logger *zap.Logger) *Service {
	secret := os.Getenv("AGENTBOX_JWT_SECRET")
	if secret == "" {
		secret = "change-me-in-production" // Default, should be set via env var
	}

	return &Service{
		db:          db,
		userService: userService,
		jwtSecret:   []byte(secret),
		logger:      logger,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// LoginRequest is the request to login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response from login
type LoginResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token,omitempty"`
	User         *users.User `json:"user"`
	ExpiresAt    time.Time   `json:"expires_at"`
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Get user with password hash
	user, passwordHash, err := s.userService.GetUserWithPassword(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if user.Status != "active" {
		return nil, fmt.Errorf("user account is not active")
	}

	// Verify password
	if passwordHash == "" {
		return nil, fmt.Errorf("user has no password set")
	}

	if !users.VerifyPassword(passwordHash, req.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	_ = s.userService.UpdateLastLogin(ctx, user.ID)

	// Generate JWT token
	token, expiresAt, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		Token:     token,
		User:      user,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateJWT validates a JWT token and returns the user
func (s *Service) ValidateJWT(ctx context.Context, tokenString string) (*users.User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// Get user from database
		user, err := s.userService.GetUserByID(ctx, claims.UserID)
		if err != nil {
			return nil, fmt.Errorf("user not found")
		}

		// Check if user is still active
		if user.Status != "active" {
			return nil, fmt.Errorf("user account is not active")
		}

		return user, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ValidateAPIKey validates an API key and returns the user
func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*users.User, error) {
	// Hash the provided API key
	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])

	// Look up in database
	var key struct {
		ID        string
		UserID    string
		ExpiresAt sql.NullTime
		RevokedAt sql.NullTime
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, expires_at, revoked_at
		FROM api_keys
		WHERE key_hash = $1
	`, keyHash).Scan(&key.ID, &key.UserID, &key.ExpiresAt, &key.RevokedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid API key")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	// Check if revoked
	if key.RevokedAt.Valid {
		return nil, fmt.Errorf("API key has been revoked")
	}

	// Check if expired
	if key.ExpiresAt.Valid && key.ExpiresAt.Time.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last_used timestamp
	_, _ = s.db.ExecContext(ctx, `
		UPDATE api_keys
		SET last_used = CURRENT_TIMESTAMP
		WHERE id = $1
	`, key.ID)

	// Get user
	user, err := s.userService.GetUserByID(ctx, key.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if user is active
	if user.Status != "active" {
		return nil, fmt.Errorf("user account is not active")
	}

	return user, nil
}

// generateToken generates a JWT token for a user
func (s *Service) generateToken(user *users.User) (string, time.Time, error) {
	expiryStr := os.Getenv("AGENTBOX_JWT_EXPIRY")
	if expiryStr == "" {
		expiryStr = "15m"
	}

	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		expiry = 15 * time.Minute
	}

	expiresAt := time.Now().Add(expiry)

	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "agentbox",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// CreateAPIKeyRequest is the request to create an API key
type CreateAPIKeyRequest struct {
	UserID      string
	Description string
	ExpiresAt   *time.Time
}

// APIKeyResponse is the response when creating an API key
type APIKeyResponse struct {
	ID          string     `json:"id"`
	Key         string     `json:"key"` // Only shown once on creation
	KeyPrefix   string     `json:"key_prefix"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKey creates a new API key for a user
func (s *Service) CreateAPIKey(ctx context.Context, req *CreateAPIKeyRequest) (*APIKeyResponse, error) {
	// Generate random API key
	keyLength := 32
	keyBytes := make([]byte, keyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	apiKey := hex.EncodeToString(keyBytes)
	keyPrefix := os.Getenv("AGENTBOX_API_KEY_PREFIX")
	if keyPrefix == "" {
		keyPrefix = "ak_live_"
	}
	fullKey := keyPrefix + apiKey

	// Hash the key
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	id := uuid.New().String()

	var expiresAt sql.NullTime
	if req.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *req.ExpiresAt, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, user_id, key_hash, key_prefix, description, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
	`, id, req.UserID, keyHash, keyPrefix, req.Description, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	var expiresAtPtr *time.Time
	if expiresAt.Valid {
		expiresAtPtr = &expiresAt.Time
	}

	return &APIKeyResponse{
		ID:          id,
		Key:         fullKey, // Only returned once
		KeyPrefix:   keyPrefix,
		Description: req.Description,
		ExpiresAt:   expiresAtPtr,
	}, nil
}

// ListAPIKeys lists API keys for a user
func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]*APIKeyInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, key_prefix, description, last_used, created_at, expires_at, revoked_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKeyInfo
	for rows.Next() {
		var key APIKeyInfo
		var lastUsed, expiresAt, revokedAt sql.NullTime

		err := rows.Scan(
			&key.ID, &key.KeyPrefix, &key.Description,
			&lastUsed, &key.CreatedAt, &expiresAt, &revokedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}

		if lastUsed.Valid {
			key.LastUsed = &lastUsed.Time
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if revokedAt.Valid {
			key.RevokedAt = &revokedAt.Time
		}

		keys = append(keys, &key)
	}

	return keys, nil
}

// APIKeyInfo represents API key information (without the actual key)
type APIKeyInfo struct {
	ID          string     `json:"id"`
	KeyPrefix   string     `json:"key_prefix"`
	Description string     `json:"description"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// RevokeAPIKey revokes an API key
func (s *Service) RevokeAPIKey(ctx context.Context, keyID, userID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, keyID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or already revoked")
	}

	return nil
}
