# Backend Implementation Plan for UI Features

## Overview

This document outlines the backend implementation required to support the UI features, including authentication, user management, API keys, database integration, and metrics collection.

## 1. Database Integration

### Database Package Structure

```
pkg/database/
├── database.go          # Database connection and initialization
├── migrations/          # SQL migration files
│   ├── 0001_initial.up.sql
│   ├── 0001_initial.down.sql
│   ├── 0002_add_metrics.up.sql
│   └── ...
├── schema.go            # Schema validation and migration logic
└── models.go            # Database models (structs)
```

### Database Initialization

**Location:** `pkg/database/database.go`

**Responsibilities:**
- Connect to database (SQLite or PostgreSQL based on DSN)
- Run schema migrations on startup
- Validate schema version
- Handle connection pooling
- Provide database interface for other packages

**Implementation:**
```go
package database

import (
    "database/sql"
    "fmt"
    "os"
    
    _ "github.com/lib/pq"           // PostgreSQL driver
    _ "github.com/mattn/go-sqlite3" // SQLite driver
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    "github.com/golang-migrate/migrate/v4/database/sqlite3"
    "github.com/golang-migrate/migrate/v4/source/iofs"
)

type DB struct {
    *sql.DB
    driver string
}

func NewDB() (*DB, error) {
    dsn := os.Getenv("AGENTBOX_DB_DSN")
    dbPath := os.Getenv("AGENTBOX_DB_PATH")
    
    var db *sql.DB
    var driver string
    var err error
    
    if dsn != "" {
        // PostgreSQL
        db, err = sql.Open("postgres", dsn)
        driver = "postgres"
    } else {
        // SQLite (default)
        if dbPath == "" {
            dbPath = "./agentbox.db"
        }
        db, err = sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
        driver = "sqlite3"
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    database := &DB{DB: db, driver: driver}
    
    // Run migrations
    if err := database.Migrate(); err != nil {
        return nil, fmt.Errorf("failed to migrate database: %w", err)
    }
    
    return database, nil
}

func (db *DB) Migrate() error {
    // Use golang-migrate to run migrations
    // This will validate and update schema on every startup
    // ...
}
```

### Schema Migration Strategy

**On Startup:**
1. Check current schema version
2. Compare with expected version (from migration files)
3. Run pending migrations if needed
4. Validate schema integrity
5. Log migration results

**Migration Files:**
- Use `golang-migrate` format
- Versioned migrations (0001, 0002, etc.)
- Up and down migrations for rollback support
- Embedded in binary using `embed` package

## 2. Authentication System

### Authentication Package

```
pkg/auth/
├── auth.go              # Main authentication logic
├── jwt.go               # JWT token generation/validation
├── apikey.go            # API key validation
├── password.go          # Password hashing/validation
└── middleware.go        # Authentication middleware
```

### API Key Authentication

**Implementation:**
```go
package auth

import (
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "fmt"
    "time"
)

type APIKeyAuth struct {
    db *database.DB
}

func (a *APIKeyAuth) ValidateAPIKey(ctx context.Context, apiKey string) (*User, error) {
    // Hash the provided API key
    hash := sha256.Sum256([]byte(apiKey))
    keyHash := hex.EncodeToString(hash[:])
    
    // Look up in database
    var key struct {
        ID        string
        UserID    string
        KeyHash   string
        ExpiresAt sql.NullTime
        RevokedAt sql.NullTime
    }
    
    err := a.db.QueryRowContext(ctx, `
        SELECT id, user_id, key_hash, expires_at, revoked_at
        FROM api_keys
        WHERE key_hash = $1
    `, keyHash).Scan(&key.ID, &key.UserID, &key.KeyHash, &key.ExpiresAt, &key.RevokedAt)
    
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("invalid API key")
    }
    if err != nil {
        return nil, err
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
    a.db.ExecContext(ctx, `
        UPDATE api_keys
        SET last_used = CURRENT_TIMESTAMP
        WHERE id = $1
    `, key.ID)
    
    // Get user
    return a.getUser(ctx, key.UserID)
}
```

### Authentication Middleware

