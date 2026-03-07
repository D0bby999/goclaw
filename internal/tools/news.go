package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ── NewsSaveTool ─────────────────────────────────────────────────────

// NewsSaveTool lets agents save scraped and analyzed news items with URL deduplication.
type NewsSaveTool struct {
	newsStore store.NewsStore
}

func NewNewsSaveTool(ns store.NewsStore) *NewsSaveTool {
	return &NewsSaveTool{newsStore: ns}
}

func (t *NewsSaveTool) Name() string { return "news_save" }

func (t *NewsSaveTool) Description() string {
	return `Save a scraped and analyzed news item. Automatically deduplicates by URL.
Returns whether the item was newly created or already existed (duplicate).
Use after scraping and analyzing an article to persist the results.`
}

func (t *NewsSaveTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url":          map[string]interface{}{"type": "string", "description": "Article URL"},
			"title":        map[string]interface{}{"type": "string", "description": "Article title"},
			"content":      map[string]interface{}{"type": "string", "description": "Raw article content (will be truncated to 5000 chars)"},
			"summary":      map[string]interface{}{"type": "string", "description": "AI-generated 2-3 sentence summary"},
			"categories":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Topic categories (e.g. tech, ai, startup, finance)"},
			"tags":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Specific tags (e.g. gpt-5, funding-round, saas)"},
			"sentiment":    map[string]interface{}{"type": "string", "enum": []string{"positive", "negative", "neutral"}, "description": "Article sentiment"},
			"insights":     map[string]interface{}{"type": "object", "description": "Structured insights: {app_ideas: [], biz_ideas: [], key_points: []}"},
			"source_type":  map[string]interface{}{"type": "string", "description": "Source type: reddit, website, twitter, rss"},
			"source_name":  map[string]interface{}{"type": "string", "description": "Human-readable source name (e.g. r/technology, TechCrunch)"},
			"published_at": map[string]interface{}{"type": "string", "description": "Original publish date (ISO 8601)"},
		},
		"required": []string{"url", "title"},
	}
}

func (t *NewsSaveTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	rawURL, _ := args["url"].(string)
	title, _ := args["title"].(string)
	if rawURL == "" || title == "" {
		return ErrorResult("url and title are required")
	}

	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	urlHash := hashURL(rawURL)

	item := &store.NewsItem{
		AgentID: agentID,
		URLHash: urlHash,
		URL:     rawURL,
		Title:   title,
	}

	// Optional fields
	if v, ok := args["content"].(string); ok {
		if len(v) > 5000 {
			v = v[:5000]
		}
		item.Content = &v
	}
	if v, ok := args["summary"].(string); ok {
		item.Summary = &v
	}
	if v, ok := args["sentiment"].(string); ok {
		item.Sentiment = &v
	}
	if v, ok := args["source_type"].(string); ok {
		item.SourceType = &v
	}
	if v, ok := args["source_name"].(string); ok {
		item.SourceName = &v
	}

	if cats, ok := args["categories"].([]interface{}); ok {
		for _, c := range cats {
			if s, ok := c.(string); ok {
				item.Categories = append(item.Categories, s)
			}
		}
	}
	if tags, ok := args["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				item.Tags = append(item.Tags, s)
			}
		}
	}

	if insightsRaw, ok := args["insights"]; ok {
		if b, err := json.Marshal(insightsRaw); err == nil {
			item.Insights = b
		}
	}

	if pubStr, ok := args["published_at"].(string); ok && pubStr != "" {
		if t, err := time.Parse(time.RFC3339, pubStr); err == nil {
			item.PublishedAt = &t
		}
	}

	created, err := t.newsStore.SaveItem(ctx, item)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to save news item: %v", err))
	}

	if created {
		return NewResult(fmt.Sprintf("Saved: %s", title))
	}
	return NewResult(fmt.Sprintf("Duplicate, skipped: %s", title))
}

// ── NewsQueryTool ────────────────────────────────────────────────────

// NewsQueryTool lets agents query saved news items for building digests or checking duplicates.
type NewsQueryTool struct {
	newsStore store.NewsStore
}

func NewNewsQueryTool(ns store.NewsStore) *NewsQueryTool {
	return &NewsQueryTool{newsStore: ns}
}

func (t *NewsQueryTool) Name() string { return "news_query" }

func (t *NewsQueryTool) Description() string {
	return `Query saved news items. Filter by categories, date range, or source.
Use to build digests, check what's already been scraped, or analyze trends.`
}

func (t *NewsQueryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"categories": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Filter by categories (ANY match)"},
			"since":      map[string]interface{}{"type": "string", "description": "Time window: '1h', '24h', '7d', '30d', or ISO 8601 timestamp"},
			"source_id":  map[string]interface{}{"type": "string", "description": "Filter by source UUID"},
			"limit":      map[string]interface{}{"type": "number", "description": "Max results (default 50)"},
			"offset":     map[string]interface{}{"type": "number", "description": "Pagination offset"},
		},
	}
}

