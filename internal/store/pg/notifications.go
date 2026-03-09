package pg

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type PGNotificationStore struct {
	db *sql.DB
}

func NewPGNotificationStore(db *sql.DB) *PGNotificationStore {
	return &PGNotificationStore{db: db}
}

func (s *PGNotificationStore) Create(ctx context.Context, n *store.Notification) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	metaBytes := n.Metadata
	if metaBytes == nil {
		metaBytes = []byte("{}")
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notifications (id, user_id, agent_id, type, title, message, metadata, read, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		n.ID, n.UserID, n.AgentID, n.Type, n.Title, n.Message, metaBytes, n.Read, n.CreatedAt,
	)
	return err
}

func (s *PGNotificationStore) List(ctx context.Context, userID string, limit, offset int) ([]store.Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, agent_id, type, title, message, metadata, read, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.Notification
	for rows.Next() {
		var n store.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.AgentID, &n.Type, &n.Title, &n.Message, &n.Metadata, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *PGNotificationStore) MarkRead(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET read = TRUE WHERE id = $1`, id,
	)
	return err
}

func (s *PGNotificationStore) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`, userID,
	)
	return err
}

func (s *PGNotificationStore) CountUnread(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE`, userID,
	).Scan(&count)
	return count, err
}

func (s *PGNotificationStore) DeleteOld(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notifications WHERE created_at < $1`, olderThan,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
