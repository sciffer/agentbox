package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO required)
)

// DB wraps a database connection with driver information
type DB struct {
	*sql.DB
	driver string
	logger *zap.Logger
}

// NewDB creates a new database connection
// Uses PostgreSQL if AGENTBOX_DB_DSN is set, otherwise SQLite
func NewDB(logger *zap.Logger) (*DB, error) {
	dsn := os.Getenv("AGENTBOX_DB_DSN")
	dbPath := os.Getenv("AGENTBOX_DB_PATH")

	var db *sql.DB
	var driver string
	var err error

	if dsn != "" {
		// PostgreSQL
		db, err = sql.Open("postgres", dsn)
		driver = "postgres"
		if err != nil {
			return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
		}
		logger.Info("connected to PostgreSQL database")
	} else {
		// SQLite (default for development/testing)
		if dbPath == "" {
			dbPath = "./agentbox.db"
		}
		// modernc.org/sqlite uses "sqlite" as driver name and different pragma syntax
		db, err = sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
		driver = "sqlite"
		if err != nil {
			return nil, fmt.Errorf("failed to open SQLite database: %w", err)
		}
		logger.Info("connected to SQLite database", zap.String("path", dbPath))
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &DB{
		DB:     db,
		driver: driver,
		logger: logger,
	}

	// Run migrations
	if err := database.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return database, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	db.logger.Info("running database migrations")

	// Create schema_version table if it doesn't exist
	createVersionTable := `
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(createVersionTable); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current version
	var currentVersion int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	db.logger.Info("current schema version", zap.Int("version", currentVersion))

	// Run migrations
	migrations := getMigrations()
	for version, migration := range migrations {
		if version <= currentVersion {
			continue
		}

		db.logger.Info("applying migration", zap.Int("version", version))

		// Execute migration
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", version, err)
		}

		// Record version
		if _, err := db.Exec("INSERT INTO schema_version (version) VALUES ($1)", version); err != nil {
			return fmt.Errorf("failed to record migration version %d: %w", version, err)
		}

		db.logger.Info("migration applied successfully", zap.Int("version", version))
	}

	db.logger.Info("database migrations completed")
	return nil
}

// getMigrations returns a map of version -> SQL migration
func getMigrations() map[int]string {
	return map[int]string{
		1: initialSchema,
		2: apiKeyPermissionsSchema,
		3: environmentsAndExecutionsSchema,
	}
}

// apiKeyPermissionsSchema adds API key permissions table
const apiKeyPermissionsSchema = `
-- API Key Permissions table (for environment-scoped API keys)
CREATE TABLE IF NOT EXISTS api_key_permissions (
    id TEXT PRIMARY KEY,
    api_key_id TEXT NOT NULL,
    environment_id VARCHAR(255) NOT NULL,
    permission VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE CASCADE,
    UNIQUE(api_key_id, environment_id)
);

CREATE INDEX IF NOT EXISTS idx_api_key_perms_key_id ON api_key_permissions(api_key_id);
CREATE INDEX IF NOT EXISTS idx_api_key_perms_env_id ON api_key_permissions(environment_id);
`

// initialSchema is the initial database schema
const initialSchema = `
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash TEXT,
    role VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    google_id VARCHAR(255) UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    key_hash TEXT UNIQUE NOT NULL,
    key_prefix TEXT NOT NULL,
    description TEXT,
    last_used TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked ON api_keys(revoked_at);

-- Environment Permissions table
CREATE TABLE IF NOT EXISTS environment_permissions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    environment_id VARCHAR(255) NOT NULL,
    permission VARCHAR(50) NOT NULL,
    granted_by TEXT,
    granted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (granted_by) REFERENCES users(id),
    UNIQUE(user_id, environment_id)
);

CREATE INDEX IF NOT EXISTS idx_env_perms_user_id ON environment_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_env_perms_env_id ON environment_permissions(environment_id);

-- Metrics table
CREATE TABLE IF NOT EXISTS metrics (
    id TEXT PRIMARY KEY,
    environment_id VARCHAR(255),
    metric_type VARCHAR(50) NOT NULL,
    value REAL NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_env_id ON metrics(environment_id);
CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_env_type_time ON metrics(environment_id, metric_type, timestamp);
`

// environmentsAndExecutionsSchema adds tables for environments and executions
const environmentsAndExecutionsSchema = `
-- Environments table
CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    image TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    user_id TEXT,
    namespace TEXT NOT NULL,
    endpoint TEXT,
    timeout INTEGER,
    -- Resources (stored as JSON)
    resources_cpu TEXT NOT NULL,
    resources_memory TEXT NOT NULL,
    resources_storage TEXT NOT NULL,
    -- Optional fields (stored as JSON)
    env_vars TEXT,
    command TEXT,
    labels TEXT,
    node_selector TEXT,
    tolerations TEXT,
    isolation_config TEXT,
    pool_config TEXT
);

CREATE INDEX IF NOT EXISTS idx_environments_user_id ON environments(user_id);
CREATE INDEX IF NOT EXISTS idx_environments_status ON environments(status);
CREATE INDEX IF NOT EXISTS idx_environments_namespace ON environments(namespace);

-- Executions table
CREATE TABLE IF NOT EXISTS executions (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL,
    user_id TEXT,
    command TEXT NOT NULL,
    env_vars TEXT,
    status VARCHAR(50) NOT NULL,
    pod_name TEXT,
    namespace TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    queued_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    exit_code INTEGER,
    stdout TEXT,
    stderr TEXT,
    error TEXT,
    duration_ms BIGINT,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_executions_env_id ON executions(environment_id);
CREATE INDEX IF NOT EXISTS idx_executions_user_id ON executions(user_id);
CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);
CREATE INDEX IF NOT EXISTS idx_executions_created_at ON executions(created_at);
`