**Implementation:**
```go
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            h.respondError(w, http.StatusUnauthorized, "missing authorization header", nil)
            return
        }
        
        // Extract token/key
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            h.respondError(w, http.StatusUnauthorized, "invalid authorization header", nil)
            return
        }
        
        token := parts[1]
        
        // Try JWT first
        user, err := h.auth.ValidateJWT(r.Context(), token)
        if err == nil {
            // JWT valid, set user in context
            ctx := context.WithValue(r.Context(), "user", user)
            next.ServeHTTP(w, r.WithContext(ctx))
            return
        }
        
        // Try API key
        user, err = h.auth.ValidateAPIKey(r.Context(), token)
        if err != nil {
            h.respondError(w, http.StatusUnauthorized, "invalid token or API key", err)
            return
        }
        
        // API key valid, set user in context
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## 3. User Management

### User Package

```
pkg/users/
├── users.go             # User CRUD operations
├── permissions.go       # Permission management
└── roles.go             # Role definitions
```

### Default Admin User Creation

**On First Startup:**
```go
func (u *UserService) EnsureDefaultAdmin(ctx context.Context) error {
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
    exists, err := u.userExists(ctx, adminUsername)
    if err != nil {
        return err
    }
    
    if !exists {
        // Create default admin
        _, err := u.CreateUser(ctx, &CreateUserRequest{
            Username: adminUsername,
            Email:    adminEmail,
            Password: adminPassword,
            Role:     "super_admin",
            Status:   "active",
        })
        return err
    }
    
    return nil
}
```

## 4. Metrics Collection

### Metrics Package

```
pkg/metrics/
├── collector.go         # Metrics collection logic
├── storage.go           # Database storage
└── types.go             # Metric types
```

### Metrics Collection

**Implementation:**
```go
package metrics

import (
    "context"
    "time"
)

type Collector struct {
    db        *database.DB
    interval  time.Duration
    enabled   bool
    stopChan  chan struct{}
}

func NewCollector(db *database.DB) *Collector {
    enabled := os.Getenv("AGENTBOX_METRICS_ENABLED") != "false"
    intervalStr := os.Getenv("AGENTBOX_METRICS_COLLECTION_INTERVAL")
    interval := 30 * time.Second
    if intervalStr != "" {
        if d, err := time.ParseDuration(intervalStr); err == nil {
            interval = d
        }
    }
    
    return &Collector{
        db:       db,
        interval: interval,
        enabled:  enabled,
        stopChan: make(chan struct{}),
    }
}

func (c *Collector) Start(ctx context.Context) {
    if !c.enabled {
        return
    }
    
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.collectMetrics(ctx)
        case <-c.stopChan:
            return
        case <-ctx.Done():
            return
        }
    }
}

func (c *Collector) collectMetrics(ctx context.Context) {
    // Collect global metrics
    c.collectGlobalMetrics(ctx)
    
    // Collect per-environment metrics
    c.collectEnvironmentMetrics(ctx)
}

