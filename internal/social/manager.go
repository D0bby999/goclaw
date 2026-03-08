package social

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// Manager orchestrates publishing, scheduling, and token refresh.
type Manager struct {
	store      store.SocialStore
	encKey     string
	logger     *slog.Logger
	browserMgr *browser.Manager
	cookieSrc  CookieSource
}

// NewManager creates a new social manager.
func NewManager(store store.SocialStore, encKey string) *Manager {
	return &Manager{
		store:  store,
		encKey: encKey,
		logger: slog.Default().With("component", "social"),
	}
}

// SetBrowser wires an optional browser manager for platforms that use automation.
func (m *Manager) SetBrowser(mgr *browser.Manager) {
	m.browserMgr = mgr
}

// SetCookieStore wires an optional cookie source for browser-based authentication.
func (m *Manager) SetCookieStore(cs CookieSource) {
	m.cookieSrc = cs
}

// Store returns the underlying social store (for direct queries by handlers).
func (m *Manager) Store() store.SocialStore {
	return m.store
}

// PublishPost publishes a post to all its targets.
func (m *Manager) PublishPost(ctx context.Context, postID uuid.UUID) error {
	post, err := m.store.GetPost(ctx, postID)
	if err != nil {
		return fmt.Errorf("get post: %w", err)
	}

	if len(post.Targets) == 0 {
		return fmt.Errorf("post has no targets")
	}

	// Mark post as publishing
	_ = m.store.UpdatePost(ctx, postID, map[string]any{"status": store.SocialPostStatusPublishing})

	// Mark all targets as publishing
	for _, target := range post.Targets {
		_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{"status": store.SocialTargetStatusPublishing})
	}

	// Publish to each target
	published, failed := 0, 0
	for _, target := range post.Targets {
		m.publishSingleTarget(ctx, post, target)
		// Re-read target status to check result
		targets, _ := m.store.ListTargets(ctx, postID)
		for _, t := range targets {
			if t.ID == target.ID {
				if t.Status == store.SocialTargetStatusPublished {
					published++
				} else {
					failed++
				}
			}
		}
	}

	// Determine final post status
	status := store.SocialPostStatusPublished
	now := time.Now().UTC()
	updates := map[string]any{"published_at": now}
	if failed > 0 && published > 0 {
		status = store.SocialPostStatusPartial
	} else if failed > 0 && published == 0 {
		status = store.SocialPostStatusFailed
	}
	updates["status"] = status

	return m.store.UpdatePost(ctx, postID, updates)
}

// PublishTarget publishes a single target entry.
func (m *Manager) PublishTarget(ctx context.Context, targetID uuid.UUID) (*PublishResult, error) {
	// Get target with post info
	// We need to find the target's account and post content
	// Target doesn't have a direct getter, so we'll work with what we have

	// Mark target as publishing
	_ = m.store.UpdateTarget(ctx, targetID, map[string]any{"status": store.SocialTargetStatusPublishing})

	// This method is called from PublishPost which already has the post loaded.
	// For standalone calls, we'd need to look up the target's post.
	// For now, return an error — callers should use PublishPost instead.
	return nil, fmt.Errorf("use PublishPost for full publish flow")
}

