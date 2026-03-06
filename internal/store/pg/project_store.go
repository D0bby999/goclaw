package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGProjectStore implements store.ProjectStore backed by Postgres.
type PGProjectStore struct {
	db     *sql.DB
	encKey string
}

func NewPGProjectStore(db *sql.DB, encryptionKey string) *PGProjectStore {
	return &PGProjectStore{db: db, encKey: encryptionKey}
}

// ============================================================
// Projects
// ============================================================

const projectSelectCols = `id, name, slug, work_dir, description, allowed_tools, claude_config, max_sessions, owner_id, team_id, status, created_at, updated_at`

func (s *PGProjectStore) CreateProject(ctx context.Context, p *store.ProjectData) error {
	if p.ID == uuid.Nil {
		p.ID = store.GenNewID()
	}
	now := nowUTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = store.ProjectStatusActive
	}
	if p.MaxSessions == 0 {
		p.MaxSessions = 3
	}

	allowedTools := jsonOrNull(p.AllowedTools)
	claudeConfig := jsonOrNull(p.ClaudeConfig)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, slug, work_dir, description, allowed_tools, claude_config, max_sessions, owner_id, team_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		p.ID, p.Name, p.Slug, p.WorkDir, p.Description,
		allowedTools, claudeConfig, p.MaxSessions,
		p.OwnerID, nilUUID(p.TeamID), p.Status, now, now,
	)
	return err
}

func (s *PGProjectStore) GetProject(ctx context.Context, id uuid.UUID) (*store.ProjectData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+projectSelectCols+` FROM projects WHERE id = $1`, id)
	return s.scanProjectRow(row)
}

func (s *PGProjectStore) GetProjectBySlug(ctx context.Context, slug string) (*store.ProjectData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+projectSelectCols+` FROM projects WHERE slug = $1`, slug)
	return s.scanProjectRow(row)
}

// allowedProjectCols defines columns that can be updated via dynamic map updates.
var allowedProjectCols = map[string]bool{
	"name": true, "slug": true, "work_dir": true, "description": true,
	"allowed_tools": true, "claude_config": true, "max_sessions": true,
	"status": true, "team_id": true, "updated_at": true,
}

// allowedSessionCols defines columns that can be updated via dynamic map updates.
var allowedSessionCols = map[string]bool{
	"claude_session_id": true, "label": true, "status": true, "pid": true,
	"input_tokens": true, "output_tokens": true, "cost_usd": true,
	"error": true, "stopped_at": true, "updated_at": true,
}

func (s *PGProjectStore) UpdateProject(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedProjectCols)
	if len(filtered) == 0 {
		return nil
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "projects", id, filtered)
}

