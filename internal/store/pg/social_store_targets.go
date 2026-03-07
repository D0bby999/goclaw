package pg

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ── Targets ───────────────────────────────────────────────────────────

func (s *PGSocialStore) AddTarget(ctx context.Context, t *store.SocialPostTargetData) error {
	if t.ID == uuid.Nil {
		t.ID = store.GenNewID()
	}
	if t.Status == "" {
		t.Status = store.SocialTargetStatusPending
	}
	t.CreatedAt = nowUTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_post_targets (id, post_id, account_id, platform_post_id, platform_url,
		 adapted_content, status, error, published_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		t.ID, t.PostID, t.AccountID, t.PlatformPostID, t.PlatformURL,
		t.AdaptedContent, t.Status, t.Error, nilTime(t.PublishedAt), t.CreatedAt,
	)
	return err
}

// Allowed columns for social_post_targets updates.
var allowedTargetCols = map[string]bool{
	"platform_post_id": true, "platform_url": true, "adapted_content": true,
	"status": true, "error": true, "published_at": true,
}

func (s *PGSocialStore) UpdateTarget(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedTargetCols)
	if len(filtered) == 0 {
		return nil
	}
	return execMapUpdate(ctx, s.db, "social_post_targets", id, filtered)
}

func (s *PGSocialStore) ListTargets(ctx context.Context, postID uuid.UUID) ([]store.SocialPostTargetData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.post_id, t.account_id, t.platform_post_id, t.platform_url,
		 t.adapted_content, t.status, t.error, t.published_at, t.created_at,
		 a.platform, a.platform_username
		 FROM social_post_targets t
		 LEFT JOIN social_accounts a ON a.id = t.account_id
		 WHERE t.post_id = $1
		 ORDER BY t.created_at`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTargets(rows)
}

func (s *PGSocialStore) RemoveTarget(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM social_post_targets WHERE id = $1`, id)
	return err
}

// ── Media ─────────────────────────────────────────────────────────────

func (s *PGSocialStore) AddMedia(ctx context.Context, m *store.SocialPostMediaData) error {
	if m.ID == uuid.Nil {
		m.ID = store.GenNewID()
	}
	m.CreatedAt = nowUTC()
	if len(m.Metadata) == 0 {
		m.Metadata = []byte(`{}`)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_post_media (id, post_id, media_type, url, thumbnail_url, filename,
		 mime_type, file_size, width, height, duration_seconds, sort_order, metadata, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		m.ID, m.PostID, m.MediaType, m.URL, m.ThumbnailURL, m.Filename,
		m.MimeType, m.FileSize, m.Width, m.Height, m.DurationSeconds,
		m.SortOrder, m.Metadata, m.CreatedAt,
	)
	return err
}

func (s *PGSocialStore) ListMedia(ctx context.Context, postID uuid.UUID) ([]store.SocialPostMediaData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, post_id, media_type, url, thumbnail_url, filename, mime_type, file_size,
		 width, height, duration_seconds, sort_order, metadata, created_at
		 FROM social_post_media WHERE post_id = $1
		 ORDER BY sort_order, created_at`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMediaItems(rows)
}

func (s *PGSocialStore) RemoveMedia(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM social_post_media WHERE id = $1`, id)
	return err
}

