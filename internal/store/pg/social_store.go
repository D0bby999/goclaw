package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/crypto"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGSocialStore implements store.SocialStore backed by Postgres.
type PGSocialStore struct {
	db     *sql.DB
	encKey string
}

func NewPGSocialStore(db *sql.DB, encKey string) *PGSocialStore {
	return &PGSocialStore{db: db, encKey: encKey}
}

// ── Accounts ──────────────────────────────────────────────────────────

func (s *PGSocialStore) CreateAccount(ctx context.Context, a *store.SocialAccountData) error {
	if a.ID == uuid.Nil {
		a.ID = store.GenNewID()
	}
	now := nowUTC()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.ConnectedAt.IsZero() {
		a.ConnectedAt = now
	}
	if a.Status == "" {
		a.Status = store.SocialAccountStatusActive
	}
	if len(a.Metadata) == 0 {
		a.Metadata = json.RawMessage(`{}`)
	}

	accessToken := a.AccessToken
	if s.encKey != "" && accessToken != "" {
		enc, err := crypto.Encrypt(accessToken, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt access_token: %w", err)
		}
		accessToken = enc
	}
	var refreshToken *string
	if a.RefreshToken != nil && *a.RefreshToken != "" {
		rt := *a.RefreshToken
		if s.encKey != "" {
			enc, err := crypto.Encrypt(rt, s.encKey)
			if err != nil {
				return fmt.Errorf("encrypt refresh_token: %w", err)
			}
			rt = enc
		}
		refreshToken = &rt
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_accounts (id, owner_id, platform, platform_user_id, platform_username,
		 display_name, avatar_url, access_token, refresh_token, token_expires_at, scopes,
		 metadata, status, connected_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		a.ID, a.OwnerID, a.Platform, a.PlatformUserID, a.PlatformUsername,
		a.DisplayName, a.AvatarURL, accessToken, refreshToken, nilTime(a.TokenExpiresAt),
		pqStringArray(a.Scopes), a.Metadata, a.Status, a.ConnectedAt, now, now,
	)
	return err
}

func (s *PGSocialStore) GetAccount(ctx context.Context, id uuid.UUID) (*store.SocialAccountData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, platform, platform_user_id, platform_username, display_name,
		 avatar_url, access_token, refresh_token, token_expires_at, scopes, metadata,
		 status, connected_at, created_at, updated_at
		 FROM social_accounts WHERE id = $1 AND deleted_at IS NULL`, id)
	return s.scanAccount(row)
}

// Allowed columns for social_accounts updates (prevents SQL injection via map keys).
var allowedAccountCols = map[string]bool{
	"platform_username": true, "display_name": true, "avatar_url": true,
	"access_token": true, "refresh_token": true, "token_expires_at": true,
	"scopes": true, "metadata": true, "status": true, "updated_at": true,
}

func (s *PGSocialStore) UpdateAccount(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedAccountCols)
	if len(filtered) == 0 {
		return nil
	}
	// Encrypt tokens if being updated
	if at, ok := filtered["access_token"].(string); ok && s.encKey != "" && at != "" {
		enc, err := crypto.Encrypt(at, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt access_token: %w", err)
		}
		filtered["access_token"] = enc
	}
	if rt, ok := filtered["refresh_token"].(string); ok && s.encKey != "" && rt != "" {
		enc, err := crypto.Encrypt(rt, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt refresh_token: %w", err)
		}
		filtered["refresh_token"] = enc
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "social_accounts", id, filtered)
}

func (s *PGSocialStore) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE social_accounts SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *PGSocialStore) ListAccounts(ctx context.Context, ownerID string) ([]store.SocialAccountData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, platform, platform_user_id, platform_username, display_name,
		 avatar_url, access_token, refresh_token, token_expires_at, scopes, metadata,
		 status, connected_at, created_at, updated_at
		 FROM social_accounts WHERE owner_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanAccounts(rows)
}

func (s *PGSocialStore) ListAccountsByPlatform(ctx context.Context, ownerID, platform string) ([]store.SocialAccountData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, platform, platform_user_id, platform_username, display_name,
		 avatar_url, access_token, refresh_token, token_expires_at, scopes, metadata,
		 status, connected_at, created_at, updated_at
		 FROM social_accounts WHERE owner_id = $1 AND platform = $2 AND deleted_at IS NULL
		 ORDER BY created_at`, ownerID, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanAccounts(rows)
}

// ── Scan helpers (accounts) ───────────────────────────────────────────

func (s *PGSocialStore) scanAccount(row *sql.Row) (*store.SocialAccountData, error) {
	var a store.SocialAccountData
	var accessToken, refreshToken sql.NullString
	var tokenExpires sql.NullTime
	var scopeBytes []byte
	err := row.Scan(
		&a.ID, &a.OwnerID, &a.Platform, &a.PlatformUserID, &a.PlatformUsername,
		&a.DisplayName, &a.AvatarURL, &accessToken, &refreshToken, &tokenExpires,
		&scopeBytes, &a.Metadata, &a.Status, &a.ConnectedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.decryptAccountTokens(&a, accessToken, refreshToken)
	if tokenExpires.Valid {
		a.TokenExpiresAt = &tokenExpires.Time
	}
	scanStringArray(scopeBytes, &a.Scopes)
	return &a, nil
}

func (s *PGSocialStore) scanAccounts(rows *sql.Rows) ([]store.SocialAccountData, error) {
	var accounts []store.SocialAccountData
	for rows.Next() {
		var a store.SocialAccountData
		var accessToken, refreshToken sql.NullString
		var tokenExpires sql.NullTime
		var scopeBytes []byte
		if err := rows.Scan(
			&a.ID, &a.OwnerID, &a.Platform, &a.PlatformUserID, &a.PlatformUsername,
			&a.DisplayName, &a.AvatarURL, &accessToken, &refreshToken, &tokenExpires,
			&scopeBytes, &a.Metadata, &a.Status, &a.ConnectedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.decryptAccountTokens(&a, accessToken, refreshToken)
		if tokenExpires.Valid {
			a.TokenExpiresAt = &tokenExpires.Time
		}
		scanStringArray(scopeBytes, &a.Scopes)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *PGSocialStore) decryptAccountTokens(a *store.SocialAccountData, at, rt sql.NullString) {
	if at.Valid {
		a.AccessToken = at.String
		if s.encKey != "" {
			if dec, err := crypto.Decrypt(at.String, s.encKey); err == nil {
				a.AccessToken = dec
			}
		}
	}
	if rt.Valid {
		val := rt.String
		if s.encKey != "" {
			if dec, err := crypto.Decrypt(rt.String, s.encKey); err == nil {
				val = dec
			}
		}
		a.RefreshToken = &val
	}
}

// ── Posts ──────────────────────────────────────────────────────────────

func (s *PGSocialStore) CreatePost(ctx context.Context, p *store.SocialPostData) error {
	if p.ID == uuid.Nil {
		p.ID = store.GenNewID()
	}
	now := nowUTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.Status == "" {
		p.Status = store.SocialPostStatusDraft
	}
	if p.PostType == "" {
		p.PostType = "post"
	}
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage(`{}`)
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_posts (id, owner_id, title, content, post_type, status,
		 scheduled_at, published_at, post_group_id, parent_post_id, metadata, error,
		 created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		p.ID, p.OwnerID, p.Title, p.Content, p.PostType, p.Status,
		nilTime(p.ScheduledAt), nilTime(p.PublishedAt),
		nilUUID(p.PostGroupID), nilUUID(p.ParentPostID),
		p.Metadata, p.Error, now, now,
	)
	return err
}

func (s *PGSocialStore) GetPost(ctx context.Context, id uuid.UUID) (*store.SocialPostData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, title, content, post_type, status, scheduled_at, published_at,
		 post_group_id, parent_post_id, metadata, error, created_at, updated_at
		 FROM social_posts WHERE id = $1 AND deleted_at IS NULL`, id)
	p, err := scanPost(row)
	if err != nil {
		return nil, err
	}
	// Join targets + media
	p.Targets, _ = s.ListTargets(ctx, id)
	p.Media, _ = s.ListMedia(ctx, id)
	return p, nil
}

// Allowed columns for social_posts updates.
var allowedPostCols = map[string]bool{
	"title": true, "content": true, "post_type": true, "status": true,
	"scheduled_at": true, "published_at": true, "post_group_id": true,
	"parent_post_id": true, "metadata": true, "error": true, "updated_at": true,
}

func (s *PGSocialStore) UpdatePost(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedPostCols)
	if len(filtered) == 0 {
		return nil
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "social_posts", id, filtered)
}

func (s *PGSocialStore) DeletePost(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE social_posts SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *PGSocialStore) ListPosts(ctx context.Context, ownerID string, status string, limit, offset int) ([]store.SocialPostData, int, error) {
	where := []string{"owner_id = $1", "deleted_at IS NULL"}
	args := []any{ownerID}
	idx := 2

	if status != "" {
		where = append(where, fmt.Sprintf("status = $%d", idx))
		args = append(args, status)
		idx++
	}

	if limit <= 0 {
		limit = 50
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	_ = s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM social_posts WHERE %s", whereClause),
		args...).Scan(&total)

	q := fmt.Sprintf(
		`SELECT id, owner_id, title, content, post_type, status, scheduled_at, published_at,
		 post_group_id, parent_post_id, metadata, error, created_at, updated_at
		 FROM social_posts WHERE %s ORDER BY created_at DESC LIMIT %d OFFSET %d`,
		whereClause, limit, offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	posts, err := scanPosts(rows)
	return posts, total, err
}

func (s *PGSocialStore) ListDuePosts(ctx context.Context) ([]SocialPostData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, title, content, post_type, status, scheduled_at, published_at,
		 post_group_id, parent_post_id, metadata, error, created_at, updated_at
		 FROM social_posts
		 WHERE status = 'scheduled' AND scheduled_at <= NOW() AND deleted_at IS NULL
		 ORDER BY scheduled_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPosts(rows)
}
