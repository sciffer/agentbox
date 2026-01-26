package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/models"
)

// SaveExecution saves an execution to the database
func (db *DB) SaveExecution(ctx context.Context, exec *models.Execution) error {
	// Serialize optional fields to JSON
	envVarsJSON, err := json.Marshal(exec.Env)
	if err != nil {
		envVarsJSON = []byte("{}")
	}
	commandJSON, err := json.Marshal(exec.Command)
	if err != nil {
		commandJSON = []byte("[]")
	}

	query := `
		INSERT INTO executions (
			id, environment_id, user_id, command, env_vars, status, pod_name, namespace,
			created_at, queued_at, started_at, completed_at,
			exit_code, stdout, stderr, error, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			queued_at = EXCLUDED.queued_at,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			exit_code = EXCLUDED.exit_code,
			stdout = EXCLUDED.stdout,
			stderr = EXCLUDED.stderr,
			error = EXCLUDED.error,
			duration_ms = EXCLUDED.duration_ms,
			pod_name = EXCLUDED.pod_name,
			namespace = EXCLUDED.namespace
	`

	_, err = db.ExecContext(ctx, query,
		exec.ID, exec.EnvironmentID, exec.UserID, string(commandJSON), string(envVarsJSON),
		string(exec.Status), exec.PodName, exec.Namespace,
		exec.CreatedAt, exec.QueuedAt, exec.StartedAt, exec.CompletedAt,
		exec.ExitCode, exec.Stdout, exec.Stderr, exec.Error, exec.DurationMs,
	)

	if err != nil {
		return fmt.Errorf("failed to save execution: %w", err)
	}

	return nil
}

// GetExecution retrieves an execution from the database
func (db *DB) GetExecution(ctx context.Context, id string) (*models.Execution, error) {
	var exec models.Execution
	var statusStr string
	var commandJSON, envVarsJSON sql.NullString

	query := `
		SELECT id, environment_id, user_id, command, env_vars, status, pod_name, namespace,
			created_at, queued_at, started_at, completed_at,
			exit_code, stdout, stderr, error, duration_ms
		FROM executions
		WHERE id = $1
	`

	err := db.QueryRowContext(ctx, query, id).Scan(
		&exec.ID, &exec.EnvironmentID, &exec.UserID, &commandJSON, &envVarsJSON,
		&statusStr, &exec.PodName, &exec.Namespace,
		&exec.CreatedAt, &exec.QueuedAt, &exec.StartedAt, &exec.CompletedAt,
		&exec.ExitCode, &exec.Stdout, &exec.Stderr, &exec.Error, &exec.DurationMs,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	exec.Status = models.ExecutionStatus(statusStr)

	// Deserialize JSON fields
	if commandJSON.Valid {
		if err := json.Unmarshal([]byte(commandJSON.String), &exec.Command); err != nil {
			db.logger.Warn("failed to unmarshal command", zap.Error(err), zap.String("execution_id", exec.ID))
		}
	}
	if envVarsJSON.Valid {
		if err := json.Unmarshal([]byte(envVarsJSON.String), &exec.Env); err != nil {
			db.logger.Warn("failed to unmarshal env_vars", zap.Error(err), zap.String("execution_id", exec.ID))
		}
	}

	return &exec, nil
}

// ListExecutions retrieves executions for an environment from the database
func (db *DB) ListExecutions(ctx context.Context, environmentID string, limit int) ([]*models.Execution, error) {
	query := `
		SELECT id, environment_id, user_id, command, env_vars, status, pod_name, namespace,
			created_at, queued_at, started_at, completed_at,
			exit_code, stdout, stderr, error, duration_ms
		FROM executions
		WHERE environment_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.QueryContext(ctx, query, environmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var executions []*models.Execution
	for rows.Next() {
		var exec models.Execution
		var statusStr string
		var commandJSON, envVarsJSON sql.NullString

		err := rows.Scan(
			&exec.ID, &exec.EnvironmentID, &exec.UserID, &commandJSON, &envVarsJSON,
			&statusStr, &exec.PodName, &exec.Namespace,
			&exec.CreatedAt, &exec.QueuedAt, &exec.StartedAt, &exec.CompletedAt,
			&exec.ExitCode, &exec.Stdout, &exec.Stderr, &exec.Error, &exec.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		exec.Status = models.ExecutionStatus(statusStr)

		// Deserialize JSON fields
		if commandJSON.Valid {
			if err := json.Unmarshal([]byte(commandJSON.String), &exec.Command); err != nil {
				db.logger.Warn("failed to unmarshal command", zap.Error(err), zap.String("execution_id", exec.ID))
			}
		}
		if envVarsJSON.Valid {
			if err := json.Unmarshal([]byte(envVarsJSON.String), &exec.Env); err != nil {
				db.logger.Warn("failed to unmarshal env_vars", zap.Error(err), zap.String("execution_id", exec.ID))
			}
		}

		executions = append(executions, &exec)
	}

	return executions, rows.Err()
}

// DeleteExecution deletes an execution from the database
func (db *DB) DeleteExecution(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM executions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete execution: %w", err)
	}
	return nil
}

// LoadAllExecutions loads all executions from the database (for startup recovery)
func (db *DB) LoadAllExecutions(ctx context.Context) ([]*models.Execution, error) {
	query := `
		SELECT id, environment_id, user_id, command, env_vars, status, pod_name, namespace,
			created_at, queued_at, started_at, completed_at,
			exit_code, stdout, stderr, error, duration_ms
		FROM executions
		ORDER BY created_at DESC
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load executions: %w", err)
	}
	defer rows.Close()

	var executions []*models.Execution
	for rows.Next() {
		var exec models.Execution
		var statusStr string
		var commandJSON, envVarsJSON sql.NullString

		err := rows.Scan(
			&exec.ID, &exec.EnvironmentID, &exec.UserID, &commandJSON, &envVarsJSON,
			&statusStr, &exec.PodName, &exec.Namespace,
			&exec.CreatedAt, &exec.QueuedAt, &exec.StartedAt, &exec.CompletedAt,
			&exec.ExitCode, &exec.Stdout, &exec.Stderr, &exec.Error, &exec.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}

		exec.Status = models.ExecutionStatus(statusStr)

		// Deserialize JSON fields
		if commandJSON.Valid {
			if err := json.Unmarshal([]byte(commandJSON.String), &exec.Command); err != nil {
				db.logger.Warn("failed to unmarshal command", zap.Error(err), zap.String("execution_id", exec.ID))
			}
		}
		if envVarsJSON.Valid {
			if err := json.Unmarshal([]byte(envVarsJSON.String), &exec.Env); err != nil {
				db.logger.Warn("failed to unmarshal env_vars", zap.Error(err), zap.String("execution_id", exec.ID))
			}
		}

		executions = append(executions, &exec)
	}

	db.logger.Info("loaded executions from database", zap.Int("count", len(executions)))
	return executions, rows.Err()
}