func (t *NewsQueryTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	filter := store.NewsItemFilter{AgentID: agentID}

	if cats, ok := args["categories"].([]interface{}); ok {
		for _, c := range cats {
			if s, ok := c.(string); ok {
				filter.Categories = append(filter.Categories, s)
			}
		}
	}

	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		if t, err := parseSince(sinceStr); err == nil {
			filter.Since = &t
		}
	}

	if srcID, ok := args["source_id"].(string); ok && srcID != "" {
		if u, err := uuid.Parse(srcID); err == nil {
			filter.SourceID = &u
		}
	}

	if lim, ok := args["limit"].(float64); ok {
		filter.Limit = int(lim)
	}
	if off, ok := args["offset"].(float64); ok {
		filter.Offset = int(off)
	}

	items, err := t.newsStore.ListItems(ctx, filter)
	if err != nil {
		return ErrorResult(fmt.Sprintf("query failed: %v", err))
	}

	if len(items) == 0 {
		return NewResult("No news items found matching the criteria.")
	}

	// Format for LLM consumption
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d news items:\n\n", len(items)))
	for i, it := range items {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, it.Title))
		sb.WriteString(fmt.Sprintf("- URL: %s\n", it.URL))
		if it.Summary != nil {
			sb.WriteString(fmt.Sprintf("- Summary: %s\n", *it.Summary))
		}
		if len(it.Categories) > 0 {
			sb.WriteString(fmt.Sprintf("- Categories: %s\n", strings.Join(it.Categories, ", ")))
		}
		if len(it.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- Tags: %s\n", strings.Join(it.Tags, ", ")))
		}
		if it.Sentiment != nil {
			sb.WriteString(fmt.Sprintf("- Sentiment: %s\n", *it.Sentiment))
		}
		sb.WriteString(fmt.Sprintf("- Scraped: %s\n\n", it.ScrapedAt.Format(time.RFC3339)))
	}
	return NewResult(sb.String())
}

// ── NewsSourcesTool ──────────────────────────────────────────────────

// NewsSourcesTool lets agents list configured news sources for scraping.
type NewsSourcesTool struct {
	newsStore store.NewsStore
}

func NewNewsSourcesTool(ns store.NewsStore) *NewsSourcesTool {
	return &NewsSourcesTool{newsStore: ns}
}

func (t *NewsSourcesTool) Name() string { return "news_sources" }

func (t *NewsSourcesTool) Description() string {
	return `List configured news sources for this agent. Returns source names, types, and scraping configs.
Use this to know which sources to scrape during a news digest run.`
}

func (t *NewsSourcesTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enabled_only": map[string]interface{}{"type": "boolean", "description": "Only show enabled sources (default true)"},
		},
	}
}

func (t *NewsSourcesTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	enabledOnly := true
	if v, ok := args["enabled_only"].(bool); ok {
		enabledOnly = v
	}

	sources, err := t.newsStore.ListSources(ctx, agentID, enabledOnly)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list sources: %v", err))
	}

	if len(sources) == 0 {
		return NewResult("No news sources configured. Use the news management API to add sources.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configured news sources (%d):\n\n", len(sources)))
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, src.Name, src.SourceType))
		sb.WriteString(fmt.Sprintf("   - Config: %s\n", string(src.Config)))
		sb.WriteString(fmt.Sprintf("   - Interval: %s\n", src.ScrapeInterval))
		if src.LastScrapedAt != nil {
			sb.WriteString(fmt.Sprintf("   - Last scraped: %s\n", src.LastScrapedAt.Format(time.RFC3339)))
		} else {
			sb.WriteString("   - Last scraped: never\n")
		}
		sb.WriteString("\n")
	}
	return NewResult(sb.String())
}

// ── Helpers ──────────────────────────────────────────────────────────

// hashURL normalizes a URL and returns its SHA-256 hex digest.
func hashURL(rawURL string) string {
	normalized := normalizeURL(rawURL)
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

// normalizeURL strips tracking params, fragment, trailing slash, and lowercases scheme+host.
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

	// Remove tracking parameters
	q := u.Query()
	trackingPrefixes := []string{"utm_", "fbclid", "ref", "source", "gi"}
	for key := range q {
		for _, prefix := range trackingPrefixes {
			if strings.HasPrefix(strings.ToLower(key), prefix) {
				q.Del(key)
				break
			}
		}
	}
	u.RawQuery = q.Encode()

	// Remove trailing slash
	u.Path = strings.TrimRight(u.Path, "/")

	return u.String()
}

// parseSince parses duration strings like "24h", "7d", "30d" or ISO 8601 timestamps.
func parseSince(s string) (time.Time, error) {
	// Try duration shortcuts: Xh, Xd
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err == nil {
			return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	// Try ISO 8601
	return time.Parse(time.RFC3339, s)
}
