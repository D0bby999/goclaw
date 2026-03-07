package social

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PlatformClient defines the interface each social platform must implement.
type PlatformClient interface {
	Platform() string
	Publish(ctx context.Context, req PublishRequest) (*PublishResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResult, error)
	GetProfile(ctx context.Context) (*ProfileResult, error)
}

// PublishRequest contains everything needed to publish a post.
type PublishRequest struct {
	Content  string         `json:"content"`
	Media    []MediaItem    `json:"media,omitempty"`
	PostType string         `json:"post_type"` // post, reel, story, thread
	Metadata map[string]any `json:"metadata,omitempty"`
	ReplyTo  string         `json:"reply_to,omitempty"` // for threading
}

// PublishResult is returned after a successful publish.
type PublishResult struct {
	PlatformPostID string `json:"platform_post_id"`
	PlatformURL    string `json:"platform_url"`
}

// TokenResult is returned after a token refresh.
type TokenResult struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// ProfileResult contains basic profile info from a platform.
type ProfileResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
}

// MediaItem represents a media attachment for publishing.
type MediaItem struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type"` // image, video, gif
	MimeType  string `json:"mime_type"`
	Filename  string `json:"filename"`
}

// ContentLimits defines per-platform content constraints.
type ContentLimits struct {
	MaxChars    int `json:"max_chars"`
	MaxHashtags int `json:"max_hashtags"`
	LinkLength  int `json:"link_length,omitempty"` // twitter t.co
}

// PlatformLimits maps platform names to their content limits.
var PlatformLimits = map[string]ContentLimits{
	"twitter":   {MaxChars: 280, MaxHashtags: 30, LinkLength: 23},
	"instagram": {MaxChars: 2200, MaxHashtags: 30},
	"tiktok":    {MaxChars: 2200, MaxHashtags: 30},
	"linkedin":  {MaxChars: 3000, MaxHashtags: 30},
	"threads":   {MaxChars: 500, MaxHashtags: 30},
	"facebook":  {MaxChars: 63206, MaxHashtags: 30},
	"bluesky":   {MaxChars: 300, MaxHashtags: 0},
	"youtube":   {MaxChars: 5000, MaxHashtags: 15},
}

// AdaptContent truncates/adjusts content for a specific platform.
func AdaptContent(content, platform string) (adapted string, warnings []string) {
	limits, ok := PlatformLimits[platform]
	if !ok {
		return content, nil
	}

	adapted = content

	// Count and limit hashtags
	if limits.MaxHashtags > 0 {
		words := strings.Fields(adapted)
		hashtagCount := 0
		var filtered []string
		for _, w := range words {
			if strings.HasPrefix(w, "#") {
				hashtagCount++
				if hashtagCount > limits.MaxHashtags {
					warnings = append(warnings, fmt.Sprintf("removed hashtag %s (over %d limit)", w, limits.MaxHashtags))
					continue
				}
			}
			filtered = append(filtered, w)
		}
		if hashtagCount > limits.MaxHashtags {
			adapted = strings.Join(filtered, " ")
		}
	}

	// Truncate content
	if len([]rune(adapted)) > limits.MaxChars {
		runes := []rune(adapted)
		adapted = string(runes[:limits.MaxChars-3]) + "..."
		warnings = append(warnings, fmt.Sprintf("content truncated to %d chars", limits.MaxChars))
	}

	return adapted, warnings
}

// OAuthConfig holds platform OAuth credentials from config.
type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

// CookieSource retrieves stored cookies for a platform (used for browser automation).
type CookieSource interface {
	GetDefault(ctx context.Context, platform string) (string, error)
}

// NewClient creates a platform client with the given token and metadata.
func NewClient(platform, accessToken string, metadata json.RawMessage) (PlatformClient, error) {
	switch platform {
	case "facebook":
		return newFacebookClient(accessToken, metadata), nil
	case "instagram":
		return newInstagramClient(accessToken, metadata), nil
	case "twitter":
		return newTwitterClient(accessToken, metadata), nil
	case "youtube":
		return newYouTubeClient(accessToken, metadata), nil
	case "tiktok":
		return newTikTokClient(accessToken, metadata), nil
	case "threads":
		return newThreadsClient(accessToken, metadata), nil
	case "linkedin":
		return newLinkedInClient(accessToken, metadata), nil
	case "bluesky":
		return newBlueskyClient(accessToken, metadata), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// ErrorCategory classifies platform API errors.
type ErrorCategory string

const (
	ErrCategoryRateLimit ErrorCategory = "rate_limit"
	ErrCategoryAuth      ErrorCategory = "auth"
	ErrCategoryPlatform  ErrorCategory = "platform"
	ErrCategoryClient    ErrorCategory = "client"
	ErrCategoryNetwork   ErrorCategory = "network"
)

// PlatformError wraps platform-specific errors with category.
type PlatformError struct {
	Platform string        `json:"platform"`
	Category ErrorCategory `json:"category"`
	Code     int           `json:"code,omitempty"`
	Message  string        `json:"message"`
}

func (e *PlatformError) Error() string {
	return fmt.Sprintf("%s [%s]: %s", e.Platform, e.Category, e.Message)
}