// publishSingleTarget handles the actual publish for one target.
func (m *Manager) publishSingleTarget(ctx context.Context, post *store.SocialPostData, target store.SocialPostTargetData) {
	// Get account
	account, err := m.store.GetAccount(ctx, target.AccountID)
	if err != nil {
		errMsg := err.Error()
		_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{
			"status": store.SocialTargetStatusFailed,
			"error":  errMsg,
		})
		return
	}

	// For Facebook/Instagram: override metadata with default page from social_pages table.
	metadata := account.Metadata
	if account.Platform == "facebook" || account.Platform == "instagram" {
		metadata = m.applyDefaultPage(ctx, account)
	}

	// Create platform client — use browser automation for TikTok when no API token.
	var client PlatformClient
	if account.Platform == "tiktok" && account.AccessToken == "" {
		if m.browserMgr == nil {
			errMsg := "no API token and no browser manager available for tiktok"
			_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{
				"status": store.SocialTargetStatusFailed,
				"error":  errMsg,
			})
			return
		}
		client = newTikTokBrowserClient(m.browserMgr, m.cookieSrc, account.Metadata)
	} else {
		var clientErr error
		client, clientErr = NewClient(account.Platform, account.AccessToken, metadata)
		if clientErr != nil {
			errMsg := clientErr.Error()
			_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{
				"status": store.SocialTargetStatusFailed,
				"error":  errMsg,
			})
			return
		}
	}

	// Adapt content for platform
	adapted, _ := AdaptContent(post.Content, account.Platform)

	// Build media items
	var media []MediaItem
	for _, pm := range post.Media {
		media = append(media, MediaItem{
			URL:       pm.URL,
			MediaType: pm.MediaType,
			MimeType:  derefStrPtr(pm.MimeType),
			Filename:  derefStrPtr(pm.Filename),
		})
	}

	// Publish
	result, err := client.Publish(ctx, PublishRequest{
		Content:  adapted,
		Media:    media,
		PostType: post.PostType,
		Metadata: nil,
	})

	if err != nil {
		errMsg := err.Error()
		_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{
			"status": store.SocialTargetStatusFailed,
			"error":  errMsg,
		})
		return
	}

	// Update target with success
	now := time.Now().UTC()
	_ = m.store.UpdateTarget(ctx, target.ID, map[string]any{
		"status":           store.SocialTargetStatusPublished,
		"platform_post_id": result.PlatformPostID,
		"platform_url":     result.PlatformURL,
		"adapted_content":  adapted,
		"published_at":     now,
	})

	// Save adapted content
	m.logger.Info("published to platform",
		"platform", account.Platform,
		"post_id", target.PostID,
		"platform_post_id", result.PlatformPostID,
	)
}

// applyDefaultPage checks social_pages for a default page and merges its
// token/ID into account metadata. Falls back to original metadata if no page found.
func (m *Manager) applyDefaultPage(ctx context.Context, account *store.SocialAccountData) json.RawMessage {
	page, err := m.store.GetDefaultPage(ctx, account.ID)
	if err != nil || page == nil {
		return account.Metadata // fallback to legacy metadata
	}

	// Merge page data into a copy of the metadata.
	var meta map[string]any
	if account.Metadata != nil {
		_ = json.Unmarshal(account.Metadata, &meta)
	}
	if meta == nil {
		meta = map[string]any{}
	}

	switch account.Platform {
	case "facebook":
		meta["page_id"] = page.PageID
		meta["page_token"] = page.PageToken
		if page.PageName != nil {
			meta["page_name"] = *page.PageName
		}
	case "instagram":
		if page.PageToken != "" {
			meta["page_token"] = page.PageToken
		}
		// ig_user_id may come from page metadata
		var pageMeta map[string]string
		if page.Metadata != nil {
			_ = json.Unmarshal(page.Metadata, &pageMeta)
		}
		if igID, ok := pageMeta["ig_user_id"]; ok {
			meta["ig_user_id"] = igID
		}
	}

	b, _ := json.Marshal(meta)
	return b
}

// ProcessDuePosts finds and publishes all scheduled posts that are due.
func (m *Manager) ProcessDuePosts(ctx context.Context) error {
	posts, err := m.store.ListDuePosts(ctx)
	if err != nil {
		return fmt.Errorf("list due posts: %w", err)
	}
	if len(posts) == 0 {
		return nil
	}

	m.logger.Info("processing due posts", "count", len(posts))
	for _, post := range posts {
		if err := m.PublishPost(ctx, post.ID); err != nil {
			m.logger.Warn("failed to publish due post", "post_id", post.ID, "error", err)
		}
	}
	return nil
}

// RefreshExpiringTokens proactively refreshes tokens that expire within 24h.
func (m *Manager) RefreshExpiringTokens(ctx context.Context) error {
	// Query accounts with tokens expiring in the next 24 hours
	// This is a simplified approach — a proper implementation would add
	// a store method for this query
	m.logger.Info("checking for expiring tokens")
	return nil
}

func derefStrPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
