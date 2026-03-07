package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Social platform constants.
const (
	PlatformFacebook  = "facebook"
	PlatformInstagram = "instagram"
	PlatformTwitter   = "twitter"
	PlatformYouTube   = "youtube"
	PlatformTikTok    = "tiktok"
	PlatformThreads   = "threads"
	PlatformLinkedIn  = "linkedin"
	PlatformBluesky   = "bluesky"
)

// Social account status constants.
const (
	SocialAccountStatusActive  = "active"
	SocialAccountStatusExpired = "expired"
	SocialAccountStatusRevoked = "revoked"
)

// Social post status constants.
const (
	SocialPostStatusDraft      = "draft"
	SocialPostStatusScheduled  = "scheduled"
	SocialPostStatusPublishing = "publishing"
	SocialPostStatusPublished  = "published"
	SocialPostStatusPartial    = "partial"
	SocialPostStatusFailed     = "failed"
)

// Social target status constants.
const (
	SocialTargetStatusPending    = "pending"
	SocialTargetStatusPublishing = "publishing"
	SocialTargetStatusPublished  = "published"
	SocialTargetStatusFailed     = "failed"
)

// SocialAccountData represents a connected social platform account.
type SocialAccountData struct {
	BaseModel
	OwnerID          string          `json:"owner_id"`
	Platform         string          `json:"platform"`
	PlatformUserID   string          `json:"platform_user_id"`
	PlatformUsername *string         `json:"platform_username,omitempty"`
	DisplayName      *string         `json:"display_name,omitempty"`
	AvatarURL        *string         `json:"avatar_url,omitempty"`
	AccessToken      string          `json:"-"`
	RefreshToken     *string         `json:"-"`
	TokenExpiresAt   *time.Time      `json:"token_expires_at,omitempty"`
	Scopes           []string        `json:"scopes,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	Status           string          `json:"status"`
	ConnectedAt      time.Time       `json:"connected_at"`
	DeletedAt        *time.Time      `json:"deleted_at,omitempty"`
}

// SocialPostData represents a social media post.
type SocialPostData struct {
	BaseModel
	OwnerID      string          `json:"owner_id"`
	Title        *string         `json:"title,omitempty"`
	Content      string          `json:"content"`
	PostType     string          `json:"post_type"`
	Status       string          `json:"status"`
	ScheduledAt  *time.Time      `json:"scheduled_at,omitempty"`
	PublishedAt  *time.Time      `json:"published_at,omitempty"`
	PostGroupID  *uuid.UUID      `json:"post_group_id,omitempty"`
	ParentPostID *uuid.UUID      `json:"parent_post_id,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Error        *string         `json:"error,omitempty"`
	DeletedAt    *time.Time      `json:"deleted_at,omitempty"`
	// Joined
	Targets []SocialPostTargetData `json:"targets,omitempty"`
	Media   []SocialPostMediaData  `json:"media,omitempty"`
}

// SocialPostTargetData represents a per-account publish target for a post.
type SocialPostTargetData struct {
	ID             uuid.UUID  `json:"id"`
	PostID         uuid.UUID  `json:"post_id"`
	AccountID      uuid.UUID  `json:"account_id"`
	PlatformPostID *string    `json:"platform_post_id,omitempty"`
	PlatformURL    *string    `json:"platform_url,omitempty"`
	AdaptedContent *string    `json:"adapted_content,omitempty"`
	Status         string     `json:"status"`
	Error          *string    `json:"error,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	// Joined
	Platform         string  `json:"platform,omitempty"`
	PlatformUsername *string `json:"platform_username,omitempty"`
}

// SocialPostMediaData represents a media attachment for a post.
type SocialPostMediaData struct {
	ID              uuid.UUID       `json:"id"`
	PostID          uuid.UUID       `json:"post_id"`
	MediaType       string          `json:"media_type"`
	URL             string          `json:"url"`
	ThumbnailURL    *string         `json:"thumbnail_url,omitempty"`
	Filename        *string         `json:"filename,omitempty"`
	MimeType        *string         `json:"mime_type,omitempty"`
	FileSize        *int64          `json:"file_size,omitempty"`
	Width           *int            `json:"width,omitempty"`
	Height          *int            `json:"height,omitempty"`
	DurationSeconds *int            `json:"duration_seconds,omitempty"`
	SortOrder       int             `json:"sort_order"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// SocialOAuthStateData represents a temporary OAuth state for CSRF protection.
type SocialOAuthStateData struct {
	ID          uuid.UUID       `json:"id"`
	Platform    string          `json:"platform"`
	State       string          `json:"state"`
	OwnerID     string          `json:"owner_id"`
	RedirectURL *string         `json:"redirect_url,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	ExpiresAt   time.Time       `json:"expires_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SocialStore manages social accounts, posts, targets, and media.
type SocialStore interface {
	// Accounts
	CreateAccount(ctx context.Context, a *SocialAccountData) error
	GetAccount(ctx context.Context, id uuid.UUID) (*SocialAccountData, error)
	UpdateAccount(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeleteAccount(ctx context.Context, id uuid.UUID) error
	ListAccounts(ctx context.Context, ownerID string) ([]SocialAccountData, error)
	ListAccountsByPlatform(ctx context.Context, ownerID, platform string) ([]SocialAccountData, error)

	// Posts
	CreatePost(ctx context.Context, p *SocialPostData) error
	GetPost(ctx context.Context, id uuid.UUID) (*SocialPostData, error)
	UpdatePost(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeletePost(ctx context.Context, id uuid.UUID) error
	ListPosts(ctx context.Context, ownerID string, status string, limit, offset int) ([]SocialPostData, int, error)
	ListDuePosts(ctx context.Context) ([]SocialPostData, error)

	// Targets
	AddTarget(ctx context.Context, t *SocialPostTargetData) error
	UpdateTarget(ctx context.Context, id uuid.UUID, updates map[string]any) error
	ListTargets(ctx context.Context, postID uuid.UUID) ([]SocialPostTargetData, error)
	RemoveTarget(ctx context.Context, id uuid.UUID) error

	// Media
	AddMedia(ctx context.Context, m *SocialPostMediaData) error
	ListMedia(ctx context.Context, postID uuid.UUID) ([]SocialPostMediaData, error)
	RemoveMedia(ctx context.Context, id uuid.UUID) error
	ReorderMedia(ctx context.Context, postID uuid.UUID, mediaIDs []uuid.UUID) error

	// OAuth
	CreateOAuthState(ctx context.Context, s *SocialOAuthStateData) error
	GetOAuthState(ctx context.Context, state string) (*SocialOAuthStateData, error)
	DeleteOAuthState(ctx context.Context, state string) error
	CleanExpiredOAuthStates(ctx context.Context) error
}
