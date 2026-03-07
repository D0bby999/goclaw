package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGNewsStore implements store.NewsStore backed by Postgres.
type PGNewsStore struct {
	db *sql.DB
}

func NewPGNewsStore(db *sql.DB) *PGNewsStore {
	return &PGNewsStore{db: db}
}

// ── Sources ──────────────────────────────────────────────────────────

func (s *PGNewsStore) CreateSource(ctx context.Context, src *store.NewsSource) error {
	if src.ID == uuid.Nil {
		src.ID = store.GenNewID()
	}
	now := nowUTC()
	src.CreatedAt = now
	src.UpdatedAt = now

	if len(src.Config) == 0 {
		src.Config = []byte(`{}`)
	}
	if src.ScrapeInterval == "" {
		src.ScrapeInterval = "daily"
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO news_sources (id, agent_id, name, source_type, config, enabled, scrape_interval, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		src.ID, src.AgentID, src.Name, src.SourceType, src.Config, src.Enabled, src.ScrapeInterval, now, now,
	)
	return err
}

func (s *PGNewsStore) GetSource(ctx context.Context, id uuid.UUID) (*store.NewsSource, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, name, source_type, config, enabled, scrape_interval, last_scraped_at, created_at, updated_at
		 FROM news_sources WHERE id = $1`, id)
	return scanSource(row)
}

func (s *PGNewsStore) ListSources(ctx context.Context, agentID uuid.UUID, enabledOnly bool) ([]store.NewsSource, error) {
	q := `SELECT id, agent_id, name, source_type, config, enabled, scrape_interval, last_scraped_at, created_at, updated_at
	      FROM news_sources WHERE agent_id = $1`
	if enabledOnly {
		q += ` AND enabled = true`
	}
	q += ` ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, q, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSources(rows)
}

func (s *PGNewsStore) UpdateSource(ctx context.Context, id uuid.UUID, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	patch["updated_at"] = nowUTC()
	return execMapUpdate(ctx, s.db, "news_sources", id, patch)
}

func (s *PGNewsStore) DeleteSource(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM news_sources WHERE id = $1`, id)
	return err
}

func (s *PGNewsStore) TouchSourceScraped(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE news_sources SET last_scraped_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

// ── Items ────────────────────────────────────────────────────────────

func (s *PGNewsStore) SaveItem(ctx context.Context, item *store.NewsItem) (bool, error) {
	if item.ID == uuid.Nil {
		item.ID = store.GenNewID()
	}
	now := nowUTC()
	item.CreatedAt = now
	if item.ScrapedAt.IsZero() {
		item.ScrapedAt = now
	}
	if item.Categories == nil {
		item.Categories = []string{}
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if len(item.Insights) == 0 {
		item.Insights = []byte(`{}`)
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO news_items (id, source_id, agent_id, url_hash, url, title, content, summary,
		 categories, tags, sentiment, insights, source_type, source_name, published_at, scraped_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 ON CONFLICT (agent_id, url_hash) DO NOTHING`,
		item.ID, nilUUID(item.SourceID), item.AgentID, item.URLHash, item.URL, item.Title,
		item.Content, item.Summary,
		pqStringArray(item.Categories), pqStringArray(item.Tags),
		item.Sentiment, item.Insights, item.SourceType, item.SourceName,
		nilTime(item.PublishedAt), item.ScrapedAt, now,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *PGNewsStore) GetItem(ctx context.Context, id uuid.UUID) (*store.NewsItem, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, source_id, agent_id, url_hash, url, title, content, summary,
		 categories, tags, sentiment, insights, source_type, source_name,
		 published_at, scraped_at, created_at
		 FROM news_items WHERE id = $1`, id)
	return scanItem(row)
}

func (s *PGNewsStore) ListItems(ctx context.Context, filter store.NewsItemFilter) ([]store.NewsItem, error) {
	var where []string
	var args []any
	idx := 1

	where = append(where, fmt.Sprintf("agent_id = $%d", idx))
	args = append(args, filter.AgentID)
	idx++

	if filter.SourceID != nil && *filter.SourceID != uuid.Nil {
		where = append(where, fmt.Sprintf("source_id = $%d", idx))
		args = append(args, *filter.SourceID)
		idx++
	}
	if len(filter.Categories) > 0 {
		where = append(where, fmt.Sprintf("categories && $%d", idx))
		args = append(args, pqStringArray(filter.Categories))
		idx++
	}
	if filter.Since != nil {
		where = append(where, fmt.Sprintf("scraped_at >= $%d", idx))
		args = append(args, *filter.Since)
		idx++
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	q := fmt.Sprintf(
		`SELECT id, source_id, agent_id, url_hash, url, title, content, summary,
		 categories, tags, sentiment, insights, source_type, source_name,
		 published_at, scraped_at, created_at
		 FROM news_items WHERE %s ORDER BY scraped_at DESC LIMIT %d OFFSET %d`,
		strings.Join(where, " AND "), limit, filter.Offset,
	)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func (s *PGNewsStore) CountItems(ctx context.Context, agentID uuid.UUID, since *time.Time) (int, error) {
	var count int
	if since != nil {
		err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM news_items WHERE agent_id = $1 AND scraped_at >= $2`,
			agentID, *since).Scan(&count)
		return count, err
	}
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM news_items WHERE agent_id = $1`, agentID).Scan(&count)
	return count, err
}

func (s *PGNewsStore) DeleteOldItems(ctx context.Context, agentID uuid.UUID, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM news_items WHERE agent_id = $1 AND scraped_at < $2`, agentID, olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ── Scan helpers ─────────────────────────────────────────────────────

