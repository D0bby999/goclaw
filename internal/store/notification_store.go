package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Notification represents a persisted user notification.
type Notification struct {
	ID        uuid.UUID       `json:"id"`
	UserID    string          `json:"userId"`
	AgentID   *uuid.UUID      `json:"agentId,omitempty"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Message   string          `json:"message"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	Read      bool            `json:"read"`
	CreatedAt time.Time       `json:"createdAt"`
}

// NotificationStore manages persisted notifications.
type NotificationStore interface {
	Create(ctx context.Context, n *Notification) error
	List(ctx context.Context, userID string, limit, offset int) ([]Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID) error
	MarkAllRead(ctx context.Context, userID string) error
	CountUnread(ctx context.Context, userID string) (int, error)
	DeleteOld(ctx context.Context, olderThan time.Time) (int64, error)
}
