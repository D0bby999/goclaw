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

// NewsTool is a unified news tool that handles save, query, sources, and ideas actions.
type NewsTool struct {
	newsStore store.NewsStore
}

func NewNewsTool(ns store.NewsStore) *NewsTool {
	return &NewsTool{newsStore: ns}
}

func (t *NewsTool) Name() string { return "news" }

func (t *NewsTool) Description() string {
	return `Unified news management tool. Actions:
- save: Save a scraped news item (deduplicates by URL)
- query: Search/filter saved news items by categories, date, keywords
- sources: List configured news sources for scraping
- ideas: Extract app ideas, business ideas, and key points from news insights

Examples: "tìm tin tức hôm nay", "idea tốt nhất tuần này", "tin mới nhất về AI", "lưu bài viết này"`
}

func (t *NewsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"save", "query", "sources", "ideas"},
				"description": "Action to perform",
			},
			// save params
			"url":          map[string]interface{}{"type": "string", "description": "[save] Article URL"},
			"title":        map[string]interface{}{"type": "string", "description": "[save] Article title"},
			"content":      map[string]interface{}{"type": "string", "description": "[save] Raw content (truncated to 5000 chars)"},
			"summary":      map[string]interface{}{"type": "string", "description": "[save] AI-generated summary"},
			"sentiment":    map[string]interface{}{"type": "string", "enum": []string{"positive", "negative", "neutral"}, "description": "[save] Article sentiment"},
			"insights":     map[string]interface{}{"type": "object", "description": "[save] {app_ideas: [], biz_ideas: [], key_points: []}"},
			"source_type":  map[string]interface{}{"type": "string", "description": "[save] Source type: reddit, website, twitter, rss"},
			"source_name":  map[string]interface{}{"type": "string", "description": "[save] Human-readable source name"},
			"published_at": map[string]interface{}{"type": "string", "description": "[save] Original publish date (ISO 8601)"},
			// shared filter params
			"categories": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "[save/query/ideas] Topic categories"},
			"tags":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "[save/query] Tags"},
			"since":      map[string]interface{}{"type": "string", "description": "[query/ideas] Time window: '1h', '24h', '7d', '30d', or ISO 8601. Default: '24h'"},
			"limit":      map[string]interface{}{"type": "number", "description": "[query/ideas] Max results (default 50/100)"},
			"offset":     map[string]interface{}{"type": "number", "description": "[query] Pagination offset"},
			"source_id":  map[string]interface{}{"type": "string", "description": "[query] Filter by source UUID"},
			// ideas params
			"idea_type": map[string]interface{}{"type": "string", "enum": []string{"all", "app", "biz", "key_points"}, "description": "[ideas] Filter idea type. Default: 'all'"},
			// sources params
			"enabled_only": map[string]interface{}{"type": "boolean", "description": "[sources] Only enabled sources (default true)"},
			"category":     map[string]interface{}{"type": "string", "description": "[sources] Filter by category/sector (e.g. finance, crypto, tech)"},
		},
		"required": []string{"action"},
	}
}

func (t *NewsTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	action, _ := args["action"].(string)
	switch action {
	case "save":
		return t.execSave(ctx, args)
	case "query":
		return t.execQuery(ctx, args)
	case "sources":
		return t.execSources(ctx, args)
	case "ideas":
		return t.execIdeas(ctx, args)
	default:
		return ErrorResult("action must be one of: save, query, sources, ideas")
	}
}

// ── save ─────────────────────────────────────────────────────────────

