package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/crypto"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ── Pages ─────────────────────────────────────────────────────────────

func (s *PGSocialStore) CreatePage(ctx context.Context, p *store.SocialPageData) error {
	if p.ID == uuid.Nil {
		p.ID = store.GenNewID()
	}
	now := nowUTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.PageType == "" {
		p.PageType = "page"
	}
	if p.Status == "" {
		p.Status = "active"
	}
	if len(p.Metadata) == 0 {
		p.Metadata = json.RawMessage(`{}`)
	}

	pageToken := p.PageToken
	if s.encKey != "" && pageToken != "" {
		enc, err := crypto.Encrypt(pageToken, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt page_token: %w", err)
		}
		pageToken = enc
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO social_pages (id, account_id, page_id, page_name, page_token, page_type,
		 avatar_url, is_default, metadata, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (account_id, page_id) DO UPDATE SET
		   page_name = EXCLUDED.page_name,
		   page_token = EXCLUDED.page_token,
		   page_type = EXCLUDED.page_type,
		   avatar_url = EXCLUDED.avatar_url,
		   metadata = EXCLUDED.metadata,
		   status = EXCLUDED.status,
		   updated_at = EXCLUDED.updated_at`,
		p.ID, p.AccountID, p.PageID, p.PageName, nilStr(pageToken), p.PageType,
		p.AvatarURL, p.IsDefault, p.Metadata, p.Status, now, now,
	)
	return err
}

func (s *PGSocialStore) ListPages(ctx context.Context, accountID uuid.UUID) ([]store.SocialPageData, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, page_id, page_name, page_token, page_type,
		 avatar_url, is_default, metadata, status, created_at, updated_at
		 FROM social_pages WHERE account_id = $1
		 ORDER BY is_default DESC, created_at`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanPages(rows)
}

func (s *PGSocialStore) GetPage(ctx context.Context, id uuid.UUID) (*store.SocialPageData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, page_id, page_name, page_token, page_type,
		 avatar_url, is_default, metadata, status, created_at, updated_at
		 FROM social_pages WHERE id = $1`, id)
	return s.scanPage(row)
}

func (s *PGSocialStore) GetDefaultPage(ctx context.Context, accountID uuid.UUID) (*store.SocialPageData, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, account_id, page_id, page_name, page_token, page_type,
		 avatar_url, is_default, metadata, status, created_at, updated_at
		 FROM social_pages WHERE account_id = $1 AND is_default = true
		 LIMIT 1`, accountID)
	return s.scanPage(row)
}

var allowedPageCols = map[string]bool{
	"page_name": true, "page_token": true, "page_type": true,
	"avatar_url": true, "is_default": true, "metadata": true,
	"status": true, "updated_at": true,
}

func (s *PGSocialStore) UpdatePage(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	filtered := filterCols(updates, allowedPageCols)
	if len(filtered) == 0 {
		return nil
	}
	if pt, ok := filtered["page_token"].(string); ok && s.encKey != "" && pt != "" {
		enc, err := crypto.Encrypt(pt, s.encKey)
		if err != nil {
			return fmt.Errorf("encrypt page_token: %w", err)
		}
		filtered["page_token"] = enc
	}
	filtered["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "social_pages", id, filtered)
}

func (s *PGSocialStore) DeletePage(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM social_pages WHERE id = $1`, id)
	return err
}

func (s *PGSocialStore) SetDefaultPage(ctx context.Context, accountID, pageID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all defaults for this account.
	if _, err := tx.ExecContext(ctx,
		`UPDATE social_pages SET is_default = false, updated_at = NOW() WHERE account_id = $1`,
		accountID); err != nil {
		return err
	}
	// Set the chosen page as default.
	res, err := tx.ExecContext(ctx,
		`UPDATE social_pages SET is_default = true, updated_at = NOW() WHERE id = $1 AND account_id = $2`,
		pageID, accountID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("page not found")
	}
	return tx.Commit()
}

// ── Scan helpers (pages) ──────────────────────────────────────────────

func (s *PGSocialStore) scanPage(row *sql.Row) (*store.SocialPageData, error) {
	var p store.SocialPageData
	var pageToken sql.NullString
	err := row.Scan(
		&p.ID, &p.AccountID, &p.PageID, &p.PageName, &pageToken, &p.PageType,
		&p.AvatarURL, &p.IsDefault, &p.Metadata, &p.Status, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.decryptPageToken(&p, pageToken)
	return &p, nil
}

func (s *PGSocialStore) scanPages(rows *sql.Rows) ([]store.SocialPageData, error) {
	var pages []store.SocialPageData
	for rows.Next() {
		var p store.SocialPageData
		var pageToken sql.NullString
		if err := rows.Scan(
			&p.ID, &p.AccountID, &p.PageID, &p.PageName, &pageToken, &p.PageType,
			&p.AvatarURL, &p.IsDefault, &p.Metadata, &p.Status, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.decryptPageToken(&p, pageToken)
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (s *PGSocialStore) decryptPageToken(p *store.SocialPageData, pt sql.NullString) {
	if pt.Valid {
		p.PageToken = pt.String
		if s.encKey != "" {
			if dec, err := crypto.Decrypt(pt.String, s.encKey); err == nil {
				p.PageToken = dec
			}
		}
	}
}
