package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sciffer/agentbox/pkg/models"
)

// SaveEnvironmentEvent persists a reconciliation or lifecycle event for an environment (shown in logs tab)
func (db *DB) SaveEnvironmentEvent(ctx context.Context, envID, eventType, message, details string) (*models.EnvironmentEvent, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO environment_events (id, environment_id, event_type, message, details, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := db.ExecContext(ctx, query, id, envID, eventType, message, nullIfEmpty(details), now)
	if err != nil {
		return nil, fmt.Errorf("failed to save environment event: %w", err)
	}

	return &models.EnvironmentEvent{
		ID:            id,
		EnvironmentID: envID,
		EventType:     eventType,
		Message:       message,
		Details:       details,
		CreatedAt:     now,
	}, nil
}

// ListEnvironmentEvents returns events for an environment, newest first (for merging with pod logs)
func (db *DB) ListEnvironmentEvents(ctx context.Context, environmentID string, limit int) ([]*models.EnvironmentEvent, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	query := `
		SELECT id, environment_id, event_type, message, COALESCE(details, ''), created_at
		FROM environment_events
		WHERE environment_id = $1
		ORDER BY created_at ASC
		LIMIT $2
	`
	rows, err := db.QueryContext(ctx, query, environmentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list environment events: %w", err)
	}
	defer rows.Close()

	var events []*models.EnvironmentEvent
	for rows.Next() {
		var e models.EnvironmentEvent
		if err := rows.Scan(&e.ID, &e.EnvironmentID, &e.EventType, &e.Message, &e.Details, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan environment event: %w", err)
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}