func (t *NewsTool) execSave(ctx context.Context, args map[string]interface{}) *Result {
	rawURL, _ := args["url"].(string)
	title, _ := args["title"].(string)
	if rawURL == "" || title == "" {
		return ErrorResult("url and title are required for save action")
	}

	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	item := &store.NewsItem{
		AgentID: agentID,
		URLHash: hashURL(rawURL),
		URL:     rawURL,
		Title:   title,
	}

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
	item.Categories = parseStringSlice(args, "categories")
	item.Tags = parseStringSlice(args, "tags")

	if insightsRaw, ok := args["insights"]; ok {
		if b, err := json.Marshal(insightsRaw); err == nil {
			item.Insights = b
		}
	}
	if pubStr, ok := args["published_at"].(string); ok && pubStr != "" {
		if pt, err := time.Parse(time.RFC3339, pubStr); err == nil {
			item.PublishedAt = &pt
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

// ── query ────────────────────────────────────────────────────────────

func (t *NewsTool) execQuery(ctx context.Context, args map[string]interface{}) *Result {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	filter := store.NewsItemFilter{AgentID: agentID}
	filter.Categories = parseStringSlice(args, "categories")

	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		if st, err := parseSince(sinceStr); err == nil {
			filter.Since = &st
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

// ── sources ──────────────────────────────────────────────────────────

func (t *NewsTool) execSources(ctx context.Context, args map[string]interface{}) *Result {
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

	// optionally filter by category client-side
	categoryFilter, _ := args["category"].(string)
	if categoryFilter != "" {
		var filtered []store.NewsSource
		for _, src := range sources {
			if src.Category != nil && strings.EqualFold(*src.Category, categoryFilter) {
				filtered = append(filtered, src)
			}
		}
		sources = filtered
		if len(sources) == 0 {
			return NewResult(fmt.Sprintf("No news sources found for category: %s", categoryFilter))
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configured news sources (%d):\n\n", len(sources)))
	for i, src := range sources {
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s]\n", i+1, src.Name, src.SourceType))
		if src.Category != nil {
			sb.WriteString(fmt.Sprintf("   - Category: %s\n", *src.Category))
		}
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

// ── ideas ────────────────────────────────────────────────────────────

// newsInsights represents the structured insights JSONB from news_items.
type newsInsights struct {
	AppIdeas  []string `json:"app_ideas"`
	BizIdeas  []string `json:"biz_ideas"`
	KeyPoints []string `json:"key_points"`
}

type ideaEntry struct {
	Idea        string
	SourceTitle string
	SourceURL   string
}

func (t *NewsTool) execIdeas(ctx context.Context, args map[string]interface{}) *Result {
	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}

	sinceStr := "24h"
	if v, ok := args["since"].(string); ok && v != "" {
		sinceStr = v
	}
	sinceTime, err := parseSince(sinceStr)
	if err != nil {
		return ErrorResult(fmt.Sprintf("invalid since value: %v", err))
	}

	filter := store.NewsItemFilter{
		AgentID: agentID,
		Since:   &sinceTime,
		Limit:   100,
	}
	filter.Categories = parseStringSlice(args, "categories")
	if lim, ok := args["limit"].(float64); ok && lim > 0 {
		filter.Limit = int(lim)
	}

	ideaType := "all"
	if v, ok := args["idea_type"].(string); ok && v != "" {
		ideaType = v
	}

	items, err := t.newsStore.ListItems(ctx, filter)
	if err != nil {
		return ErrorResult(fmt.Sprintf("query failed: %v", err))
	}
	if len(items) == 0 {
		return NewResult(fmt.Sprintf("No news items found in the last %s.", sinceStr))
	}

	var appIdeas, bizIdeas, keyPoints []ideaEntry
	for _, it := range items {
		if len(it.Insights) == 0 || string(it.Insights) == "{}" || string(it.Insights) == "null" {
			continue
		}
		var ins newsInsights
		if err := json.Unmarshal(it.Insights, &ins); err != nil {
			continue
		}
		for _, idea := range ins.AppIdeas {
			appIdeas = append(appIdeas, ideaEntry{Idea: idea, SourceTitle: it.Title, SourceURL: it.URL})
		}
		for _, idea := range ins.BizIdeas {
			bizIdeas = append(bizIdeas, ideaEntry{Idea: idea, SourceTitle: it.Title, SourceURL: it.URL})
		}
		for _, kp := range ins.KeyPoints {
			keyPoints = append(keyPoints, ideaEntry{Idea: kp, SourceTitle: it.Title, SourceURL: it.URL})
		}
	}

	if len(appIdeas)+len(bizIdeas)+len(keyPoints) == 0 {
		return NewResult(fmt.Sprintf("Found %d news items but none have structured ideas/insights.", len(items)))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Ideas from %d news items (last %s):\n\n", len(items), sinceStr))

	writeSection := func(title string, entries []ideaEntry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", title, len(entries)))
		for i, e := range entries {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, e.Idea))
			sb.WriteString(fmt.Sprintf("   _Source: %s_ (%s)\n\n", e.SourceTitle, e.SourceURL))
		}
	}

	switch ideaType {
	case "app":
		writeSection("App Ideas", appIdeas)
	case "biz":
		writeSection("Business Ideas", bizIdeas)
	case "key_points":
		writeSection("Key Points", keyPoints)
	default:
		writeSection("App Ideas", appIdeas)
		writeSection("Business Ideas", bizIdeas)
		writeSection("Key Points", keyPoints)
	}

	return NewResult(sb.String())
}

// ── Helpers ──────────────────────────────────────────────────────────

func parseStringSlice(args map[string]interface{}, key string) []string {
	raw, ok := args[key].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func hashURL(rawURL string) string {
	normalized := normalizeURL(rawURL)
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""

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
	u.Path = strings.TrimRight(u.Path, "/")

	return u.String()
}

func parseSince(s string) (time.Time, error) {
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
	return time.Parse(time.RFC3339, s)
}
