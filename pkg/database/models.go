package database

import (
	"database/sql"
	"time"
)

// User represents a user in the database
type User struct {
	ID           string
	Username     string
	Email        sql.NullString
	PasswordHash sql.NullString
	Role         string
	Status       string
	GoogleID     sql.NullString
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLogin    sql.NullTime
}

// APIKey represents an API key in the database
type APIKey struct {
	ID          string
	UserID      string
	KeyHash     string
	KeyPrefix   string
	Description sql.NullString
	LastUsed    sql.NullTime
	CreatedAt   time.Time
	ExpiresAt   sql.NullTime
	RevokedAt   sql.NullTime
}

// EnvironmentPermission represents environment access permissions
type EnvironmentPermission struct {
	ID            string
	UserID        string
	EnvironmentID string
	Permission    string // owner, editor, viewer
	GrantedBy     sql.NullString
	GrantedAt     time.Time
}

// Metric represents a metric data point
type Metric struct {
	ID            string
	EnvironmentID sql.NullString
	MetricType    string // running_sandboxes, cpu_usage, memory_usage, start_time
	Value         float64
	Timestamp     time.Time
}
