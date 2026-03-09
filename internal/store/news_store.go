package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// NewsSource represents a configured scraping target.
type NewsSource struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	Name           string          `json:"name"`
	SourceType     string          `json:"sourceType"`     // "reddit", "website", "twitter", "rss"
	Config         json.RawMessage `json:"config"`         // actor-specific config
	Enabled        bool            `json:"enabled"`
	ScrapeInterval string          `json:"scrapeInterval"` // "hourly", "daily", "weekly"
	Category       *string         `json:"category,omitempty"` // sector: finance, crypto, tech, etc.
	LastScrapedAt  *time.Time      `json:"lastScrapedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// NewsItem represents a scraped and AI-analyzed article.
type NewsItem struct {
	ID          uuid.UUID       `json:"id"`
	SourceID    *uuid.UUID      `json:"sourceId,omitempty"`
	AgentID     uuid.UUID       `json:"agentId"`
	URLHash     string          `json:"urlHash"`
	URL         string          `json:"url"`
	Title       string          `json:"title"`
	Content     *string         `json:"content,omitempty"`
	Summary     *string         `json:"summary,omitempty"`
	Categories  []string        `json:"categories"`
	Tags        []string        `json:"tags"`
	Sentiment   *string         `json:"sentiment,omitempty"` // "positive", "negative", "neutral"
	Insights    json.RawMessage `json:"insights"`            // {app_ideas, biz_ideas, key_points}
	SourceType  *string         `json:"sourceType,omitempty"`
	SourceName  *string         `json:"sourceName,omitempty"`
	PublishedAt *time.Time      `json:"publishedAt,omitempty"`
	ScrapedAt   time.Time       `json:"scrapedAt"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// NewsItemFilter specifies criteria for listing news items.
type NewsItemFilter struct {
	AgentID    uuid.UUID
	SourceID   *uuid.UUID
	Categories []string   // ANY match
	Since      *time.Time // items scraped after this time
	Limit      int
	Offset     int
}

// NewsStore manages news sources and scraped items.
type NewsStore interface {
	// Sources
	CreateSource(ctx context.Context, src *NewsSource) error
	GetSource(ctx context.Context, id uuid.UUID) (*NewsSource, error)
	ListSources(ctx context.Context, agentID uuid.UUID, enabledOnly bool) ([]NewsSource, error)
	UpdateSource(ctx context.Context, id uuid.UUID, patch map[string]any) error
	DeleteSource(ctx context.Context, id uuid.UUID) error
	TouchSourceScraped(ctx context.Context, id uuid.UUID) error

	// Items — SaveItem upserts with dedup by (agent_id, url_hash)
	SaveItem(ctx context.Context, item *NewsItem) (created bool, err error)
	GetItem(ctx context.Context, id uuid.UUID) (*NewsItem, error)
	ListItems(ctx context.Context, filter NewsItemFilter) ([]NewsItem, error)
	CountItems(ctx context.Context, agentID uuid.UUID, since *time.Time) (int, error)
	DeleteOldItems(ctx context.Context, agentID uuid.UUID, olderThan time.Time) (int64, error)
}