func scanSource(row *sql.Row) (*store.NewsSource, error) {
	var src store.NewsSource
	var lastScraped sql.NullTime
	err := row.Scan(
		&src.ID, &src.AgentID, &src.Name, &src.SourceType, &src.Config,
		&src.Enabled, &src.ScrapeInterval, &lastScraped, &src.CreatedAt, &src.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastScraped.Valid {
		src.LastScrapedAt = &lastScraped.Time
	}
	return &src, nil
}

func scanSources(rows *sql.Rows) ([]store.NewsSource, error) {
	var sources []store.NewsSource
	for rows.Next() {
		var src store.NewsSource
		var lastScraped sql.NullTime
		if err := rows.Scan(
			&src.ID, &src.AgentID, &src.Name, &src.SourceType, &src.Config,
			&src.Enabled, &src.ScrapeInterval, &lastScraped, &src.CreatedAt, &src.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastScraped.Valid {
			src.LastScrapedAt = &lastScraped.Time
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

func scanItem(row *sql.Row) (*store.NewsItem, error) {
	var item store.NewsItem
	var sourceID sql.NullString
	var published sql.NullTime
	var catBytes, tagBytes []byte
	err := row.Scan(
		&item.ID, &sourceID, &item.AgentID, &item.URLHash, &item.URL, &item.Title,
		&item.Content, &item.Summary,
		&catBytes, &tagBytes,
		&item.Sentiment, &item.Insights, &item.SourceType, &item.SourceName,
		&published, &item.ScrapedAt, &item.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if sourceID.Valid {
		u, _ := uuid.Parse(sourceID.String)
		item.SourceID = &u
	}
	if published.Valid {
		item.PublishedAt = &published.Time
	}
	scanStringArray(catBytes, &item.Categories)
	scanStringArray(tagBytes, &item.Tags)
	return &item, nil
}

func scanItems(rows *sql.Rows) ([]store.NewsItem, error) {
	var items []store.NewsItem
	for rows.Next() {
		var item store.NewsItem
		var sourceID sql.NullString
		var published sql.NullTime
		var catBytes, tagBytes []byte
		if err := rows.Scan(
			&item.ID, &sourceID, &item.AgentID, &item.URLHash, &item.URL, &item.Title,
			&item.Content, &item.Summary,
			&catBytes, &tagBytes,
			&item.Sentiment, &item.Insights, &item.SourceType, &item.SourceName,
			&published, &item.ScrapedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if sourceID.Valid {
			u, _ := uuid.Parse(sourceID.String)
			item.SourceID = &u
		}
		if published.Valid {
			item.PublishedAt = &published.Time
		}
		scanStringArray(catBytes, &item.Categories)
		scanStringArray(tagBytes, &item.Tags)
		items = append(items, item)
	}
	return items, rows.Err()
}
