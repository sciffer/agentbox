package unit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
)

func TestDatabaseConnection(t *testing.T) {
	// Use temporary SQLite database for testing
	tmpFile, err := os.CreateTemp("", "test-*.db")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	defer os.Unsetenv("AGENTBOX_DB_PATH")

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	defer db.Close()

	// Test that schema was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "users table should exist")
}

func TestDatabaseMigrations(t *testing.T) {
	// Use temporary SQLite database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	defer os.Unsetenv("AGENTBOX_DB_PATH")

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	defer db.Close()

	// Verify all tables exist
	tables := []string{"users", "api_keys", "environment_permissions", "metrics", "schema_version"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, 1, count, "table %s should exist", table)
	}

	// Verify schema version was recorded
	var version int
	err = db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&version)
	require.NoError(t, err)
	assert.Greater(t, version, 0, "schema version should be recorded")
}

func TestDatabaseReconnect(t *testing.T) {
	// Test that reconnecting to the same database doesn't fail
	tmpFile, err := os.CreateTemp("", "test-*.db")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	defer os.Unsetenv("AGENTBOX_DB_PATH")

	logger := zap.NewNop()

	// First connection
	db1, err := database.NewDB(logger)
	require.NoError(t, err)
	db1.Close()

	// Second connection (should work with existing schema)
	db2, err := database.NewDB(logger)
	require.NoError(t, err)
	defer db2.Close()

	// Should still have tables
	var count int
	err = db2.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
