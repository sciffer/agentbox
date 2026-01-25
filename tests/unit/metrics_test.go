package unit

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/metrics"
)

func setupMetricsTest(t *testing.T) (*database.DB, *metrics.Collector) {
	tmpFile, err := os.CreateTemp("", "test-metrics-*.db")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close()

	os.Setenv("AGENTBOX_DB_PATH", tmpFile.Name())
	os.Setenv("AGENTBOX_METRICS_ENABLED", "true")
	t.Cleanup(func() {
		os.Unsetenv("AGENTBOX_DB_PATH")
		os.Unsetenv("AGENTBOX_METRICS_ENABLED")
	})

	logger := zap.NewNop()
	db, err := database.NewDB(logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.Close()
	})

	// Create a minimal collector for testing (nil orchestrator and k8sClient for basic tests)
	collector := metrics.NewCollector(db, nil, nil, logger)

	return db, collector
}

func TestStoreMetric(t *testing.T) {
	db, collector := setupMetricsTest(t)
	ctx := context.Background()

	// Store a metric
	err := collector.StoreMetric(ctx, "", "running_sandboxes", 5.0)
	require.NoError(t, err)

	// Verify it was stored
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metrics WHERE metric_type = 'running_sandboxes'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetMetrics(t *testing.T) {
	db, _ := setupMetricsTest(t)
	ctx := context.Background()

	// Store some metrics
	now := time.Now()
	for i := 0; i < 5; i++ {
		_, err := db.Exec(`
			INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
			VALUES ($1, $2, $3, $4, $5)
		`, fmt.Sprintf("id-%d", i), nil, "running_sandboxes", float64(i), now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	// Get metrics
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	metricList, err := metrics.GetMetrics(ctx, db, "", "running_sandboxes", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 5, len(metricList))
}

func TestGetMetricsWithEnvironmentID(t *testing.T) {
	db, _ := setupMetricsTest(t)
	ctx := context.Background()

	envID := "env-123"
	now := time.Now()

	// Store metrics for specific environment
	_, err := db.Exec(`
		INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, "id-1", envID, "cpu_usage", 50.0, now)
	require.NoError(t, err)

	// Get metrics for that environment
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	metricList, err := metrics.GetMetrics(ctx, db, envID, "cpu_usage", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(metricList))
	assert.NotNil(t, metricList[0].EnvironmentID)
	assert.Equal(t, envID, *metricList[0].EnvironmentID)
}
