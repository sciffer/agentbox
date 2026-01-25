package metrics

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/database"
	"github.com/sciffer/agentbox/pkg/orchestrator"
)

// Collector collects and stores metrics
type Collector struct {
	db           *database.DB
	orchestrator *orchestrator.Orchestrator
	interval     time.Duration
	enabled      bool
	stopChan     chan struct{}
	wg           sync.WaitGroup
	logger       *zap.Logger
}

// NewCollector creates a new metrics collector
func NewCollector(db *database.DB, orch *orchestrator.Orchestrator, logger *zap.Logger) *Collector {
	enabled := os.Getenv("AGENTBOX_METRICS_ENABLED") != "false"
	intervalStr := os.Getenv("AGENTBOX_METRICS_COLLECTION_INTERVAL")
	interval := 30 * time.Second
	if intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			interval = d
		}
	}

	return &Collector{
		db:           db,
		orchestrator: orch,
		interval:     interval,
		enabled:      enabled,
		stopChan:     make(chan struct{}),
		logger:       logger,
	}
}

// Start starts the metrics collection loop
func (c *Collector) Start(ctx context.Context) {
	if !c.enabled {
		c.logger.Info("metrics collection disabled")
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.collectLoop(ctx)
	}()
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	if !c.enabled {
		return
	}
	close(c.stopChan)
	c.wg.Wait()
}

// collectLoop runs the collection loop
func (c *Collector) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect immediately on start
	c.collectMetrics(ctx)

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

// collectMetrics collects all metrics
func (c *Collector) collectMetrics(ctx context.Context) {
	// Collect global metrics
	c.collectGlobalMetrics(ctx)

	// Collect per-environment metrics
	c.collectEnvironmentMetrics(ctx)
}

// collectGlobalMetrics collects system-wide metrics
func (c *Collector) collectGlobalMetrics(ctx context.Context) {
	// Get all environments
	envs, err := c.orchestrator.ListEnvironments(ctx, nil, "", 1000, 0)
	if err != nil {
		c.logger.Warn("failed to list environments for metrics", zap.Error(err))
		return
	}

	// Count running environments
	runningCount := 0
	var totalCPU, totalMemory float64
	var startTimes []time.Duration

	for i := range envs.Environments {
		env := &envs.Environments[i]
		if env.Status == "running" {
			runningCount++
		}

		// Calculate average start time (if started_at is available)
		if env.StartedAt != nil && !env.CreatedAt.IsZero() {
			startTime := env.StartedAt.Sub(env.CreatedAt)
			startTimes = append(startTimes, startTime)
		}
	}

	// Store running sandboxes metric
	if err := c.storeMetric(ctx, "", "running_sandboxes", float64(runningCount)); err != nil {
		c.logger.Warn("failed to store running_sandboxes metric", zap.Error(err))
	}

	// Calculate average start time
	if len(startTimes) > 0 {
		var total time.Duration
		for _, st := range startTimes {
			total += st
		}
		avgStartTime := total / time.Duration(len(startTimes))
		if err := c.storeMetric(ctx, "", "start_time", avgStartTime.Seconds()); err != nil {
			c.logger.Warn("failed to store start_time metric", zap.Error(err))
		}
	}

	// TODO: Collect actual CPU and memory usage from Kubernetes
	// For now, store placeholder values
	if runningCount > 0 {
		if err := c.storeMetric(ctx, "", "cpu_usage", totalCPU); err != nil {
			c.logger.Warn("failed to store cpu_usage metric", zap.Error(err))
		}
		if err := c.storeMetric(ctx, "", "memory_usage", totalMemory); err != nil {
			c.logger.Warn("failed to store memory_usage metric", zap.Error(err))
		}
	}
}

// collectEnvironmentMetrics collects metrics per environment
func (c *Collector) collectEnvironmentMetrics(ctx context.Context) {
	// Get all environments
	envs, err := c.orchestrator.ListEnvironments(ctx, nil, "", 1000, 0)
	if err != nil {
		c.logger.Warn("failed to list environments for metrics", zap.Error(err))
		return
	}

	for i := range envs.Environments {
		env := &envs.Environments[i]
		if env.Status == "running" {
			// Count running sandboxes for this environment
			if err := c.storeMetric(ctx, env.ID, "running_sandboxes", 1.0); err != nil {
				c.logger.Warn("failed to store env running_sandboxes metric", zap.Error(err))
			}

			// TODO: Get actual CPU/memory usage from Kubernetes
			// For now, store placeholder values
			if err := c.storeMetric(ctx, env.ID, "cpu_usage", 0.0); err != nil {
				c.logger.Warn("failed to store env cpu_usage metric", zap.Error(err))
			}
			if err := c.storeMetric(ctx, env.ID, "memory_usage", 0.0); err != nil {
				c.logger.Warn("failed to store env memory_usage metric", zap.Error(err))
			}

			// Calculate start time if available
			if env.StartedAt != nil && !env.CreatedAt.IsZero() {
				startTime := env.StartedAt.Sub(env.CreatedAt)
				if err := c.storeMetric(ctx, env.ID, "start_time", startTime.Seconds()); err != nil {
					c.logger.Warn("failed to store env start_time metric", zap.Error(err))
				}
			}
		}
	}
}

// StoreMetric stores a metric in the database (public for testing)
func (c *Collector) StoreMetric(ctx context.Context, envID, metricType string, value float64) error {
	return c.storeMetric(ctx, envID, metricType, value)
}

// storeMetric stores a metric in the database
func (c *Collector) storeMetric(ctx context.Context, envID, metricType string, value float64) error {
	id := uuid.New().String()

	var envIDNull interface{}
	if envID != "" {
		envIDNull = envID
	}

	_, err := c.db.ExecContext(ctx, `
		INSERT INTO metrics (id, environment_id, metric_type, value, timestamp)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	`, id, envIDNull, metricType, value)
	if err != nil {
		return fmt.Errorf("failed to store metric: %w", err)
	}

	return nil
}

// GetMetrics retrieves metrics from the database
func GetMetrics(ctx context.Context, db *database.DB, envID, metricType string,
	startTime, endTime time.Time) ([]Metric, error) {
	query := `
		SELECT id, environment_id, metric_type, value, timestamp
		FROM metrics
		WHERE metric_type = $1
		AND timestamp >= $2
		AND timestamp <= $3
	`
	args := []interface{}{metricType, startTime, endTime}

	if envID != "" {
		query += " AND environment_id = $4"
		args = append(args, envID)
	}

	query += " ORDER BY timestamp ASC"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		var envIDNull interface{}

		if err := rows.Scan(&m.ID, &envIDNull, &m.MetricType, &m.Value, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}

		if envIDNull != nil {
			if id, ok := envIDNull.(string); ok {
				m.EnvironmentID = &id
			}
		}

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// Metric represents a metric data point
type Metric struct {
	ID            string    `json:"id"`
	EnvironmentID *string   `json:"environment_id,omitempty"`
	MetricType    string    `json:"metric_type"`
	Value         float64   `json:"value"`
	Timestamp     time.Time `json:"timestamp"`
}
