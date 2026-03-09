package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGContentScheduleStore implements store.ContentScheduleStore backed by Postgres.
type PGContentScheduleStore struct {
	db *sql.DB
}

func NewPGContentScheduleStore(db *sql.DB) *PGContentScheduleStore {
	return &PGContentScheduleStore{db: db}
}

// ── CRUD ──────────────────────────────────────────────────────────────

func (s *PGContentScheduleStore) Create(ctx context.Context, cs *store.ContentScheduleData) error {
	if cs.ID == uuid.Nil {
		cs.ID = store.GenNewID()
	}
	now := nowUTC()
	cs.CreatedAt = now
	cs.UpdatedAt = now
	if cs.ContentSource == "" {
		cs.ContentSource = store.ContentSourceAgent
	}
	if cs.Timezone == "" {
		cs.Timezone = "UTC"
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO content_schedules
		 (id, owner_id, name, enabled, content_source, agent_id, prompt,
		  cron_expression, timezone, cron_job_id, posts_count, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		cs.ID, cs.OwnerID, cs.Name, cs.Enabled, cs.ContentSource,
		nilUUID(cs.AgentID), cs.Prompt, cs.CronExpression, cs.Timezone,
		cs.CronJobID, cs.PostsCount, now, now,
	)
	return err
}

func (s *PGContentScheduleStore) Get(ctx context.Context, id uuid.UUID) (*store.ContentScheduleData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, name, enabled, content_source, agent_id, prompt,
		 cron_expression, timezone, cron_job_id, last_run_at, last_status, last_error,
		 posts_count, created_at, updated_at, deleted_at
		 FROM content_schedules WHERE id = $1 AND deleted_at IS NULL`, id)
	cs, err := scanSchedule(row)
	if err != nil {
		return nil, err
	}
	cs.Pages, _ = s.ListPages(ctx, cs.ID)
	return cs, nil
}

var allowedScheduleCols = map[string]bool{
	"name": true, "enabled": true, "content_source": true, "agent_id": true,
	"prompt": true, "cron_expression": true, "timezone": true, "cron_job_id": true,
	"last_run_at": true, "last_status": true, "last_error": true,
	"posts_count": true, "updated_at": true,
}

func (s *PGContentScheduleStore) Update(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedScheduleCols)
	if len(filtered) == 0 {
		return nil
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "content_schedules", id, filtered)
}

func (s *PGContentScheduleStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE content_schedules SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *PGContentScheduleStore) List(ctx context.Context, ownerID string, enabled *bool) ([]store.ContentScheduleData, error) {
	where := []string{"owner_id = $1", "deleted_at IS NULL"}
	args := []any{ownerID}
	idx := 2

	if enabled != nil {
		where = append(where, fmt.Sprintf("enabled = $%d", idx))
		args = append(args, *enabled)
		idx++
	}

	q := fmt.Sprintf(
		`SELECT id, owner_id, name, enabled, content_source, agent_id, prompt,
		 cron_expression, timezone, cron_job_id, last_run_at, last_status, last_error,
		 posts_count, created_at, updated_at, deleted_at
		 FROM content_schedules WHERE %s ORDER BY created_at DESC`,
		strings.Join(where, " AND "))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules, err := scanSchedules(rows)
	if err != nil {
		return nil, err
	}

	// Attach pages for each schedule.
	for i := range schedules {
		schedules[i].Pages, _ = s.ListPages(ctx, schedules[i].ID)
	}
	return schedules, nil
}

func (s *PGContentScheduleStore) GetByJobID(ctx context.Context, cronJobID string) (*store.ContentScheduleData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, name, enabled, content_source, agent_id, prompt,
		 cron_expression, timezone, cron_job_id, last_run_at, last_status, last_error,
		 posts_count, created_at, updated_at, deleted_at
		 FROM content_schedules WHERE cron_job_id = $1 AND deleted_at IS NULL`, cronJobID)
	cs, err := scanSchedule(row)
	if err != nil {
		return nil, err
	}
	cs.Pages, _ = s.ListPages(ctx, cs.ID)
	return cs, nil
}

// ── Scan helpers ──────────────────────────────────────────────────────

func scanSchedule(row *sql.Row) (*store.ContentScheduleData, error) {
	var cs store.ContentScheduleData
	var agentID uuid.UUID
	var lastRunAt sql.NullTime
	var lastStatus, lastError, cronJobID, prompt sql.NullString
	var deletedAt sql.NullTime

	err := row.Scan(
		&cs.ID, &cs.OwnerID, &cs.Name, &cs.Enabled, &cs.ContentSource,
		&agentID, &prompt, &cs.CronExpression, &cs.Timezone, &cronJobID,
		&lastRunAt, &lastStatus, &lastError,
		&cs.PostsCount, &cs.CreatedAt, &cs.UpdatedAt, &deletedAt,
	)
	if err != nil {
		return nil, err
	}
	if agentID != uuid.Nil {
		cs.AgentID = &agentID
	}
	if prompt.Valid {
		cs.Prompt = &prompt.String
	}
	if cronJobID.Valid {
		cs.CronJobID = &cronJobID.String
	}
	if lastRunAt.Valid {
		cs.LastRunAt = &lastRunAt.Time
	}
	if lastStatus.Valid {
		cs.LastStatus = &lastStatus.String
	}
	if lastError.Valid {
		cs.LastError = &lastError.String
	}
	if deletedAt.Valid {
		cs.DeletedAt = &deletedAt.Time
	}
	return &cs, nil
}

func scanSchedules(rows *sql.Rows) ([]store.ContentScheduleData, error) {
	var out []store.ContentScheduleData
	for rows.Next() {
		var cs store.ContentScheduleData
		var agentID uuid.UUID
		var lastRunAt sql.NullTime
		var lastStatus, lastError, cronJobID, prompt sql.NullString
		var deletedAt sql.NullTime

		if err := rows.Scan(
			&cs.ID, &cs.OwnerID, &cs.Name, &cs.Enabled, &cs.ContentSource,
			&agentID, &prompt, &cs.CronExpression, &cs.Timezone, &cronJobID,
			&lastRunAt, &lastStatus, &lastError,
			&cs.PostsCount, &cs.CreatedAt, &cs.UpdatedAt, &deletedAt,
		); err != nil {
			return nil, err
		}
		if agentID != uuid.Nil {
			cs.AgentID = &agentID
		}
		if prompt.Valid {
			cs.Prompt = &prompt.String
		}
		if cronJobID.Valid {
			cs.CronJobID = &cronJobID.String
		}
		if lastRunAt.Valid {
			cs.LastRunAt = &lastRunAt.Time
		}
		if lastStatus.Valid {
			cs.LastStatus = &lastStatus.String
		}
		if lastError.Valid {
			cs.LastError = &lastError.String
		}
		if deletedAt.Valid {
			cs.DeletedAt = &deletedAt.Time
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}
