package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ── Pages ─────────────────────────────────────────────────────────────

func (s *PGContentScheduleStore) SetPages(ctx context.Context, scheduleID uuid.UUID, pageIDs []uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM content_schedule_pages WHERE schedule_id = $1`, scheduleID); err != nil {
		return err
	}

	for _, pid := range pageIDs {
		id := store.GenNewID()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO content_schedule_pages (id, schedule_id, page_id, created_at)
			 VALUES ($1, $2, $3, NOW())`,
			id, scheduleID, pid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PGContentScheduleStore) ListPages(ctx context.Context, scheduleID uuid.UUID) ([]store.ContentSchedulePageData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT csp.id, csp.schedule_id, csp.page_id, csp.created_at,
		        sp.page_name, sp.page_type, sa.platform, sa.id AS account_id
		 FROM content_schedule_pages csp
		 JOIN social_pages sp ON sp.id = csp.page_id
		 JOIN social_accounts sa ON sa.id = sp.account_id
		 WHERE csp.schedule_id = $1
		 ORDER BY csp.created_at`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.ContentSchedulePageData
	for rows.Next() {
		var p store.ContentSchedulePageData
		var pageName sql.NullString
		if err := rows.Scan(
			&p.ID, &p.ScheduleID, &p.PageID, &p.CreatedAt,
			&pageName, &p.PageType, &p.Platform, &p.AccountID,
		); err != nil {
			return nil, err
		}
		if pageName.Valid {
			p.PageName = &pageName.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ── Logs ──────────────────────────────────────────────────────────────

func (s *PGContentScheduleStore) AddLog(ctx context.Context, l *store.ContentScheduleLogData) error {
	if l.ID == uuid.Nil {
		l.ID = store.GenNewID()
	}
	if l.RanAt.IsZero() {
		l.RanAt = nowUTC()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO content_schedule_logs
		 (id, schedule_id, post_id, status, error, content_preview,
		  pages_targeted, pages_published, duration_ms, ran_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		l.ID, l.ScheduleID, nilUUID(l.PostID), l.Status, l.Error,
		l.ContentPreview, l.PagesTargeted, l.PagesPublished, l.DurationMS, l.RanAt,
	)
	return err
}

func (s *PGContentScheduleStore) ListLogs(ctx context.Context, scheduleID uuid.UUID, limit, offset int) ([]store.ContentScheduleLogData, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	_ = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM content_schedule_logs WHERE schedule_id = $1`,
		scheduleID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, schedule_id, post_id, status, error, content_preview,
		 pages_targeted, pages_published, duration_ms, ran_at
		 FROM content_schedule_logs
		 WHERE schedule_id = $1
		 ORDER BY ran_at DESC
		 LIMIT $2 OFFSET $3`,
		scheduleID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []store.ContentScheduleLogData
	for rows.Next() {
		var l store.ContentScheduleLogData
		var postID uuid.UUID
		var logErr, preview sql.NullString
		var durationMS sql.NullInt64

		if err := rows.Scan(
			&l.ID, &l.ScheduleID, &postID, &l.Status,
			&logErr, &preview, &l.PagesTargeted, &l.PagesPublished,
			&durationMS, &l.RanAt,
		); err != nil {
			return nil, 0, err
		}
		if postID != uuid.Nil {
			l.PostID = &postID
		}
		if logErr.Valid {
			l.Error = &logErr.String
		}
		if preview.Valid {
			l.ContentPreview = &preview.String
		}
		if durationMS.Valid {
			l.DurationMS = &durationMS.Int64
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}