func (c *Collector) collectGlobalMetrics(ctx context.Context) {
    // Count running environments
    var runningCount int
    c.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM environments WHERE status = 'running'
    `).Scan(&runningCount)
    
    c.storeMetric(ctx, "", "running_sandboxes", float64(runningCount))
    
    // Calculate total CPU usage
    // Calculate total memory usage
    // Calculate average start time
    // ...
}
```

### Metrics Storage

**Store metrics with timestamps:**
```go
func (c *Collector) storeMetric(ctx context.Context, envID string, metricType string, value float64) error {
    _, err := c.db.ExecContext(ctx, `
        INSERT INTO metrics (environment_id, metric_type, value, timestamp)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
    `, envID, metricType, value)
    return err
}
```

### Metrics Retrieval API

**Handler implementation:**
```go
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
    query := r.URL.Query()
    envID := query.Get("env_id")
    metricType := query.Get("type") // running_sandboxes, cpu_usage, etc.
    start := query.Get("start")
    end := query.Get("end")
    
    // Parse time range
    startTime, _ := time.Parse(time.RFC3339, start)
    endTime, _ := time.Parse(time.RFC3339, end)
    
    // Query metrics from database
    rows, err := h.metrics.GetMetrics(r.Context(), envID, metricType, startTime, endTime)
    if err != nil {
        h.respondError(w, http.StatusInternalServerError, "failed to get metrics", err)
        return
    }
    
    h.respondJSON(w, http.StatusOK, rows)
}
```

## 5. Service Integration

### Main Service Updates

**In `cmd/server/main.go`:**
```go
func main() {
    // ... existing code ...
    
    // Initialize database
    db, err := database.NewDB()
    if err != nil {
        log.Fatal("failed to initialize database", zap.Error(err))
    }
    defer db.Close()
    
    // Ensure default admin user
    userService := users.NewUserService(db)
    if err := userService.EnsureDefaultAdmin(context.Background()); err != nil {
        log.Fatal("failed to create default admin", zap.Error(err))
    }
    
    // Start metrics collector
    metricsCollector := metrics.NewCollector(db)
    go metricsCollector.Start(context.Background())
    
    // Initialize auth service
    authService := auth.NewAuthService(db, jwtSecret)
    
    // Update handler to use auth
    handler := api.NewHandler(orch, validator, logger, authService, userService, metricsService)
    
    // Add auth middleware to protected routes
    router.Use(handler.AuthMiddleware)
    
    // ... rest of server setup ...
}
```

## 6. Configuration Updates

### Config Package Updates

**Add to `internal/config/config.go`:**
```go
type Config struct {
    // ... existing fields ...
    
    Database struct {
        DSN  string `yaml:"dsn"`  // PostgreSQL DSN
        Path string `yaml:"path"` // SQLite path
    } `yaml:"database"`
    
    Auth struct {
        AdminUsername string `yaml:"admin_username"`
        AdminPassword string `yaml:"admin_password"`
        AdminEmail    string `yaml:"admin_email"`
        JWTSecret     string `yaml:"jwt_secret"`
        JWTExpiry     string `yaml:"jwt_expiry"`
    } `yaml:"auth"`
    
    GoogleOAuth struct {
        ClientID     string `yaml:"client_id"`
        ClientSecret string `yaml:"client_secret"`
        RedirectURL  string `yaml:"redirect_url"`
        Enabled      bool   `yaml:"enabled"`
    } `yaml:"google_oauth"`
    
    Metrics struct {
        Enabled          bool          `yaml:"enabled"`
        CollectionInterval time.Duration `yaml:"collection_interval"`
        RetentionDays    int           `yaml:"retention_days"`
    } `yaml:"metrics"`
}
```

**Load from environment:**
```go
func LoadConfig() (*Config, error) {
    cfg := &Config{}
    
    // Load from YAML file first
    // ... existing code ...
    
    // Override with environment variables
    if dsn := os.Getenv("AGENTBOX_DB_DSN"); dsn != "" {
        cfg.Database.DSN = dsn
    }
    if path := os.Getenv("AGENTBOX_DB_PATH"); path != "" {
        cfg.Database.Path = path
    }
    if username := os.Getenv("AGENTBOX_ADMIN_USERNAME"); username != "" {
        cfg.Auth.AdminUsername = username
    }
    // ... etc for all config values ...
    
    return cfg, nil
}
```

## 7. Implementation Order

1. **Database Package** (Week 1)
   - Database connection
   - Migration system
   - Schema validation

2. **User & Auth Package** (Week 1-2)
   - User model and CRUD
   - Password hashing
   - JWT generation/validation
   - API key generation/validation
   - Authentication middleware

3. **Metrics Package** (Week 2)
   - Metrics collection
   - Database storage
   - API endpoints

4. **API Endpoints** (Week 2-3)
   - Auth endpoints
   - User management endpoints
   - API key endpoints
   - Metrics endpoints

5. **Integration** (Week 3)
   - Update main service
   - Add middleware
   - Update existing handlers
   - Testing

## 8. Dependencies to Add

```go
// go.mod additions
require (
    github.com/golang-migrate/migrate/v4 v4.16.2
    github.com/lib/pq v1.10.9
    github.com/mattn/go-sqlite3 v1.14.17
    github.com/golang-jwt/jwt/v5 v5.2.0
    golang.org/x/crypto v0.17.0 // for bcrypt
    golang.org/x/oauth2 v0.15.0 // for Google OAuth
)
```
