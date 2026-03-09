package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Content schedule source constants.
const (
	ContentSourceAgent  = "agent"
	ContentSourcePrompt = "prompt"
)

// Content schedule status constants.
const (
	ContentScheduleStatusSuccess = "success"
	ContentScheduleStatusFailed  = "failed"
	ContentScheduleStatusPartial = "partial"
)

// ContentScheduleData represents a recurring social posting rule.
type ContentScheduleData struct {
	ID             uuid.UUID  `json:"id"`
	OwnerID        string     `json:"owner_id"`
	Name           string     `json:"name"`
	Enabled        bool       `json:"enabled"`
	ContentSource  string     `json:"content_source"`
	AgentID        *uuid.UUID `json:"agent_id,omitempty"`
	Prompt         *string    `json:"prompt,omitempty"`
	CronExpression string     `json:"cron_expression"`
	Timezone       string     `json:"timezone"`
	CronJobID      *string    `json:"cron_job_id,omitempty"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	LastStatus     *string    `json:"last_status,omitempty"`
	LastError      *string    `json:"last_error,omitempty"`
	PostsCount     int        `json:"posts_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
	// Joined
	Pages []ContentSchedulePageData `json:"pages,omitempty"`
}

// ContentSchedulePageData represents a social page linked to a schedule.
type ContentSchedulePageData struct {
	ID         uuid.UUID `json:"id"`
	ScheduleID uuid.UUID `json:"schedule_id"`
	PageID     uuid.UUID `json:"page_id"`
	CreatedAt  time.Time `json:"created_at"`
	// Joined from social_pages / social_accounts
	PageName  *string   `json:"page_name,omitempty"`
	PageType  string    `json:"page_type,omitempty"`
	Platform  string    `json:"platform,omitempty"`
	AccountID uuid.UUID `json:"account_id,omitempty"`
}

// ContentScheduleLogData represents a single execution log entry for a schedule.
type ContentScheduleLogData struct {
	ID             uuid.UUID  `json:"id"`
	ScheduleID     uuid.UUID  `json:"schedule_id"`
	PostID         *uuid.UUID `json:"post_id,omitempty"`
	Status         string     `json:"status"`
	Error          *string    `json:"error,omitempty"`
	ContentPreview *string    `json:"content_preview,omitempty"`
	PagesTargeted  int        `json:"pages_targeted"`
	PagesPublished int        `json:"pages_published"`
	DurationMS     *int64     `json:"duration_ms,omitempty"`
	RanAt          time.Time  `json:"ran_at"`
}

// ContentScheduleStore manages content schedule records and their logs.
type ContentScheduleStore interface {
	Create(ctx context.Context, s *ContentScheduleData) error
	Get(ctx context.Context, id uuid.UUID) (*ContentScheduleData, error)
	Update(ctx context.Context, id uuid.UUID, updates map[string]any) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, ownerID string, enabled *bool) ([]ContentScheduleData, error)
	GetByJobID(ctx context.Context, cronJobID string) (*ContentScheduleData, error)
	SetPages(ctx context.Context, scheduleID uuid.UUID, pageIDs []uuid.UUID) error
	ListPages(ctx context.Context, scheduleID uuid.UUID) ([]ContentSchedulePageData, error)
	AddLog(ctx context.Context, log *ContentScheduleLogData) error
	ListLogs(ctx context.Context, scheduleID uuid.UUID, limit, offset int) ([]ContentScheduleLogData, int, error)
}