func (s *PGProjectStore) DeleteProject(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

func (s *PGProjectStore) ListProjects(ctx context.Context, ownerID string) ([]store.ProjectData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectSelectCols+` FROM projects WHERE owner_id = $1 AND status = 'active' ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanProjectRows(rows)
}

func (s *PGProjectStore) ListProjectsByTeam(ctx context.Context, teamID uuid.UUID) ([]store.ProjectData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+projectSelectCols+` FROM projects WHERE team_id = $1 AND status = 'active' ORDER BY created_at DESC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanProjectRows(rows)
}

func (s *PGProjectStore) scanProjectRow(row *sql.Row) (*store.ProjectData, error) {
	var p store.ProjectData
	var desc, status sql.NullString
	var teamID *uuid.UUID
	var allowedTools, claudeConfig []byte
	if err := row.Scan(
		&p.ID, &p.Name, &p.Slug, &p.WorkDir, &desc,
		&allowedTools, &claudeConfig, &p.MaxSessions,
		&p.OwnerID, &teamID, &status, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.AllowedTools = allowedTools
	p.ClaudeConfig = claudeConfig
	if desc.Valid {
		p.Description = desc.String
	}
	if status.Valid {
		p.Status = status.String
	}
	p.TeamID = teamID
	return &p, nil
}

func (s *PGProjectStore) scanProjectRows(rows *sql.Rows) ([]store.ProjectData, error) {
	var projects []store.ProjectData
	for rows.Next() {
		var p store.ProjectData
		var desc, status sql.NullString
		var teamID *uuid.UUID
		var allowedTools, claudeConfig []byte
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.WorkDir, &desc,
			&allowedTools, &claudeConfig, &p.MaxSessions,
			&p.OwnerID, &teamID, &status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.AllowedTools = allowedTools
		p.ClaudeConfig = claudeConfig
		if desc.Valid {
			p.Description = desc.String
		}
		if status.Valid {
			p.Status = status.String
		}
		p.TeamID = teamID
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ============================================================
// Sessions
// ============================================================

const sessionSelectCols = `s.id, s.project_id, s.claude_session_id, s.label, s.status, s.pid, s.started_by, s.input_tokens, s.output_tokens, s.cost_usd, s.error, s.started_at, s.stopped_at, s.created_at, s.updated_at`

func (s *PGProjectStore) CreateSession(ctx context.Context, sess *store.ProjectSessionData) error {
	if sess.ID == uuid.Nil {
		sess.ID = store.GenNewID()
	}
	now := nowUTC()
	sess.CreatedAt = now
	sess.UpdatedAt = now
	if sess.Status == "" {
		sess.Status = store.ProjectSessionStatusStarting
	}
	sess.StartedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO project_sessions (id, project_id, claude_session_id, label, status, pid, started_by, input_tokens, output_tokens, cost_usd, error, started_at, stopped_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		sess.ID, sess.ProjectID, nilStr(derefStr(sess.ClaudeSessionID)), sess.Label,
		sess.Status, nilInt(derefInt(sess.PID)), sess.StartedBy,
		sess.InputTokens, sess.OutputTokens, sess.CostUSD,
		nilStr(derefStr(sess.Error)), sess.StartedAt, nilTime(sess.StoppedAt),
		now, now,
	)
	return err
}

func (s *PGProjectStore) GetSession(ctx context.Context, id uuid.UUID) (*store.ProjectSessionData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+sessionSelectCols+`, COALESCE(p.name, '') AS project_name, COALESCE(p.slug, '') AS project_slug
		 FROM project_sessions s
		 LEFT JOIN projects p ON p.id = s.project_id
		 WHERE s.id = $1`, id)
	return scanSessionRow(row)
}

func (s *PGProjectStore) UpdateSession(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedSessionCols)
	if len(filtered) == 0 {
		return nil
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "project_sessions", id, filtered)
}

func (s *PGProjectStore) DeleteSession(ctx context.Context, id uuid.UUID) error {
	// Delete logs first (FK dependency), then the session.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_session_logs WHERE session_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_sessions WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PGProjectStore) ListSessions(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]store.ProjectSessionData, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_sessions WHERE project_id = $1`, projectID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+sessionSelectCols+`, COALESCE(p.name, '') AS project_name, COALESCE(p.slug, '') AS project_slug
		 FROM project_sessions s
		 LEFT JOIN projects p ON p.id = s.project_id
		 WHERE s.project_id = $1
		 ORDER BY s.created_at DESC
		 LIMIT $2 OFFSET $3`, projectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sessions []store.ProjectSessionData
	for rows.Next() {
		sess, err := scanSessionFromRows(rows)
		if err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, total, rows.Err()
}

func (s *PGProjectStore) ActiveSessionCount(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM project_sessions WHERE project_id = $1 AND status IN ('starting', 'running')`, projectID).Scan(&count)
	return count, err
}

func scanSessionRow(row *sql.Row) (*store.ProjectSessionData, error) {
	var sess store.ProjectSessionData
	var claudeSessionID, label, status, errorStr sql.NullString
	var pid sql.NullInt32
	var stoppedAt sql.NullTime
	if err := row.Scan(
		&sess.ID, &sess.ProjectID, &claudeSessionID, &label, &status,
		&pid, &sess.StartedBy, &sess.InputTokens, &sess.OutputTokens, &sess.CostUSD,
		&errorStr, &sess.StartedAt, &stoppedAt, &sess.CreatedAt, &sess.UpdatedAt,
		&sess.ProjectName, &sess.ProjectSlug,
	); err != nil {
		return nil, err
	}
	if claudeSessionID.Valid {
		sess.ClaudeSessionID = &claudeSessionID.String
	}
	if label.Valid {
		sess.Label = label.String
	}
	if status.Valid {
		sess.Status = status.String
	}
	if pid.Valid {
		v := int(pid.Int32)
		sess.PID = &v
	}
	if errorStr.Valid {
		sess.Error = &errorStr.String
	}
	if stoppedAt.Valid {
		sess.StoppedAt = &stoppedAt.Time
	}
	return &sess, nil
}

func scanSessionFromRows(rows *sql.Rows) (*store.ProjectSessionData, error) {
	var sess store.ProjectSessionData
	var claudeSessionID, label, status, errorStr sql.NullString
	var pid sql.NullInt32
	var stoppedAt sql.NullTime
	if err := rows.Scan(
		&sess.ID, &sess.ProjectID, &claudeSessionID, &label, &status,
		&pid, &sess.StartedBy, &sess.InputTokens, &sess.OutputTokens, &sess.CostUSD,
		&errorStr, &sess.StartedAt, &stoppedAt, &sess.CreatedAt, &sess.UpdatedAt,
		&sess.ProjectName, &sess.ProjectSlug,
	); err != nil {
		return nil, err
	}
	if claudeSessionID.Valid {
		sess.ClaudeSessionID = &claudeSessionID.String
	}
	if label.Valid {
		sess.Label = label.String
	}
	if status.Valid {
		sess.Status = status.String
	}
	if pid.Valid {
		v := int(pid.Int32)
		sess.PID = &v
	}
	if errorStr.Valid {
		sess.Error = &errorStr.String
	}
	if stoppedAt.Valid {
		sess.StoppedAt = &stoppedAt.Time
	}
	return &sess, nil
}

// ============================================================
// Logs
// ============================================================

func (s *PGProjectStore) AppendLog(ctx context.Context, log *store.ProjectSessionLogData) error {
	if log.ID == uuid.Nil {
		log.ID = store.GenNewID()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = nowUTC()
	}

	// Atomic seq assignment via INSERT ... SELECT to avoid TOCTOU race
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO project_session_logs (id, session_id, event_type, content, seq, created_at)
		 VALUES ($1, $2, $3, $4, (SELECT COALESCE(MAX(seq), -1) + 1 FROM project_session_logs WHERE session_id = $2), $5)
		 RETURNING seq`,
		log.ID, log.SessionID, log.EventType, []byte(log.Content), log.CreatedAt,
	).Scan(&log.Seq)
	return err
}

func (s *PGProjectStore) GetLogs(ctx context.Context, sessionID uuid.UUID, afterSeq, limit int) ([]store.ProjectSessionLogData, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, event_type, content, seq, created_at
		 FROM project_session_logs
		 WHERE session_id = $1 AND seq > $2
		 ORDER BY seq ASC
		 LIMIT $3`, sessionID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []store.ProjectSessionLogData
	for rows.Next() {
		var l store.ProjectSessionLogData
		if err := rows.Scan(&l.ID, &l.SessionID, &l.EventType, &l.Content, &l.Seq, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- Helpers ---

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// filterCols returns only map entries whose keys are in the allowed set.
func filterCols(m map[string]any, allowed map[string]bool) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if allowed[k] {
			out[k] = v
		}
	}
	return out
}
