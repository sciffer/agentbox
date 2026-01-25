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

func TestStoreCPUMetric(t *testing.T) {
	db, collector := setupMetricsTest(t)
	ctx := context.Background()

	envID := "env-cpu-test"

	// Store CPU usage metric (in millicores)
	err := collector.StoreMetric(ctx, envID, "cpu_usage", 250.0) // 250 millicores
	require.NoError(t, err)

	// Verify it was stored by querying the database directly
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metrics WHERE metric_type = 'cpu_usage' AND environment_id = $1", envID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected 1 metric to be stored")

	// Verify value
	var value float64
	err = db.QueryRow("SELECT value FROM metrics WHERE metric_type = 'cpu_usage' AND environment_id = $1", envID).Scan(&value)
	require.NoError(t, err)
	assert.Equal(t, 250.0, value)
}

func TestStoreMemoryMetric(t *testing.T) {
	db, collector := setupMetricsTest(t)
	ctx := context.Background()

	envID := "env-memory-test"

	// Store memory usage metric (in MiB)
	err := collector.StoreMetric(ctx, envID, "memory_usage", 512.5) // 512.5 MiB
	require.NoError(t, err)

	// Verify it was stored by querying the database directly
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM metrics WHERE metric_type = 'memory_usage' AND environment_id = $1", envID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected 1 metric to be stored")

	// Verify value
	var value float64
	err = db.QueryRow("SELECT value FROM metrics WHERE metric_type = 'memory_usage' AND environment_id = $1", envID).Scan(&value)
	require.NoError(t, err)
	assert.Equal(t, 512.5, value)
}

func TestStoreMultipleMetricTypes(t *testing.T) {
	db, collector := setupMetricsTest(t)
	ctx := context.Background()

	envID := "env-multi-metrics"

	// Store multiple metric types
	err := collector.StoreMetric(ctx, envID, "cpu_usage", 100.0)
	require.NoError(t, err)

	err = collector.StoreMetric(ctx, envID, "memory_usage", 256.0)
	require.NoError(t, err)

	err = collector.StoreMetric(ctx, envID, "running_sandboxes", 1.0)
	require.NoError(t, err)

	// Verify each metric type by querying the database directly
	var cpuValue, memValue, sandboxValue float64

	err = db.QueryRow("SELECT value FROM metrics WHERE metric_type = 'cpu_usage' AND environment_id = $1", envID).Scan(&cpuValue)
	require.NoError(t, err)
	assert.Equal(t, 100.0, cpuValue)

	err = db.QueryRow("SELECT value FROM metrics WHERE metric_type = 'memory_usage' AND environment_id = $1", envID).Scan(&memValue)
	require.NoError(t, err)
	assert.Equal(t, 256.0, memValue)

	err = db.QueryRow("SELECT value FROM metrics WHERE metric_type = 'running_sandboxes' AND environment_id = $1", envID).Scan(&sandboxValue)
	require.NoError(t, err)
	assert.Equal(t, 1.0, sandboxValue)
}

func TestGetMetricsTimeRange(t *testing.T) {
	db, _ := setupMetricsTest(t)
	ctx := context.Background()

	envID := "env-time-range"
	baseTime := time.Now()

	// Store metrics at different times
	for i := 0; i < 10; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Hour)
		_, err := db.Exec(`
			INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
			VALUES ($1, $2, $3, $4, $5)
		`, fmt.Sprintf("id-time-%d", i), envID, "cpu_usage", float64(i*10), ts)
		require.NoError(t, err)
	}

	// Query for a specific time range (should get subset)
	startTime := baseTime.Add(2 * time.Hour)
	endTime := baseTime.Add(5 * time.Hour)
	metricList, err := metrics.GetMetrics(ctx, db, envID, "cpu_usage", startTime, endTime)
	require.NoError(t, err)

	// Should get metrics from hours 2, 3, 4, 5 = 4 metrics
	assert.Equal(t, 4, len(metricList))

	// Verify order (should be ascending by timestamp)
	for i := 0; i < len(metricList)-1; i++ {
		assert.True(t, metricList[i].Timestamp.Before(metricList[i+1].Timestamp) ||
			metricList[i].Timestamp.Equal(metricList[i+1].Timestamp))
	}
}

func TestGetGlobalMetrics(t *testing.T) {
	db, _ := setupMetricsTest(t)
	ctx := context.Background()

	now := time.Now()

	// Store global metrics (no environment ID)
	for i := 0; i < 3; i++ {
		_, err := db.Exec(`
			INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
			VALUES ($1, $2, $3, $4, $5)
		`, fmt.Sprintf("global-%d", i), nil, "running_sandboxes", float64(i+1), now.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	// Get global metrics (empty envID)
	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	metricList, err := metrics.GetMetrics(ctx, db, "", "running_sandboxes", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 3, len(metricList))

	// All should have nil EnvironmentID
	for _, m := range metricList {
		assert.Nil(t, m.EnvironmentID)
	}
}

func TestMetricsFilterByEnvironment(t *testing.T) {
	db, _ := setupMetricsTest(t)
	ctx := context.Background()

	now := time.Now()

	// Store metrics for multiple environments
	envIDs := []string{"env-1", "env-2", "env-3"}
	for i, envID := range envIDs {
		_, err := db.Exec(`
			INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
			VALUES ($1, $2, $3, $4, $5)
		`, fmt.Sprintf("metric-%d", i), envID, "cpu_usage", float64((i+1)*100), now)
		require.NoError(t, err)
	}

	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)

	// Query for specific environment
	metricList, err := metrics.GetMetrics(ctx, db, "env-2", "cpu_usage", startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(metricList))
	assert.Equal(t, 200.0, metricList[0].Value)
	assert.Equal(t, "env-2", *metricList[0].EnvironmentID)
}
