package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/sciffer/agentbox/pkg/models"
)

// SaveEnvironment saves an environment to the database
func (db *DB) SaveEnvironment(ctx context.Context, env *models.Environment) error {
	// Serialize optional fields to JSON
	envVarsJSON, err := json.Marshal(env.Env)
	if err != nil {
		envVarsJSON = []byte("{}")
	}
	commandJSON, err := json.Marshal(env.Command)
	if err != nil {
		commandJSON = []byte("[]")
	}
	labelsJSON, err := json.Marshal(env.Labels)
	if err != nil {
		labelsJSON = []byte("{}")
	}
	nodeSelectorJSON, err := json.Marshal(env.NodeSelector)
	if err != nil {
		nodeSelectorJSON = []byte("{}")
	}
	tolerationsJSON, err := json.Marshal(env.Tolerations)
	if err != nil {
		tolerationsJSON = []byte("[]")
	}
	isolationJSON, err := json.Marshal(env.Isolation)
	if err != nil {
		isolationJSON = []byte("null")
	}
	poolJSON, err := json.Marshal(env.Pool)
	if err != nil {
		poolJSON = []byte("null")
	}

	query := `
		INSERT INTO environments (
			id, name, status, image, created_at, started_at, user_id, namespace, endpoint,
			timeout, resources_cpu, resources_memory, resources_storage,
			env_vars, command, labels, node_selector, tolerations, isolation_config, pool_config
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			started_at = EXCLUDED.started_at,
			endpoint = EXCLUDED.endpoint
	`

	_, err = db.ExecContext(ctx, query,
		env.ID, env.Name, string(env.Status), env.Image, env.CreatedAt, env.StartedAt, env.UserID,
		env.Namespace, env.Endpoint, env.Timeout,
		env.Resources.CPU, env.Resources.Memory, env.Resources.Storage,
		string(envVarsJSON), string(commandJSON), string(labelsJSON),
		string(nodeSelectorJSON), string(tolerationsJSON), string(isolationJSON), string(poolJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to save environment: %w", err)
	}

	return nil
}

// GetEnvironment retrieves an environment from the database
func (db *DB) GetEnvironment(ctx context.Context, id string) (*models.Environment, error) {
	var env models.Environment
	var statusStr string
	var envVarsJSON, commandJSON, labelsJSON, nodeSelectorJSON, tolerationsJSON, isolationJSON, poolJSON sql.NullString

	query := `
		SELECT id, name, status, image, created_at, started_at, user_id, namespace, endpoint,
			timeout, resources_cpu, resources_memory, resources_storage,
			env_vars, command, labels, node_selector, tolerations, isolation_config, pool_config
		FROM environments
		WHERE id = $1
	`

	err := db.QueryRowContext(ctx, query, id).Scan(
		&env.ID, &env.Name, &statusStr, &env.Image, &env.CreatedAt, &env.StartedAt, &env.UserID,
		&env.Namespace, &env.Endpoint, &env.Timeout,
		&env.Resources.CPU, &env.Resources.Memory, &env.Resources.Storage,
		&envVarsJSON, &commandJSON, &labelsJSON, &nodeSelectorJSON, &tolerationsJSON, &isolationJSON, &poolJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("environment not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	env.Status = models.EnvironmentStatus(statusStr)

	// Deserialize JSON fields
	if envVarsJSON.Valid {
		if err := json.Unmarshal([]byte(envVarsJSON.String), &env.Env); err != nil {
			db.logger.Warn("failed to unmarshal env_vars", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if commandJSON.Valid {
		if err := json.Unmarshal([]byte(commandJSON.String), &env.Command); err != nil {
			db.logger.Warn("failed to unmarshal command", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if labelsJSON.Valid {
		if err := json.Unmarshal([]byte(labelsJSON.String), &env.Labels); err != nil {
			db.logger.Warn("failed to unmarshal labels", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if nodeSelectorJSON.Valid {
		if err := json.Unmarshal([]byte(nodeSelectorJSON.String), &env.NodeSelector); err != nil {
			db.logger.Warn("failed to unmarshal node_selector", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if tolerationsJSON.Valid {
		if err := json.Unmarshal([]byte(tolerationsJSON.String), &env.Tolerations); err != nil {
			db.logger.Warn("failed to unmarshal tolerations", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if isolationJSON.Valid {
		if err := json.Unmarshal([]byte(isolationJSON.String), &env.Isolation); err != nil {
			db.logger.Warn("failed to unmarshal isolation_config", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}
	if poolJSON.Valid {
		if err := json.Unmarshal([]byte(poolJSON.String), &env.Pool); err != nil {
			db.logger.Warn("failed to unmarshal pool_config", zap.Error(err), zap.String("environment_id", env.ID))
		}
	}

	return &env, nil
}

// ListEnvironments retrieves all environments from the database
func (db *DB) ListEnvironments(ctx context.Context, limit, offset int) ([]*models.Environment, error) {
	query := `
		SELECT id, name, status, image, created_at, started_at, user_id, namespace, endpoint,
			timeout, resources_cpu, resources_memory, resources_storage,
			env_vars, command, labels, node_selector, tolerations, isolation_config, pool_config
		FROM environments
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	defer rows.Close()

	var environments []*models.Environment
	for rows.Next() {
		var env models.Environment
		var statusStr string
		var envVarsJSON, commandJSON, labelsJSON, nodeSelectorJSON, tolerationsJSON, isolationJSON, poolJSON sql.NullString

		err := rows.Scan(
			&env.ID, &env.Name, &statusStr, &env.Image, &env.CreatedAt, &env.StartedAt, &env.UserID,
			&env.Namespace, &env.Endpoint, &env.Timeout,
			&env.Resources.CPU, &env.Resources.Memory, &env.Resources.Storage,
			&envVarsJSON, &commandJSON, &labelsJSON, &nodeSelectorJSON, &tolerationsJSON, &isolationJSON, &poolJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}

		env.Status = models.EnvironmentStatus(statusStr)

		// Deserialize JSON fields
		if envVarsJSON.Valid {
			if err := json.Unmarshal([]byte(envVarsJSON.String), &env.Env); err != nil {
				db.logger.Warn("failed to unmarshal env_vars", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if commandJSON.Valid {
			if err := json.Unmarshal([]byte(commandJSON.String), &env.Command); err != nil {
				db.logger.Warn("failed to unmarshal command", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if labelsJSON.Valid {
			if err := json.Unmarshal([]byte(labelsJSON.String), &env.Labels); err != nil {
				db.logger.Warn("failed to unmarshal labels", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if nodeSelectorJSON.Valid {
			if err := json.Unmarshal([]byte(nodeSelectorJSON.String), &env.NodeSelector); err != nil {
				db.logger.Warn("failed to unmarshal node_selector", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if tolerationsJSON.Valid {
			if err := json.Unmarshal([]byte(tolerationsJSON.String), &env.Tolerations); err != nil {
				db.logger.Warn("failed to unmarshal tolerations", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if isolationJSON.Valid {
			if err := json.Unmarshal([]byte(isolationJSON.String), &env.Isolation); err != nil {
				db.logger.Warn("failed to unmarshal isolation_config", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}
		if poolJSON.Valid {
			if err := json.Unmarshal([]byte(poolJSON.String), &env.Pool); err != nil {
				db.logger.Warn("failed to unmarshal pool_config", zap.Error(err), zap.String("environment_id", env.ID))
			}
		}

		environments = append(environments, &env)
	}

	return environments, rows.Err()
}

// DeleteEnvironment deletes an environment from the database
func (db *DB) DeleteEnvironment(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM environments WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}
	return nil
}

// UpdateEnvironmentStatus updates an environment's status and optionally started_at
func (db *DB) UpdateEnvironmentStatus(ctx context.Context, id string, status models.EnvironmentStatus, startedAt *time.Time) error {
	query := "UPDATE environments SET status = $1, started_at = $2 WHERE id = $3"
	_, err := db.ExecContext(ctx, query, string(status), startedAt, id)
	if err != nil {
		return fmt.Errorf("failed to update environment status: %w", err)
	}
	return nil
}

// LoadAllEnvironments loads all environments from the database (for startup recovery)
func (db *DB) LoadAllEnvironments(ctx context.Context) ([]*models.Environment, error) {
	// Use a large limit to get all environments
	envs, err := db.ListEnvironments(ctx, 10000, 0)
	if err != nil {
		return nil, err
	}
	db.logger.Info("loaded environments from database", zap.Int("count", len(envs)))
	return envs, nil
}