func (s *PGSocialStore) ReorderMedia(ctx context.Context, postID uuid.UUID, mediaIDs []uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, mid := range mediaIDs {
		_, err := tx.ExecContext(ctx,
			`UPDATE social_post_media SET sort_order = $1 WHERE id = $2 AND post_id = $3`,
			i, mid, postID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── OAuth States ──────────────────────────────────────────────────────

func (s *PGSocialStore) CreateOAuthState(ctx context.Context, st *store.SocialOAuthStateData) error {
	if st.ID == uuid.Nil {
		st.ID = store.GenNewID()
	}
	st.CreatedAt = nowUTC()
	if len(st.Metadata) == 0 {
		st.Metadata = []byte(`{}`)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_oauth_states (id, platform, state, owner_id, redirect_url, metadata, expires_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		st.ID, st.Platform, st.State, st.OwnerID, st.RedirectURL,
		st.Metadata, st.ExpiresAt, st.CreatedAt,
	)
	return err
}

func (s *PGSocialStore) GetOAuthState(ctx context.Context, state string) (*store.SocialOAuthStateData, error) {
	var st store.SocialOAuthStateData
	err := s.db.QueryRowContext(ctx,
		`SELECT id, platform, state, owner_id, redirect_url, metadata, expires_at, created_at
		 FROM social_oauth_states WHERE state = $1`, state).Scan(
		&st.ID, &st.Platform, &st.State, &st.OwnerID, &st.RedirectURL,
		&st.Metadata, &st.ExpiresAt, &st.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *PGSocialStore) DeleteOAuthState(ctx context.Context, state string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM social_oauth_states WHERE state = $1`, state)
	return err
}

func (s *PGSocialStore) CleanExpiredOAuthStates(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM social_oauth_states WHERE expires_at < NOW()`)
	return err
}

// ── Scan helpers (posts, targets, media) ──────────────────────────────

// SocialPostData is a type alias to allow ListDuePosts to return store.SocialPostData.
type SocialPostData = store.SocialPostData

func scanPost(row *sql.Row) (*store.SocialPostData, error) {
	var p store.SocialPostData
	var scheduled, published sql.NullTime
	var groupID, parentID sql.NullString
	err := row.Scan(
		&p.ID, &p.OwnerID, &p.Title, &p.Content, &p.PostType, &p.Status,
		&scheduled, &published, &groupID, &parentID,
		&p.Metadata, &p.Error, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if scheduled.Valid {
		p.ScheduledAt = &scheduled.Time
	}
	if published.Valid {
		p.PublishedAt = &published.Time
	}
	if groupID.Valid {
		u, _ := uuid.Parse(groupID.String)
		p.PostGroupID = &u
	}
	if parentID.Valid {
		u, _ := uuid.Parse(parentID.String)
		p.ParentPostID = &u
	}
	return &p, nil
}

func scanPosts(rows *sql.Rows) ([]store.SocialPostData, error) {
	var posts []store.SocialPostData
	for rows.Next() {
		var p store.SocialPostData
		var scheduled, published sql.NullTime
		var groupID, parentID sql.NullString
		if err := rows.Scan(
			&p.ID, &p.OwnerID, &p.Title, &p.Content, &p.PostType, &p.Status,
			&scheduled, &published, &groupID, &parentID,
			&p.Metadata, &p.Error, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if scheduled.Valid {
			p.ScheduledAt = &scheduled.Time
		}
		if published.Valid {
			p.PublishedAt = &published.Time
		}
		if groupID.Valid {
			u, _ := uuid.Parse(groupID.String)
			p.PostGroupID = &u
		}
		if parentID.Valid {
			u, _ := uuid.Parse(parentID.String)
			p.ParentPostID = &u
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func scanTargets(rows *sql.Rows) ([]store.SocialPostTargetData, error) {
	var targets []store.SocialPostTargetData
	for rows.Next() {
		var t store.SocialPostTargetData
		var published sql.NullTime
		if err := rows.Scan(
			&t.ID, &t.PostID, &t.AccountID, &t.PlatformPostID, &t.PlatformURL,
			&t.AdaptedContent, &t.Status, &t.Error, &published, &t.CreatedAt,
			&t.Platform, &t.PlatformUsername,
		); err != nil {
			return nil, err
		}
		if published.Valid {
			t.PublishedAt = &published.Time
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func scanMediaItems(rows *sql.Rows) ([]store.SocialPostMediaData, error) {
	var items []store.SocialPostMediaData
	for rows.Next() {
		var m store.SocialPostMediaData
		if err := rows.Scan(
			&m.ID, &m.PostID, &m.MediaType, &m.URL, &m.ThumbnailURL, &m.Filename,
			&m.MimeType, &m.FileSize, &m.Width, &m.Height, &m.DurationSeconds,
			&m.SortOrder, &m.Metadata, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

