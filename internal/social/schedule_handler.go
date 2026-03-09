package social

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ScheduleContentGenerator generates content for a schedule (fulfilled by the agent runner).
type ScheduleContentGenerator interface {
	GenerateContent(ctx context.Context, agentID, ownerID, prompt string) (string, error)
}

// ScheduleHandler processes content schedule cron triggers.
type ScheduleHandler struct {
	scheduleStore store.ContentScheduleStore
	socialStore   store.SocialStore
	contentGen    ScheduleContentGenerator
	logger        *slog.Logger
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(
	scheduleStore store.ContentScheduleStore,
	socialStore store.SocialStore,
	contentGen ScheduleContentGenerator,
) *ScheduleHandler {
	return &ScheduleHandler{
		scheduleStore: scheduleStore,
		socialStore:   socialStore,
		contentGen:    contentGen,
		logger:        slog.Default().With("component", "schedule_handler"),
	}
}

// Handle processes a content schedule run triggered by a cron job.
func (h *ScheduleHandler) Handle(ctx context.Context, scheduleID uuid.UUID) (*store.CronJobResult, error) {
	start := time.Now()

	// 1. Load schedule with pages
	schedule, err := h.scheduleStore.Get(ctx, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("schedule not found: %w", err)
	}
	if !schedule.Enabled || len(schedule.Pages) == 0 {
		reason := "disabled"
		if schedule.Enabled {
			reason = "no pages"
		}
		return &store.CronJobResult{Content: "skipped: " + reason}, nil
	}

	// 2. Generate content
	content, err := h.generateContent(ctx, schedule)
	if err != nil {
		dur := time.Since(start)
		h.logRun(ctx, schedule.ID, nil, store.ContentScheduleStatusFailed, err.Error(), len(schedule.Pages), 0, dur)
		h.updateStats(ctx, schedule.ID, store.ContentScheduleStatusFailed, err.Error(), schedule.PostsCount)
		return nil, fmt.Errorf("content generation failed: %w", err)
	}

	// 3. Create social post with "publishing" status
	post := &store.SocialPostData{
		OwnerID:  schedule.OwnerID,
		Content:  content,
		PostType: "text",
		Status:   store.SocialPostStatusPublishing,
	}
	if err := h.socialStore.CreatePost(ctx, post); err != nil {
		dur := time.Since(start)
		h.logRun(ctx, schedule.ID, nil, store.ContentScheduleStatusFailed, err.Error(), len(schedule.Pages), 0, dur)
		h.updateStats(ctx, schedule.ID, store.ContentScheduleStatusFailed, err.Error(), schedule.PostsCount)
		return nil, fmt.Errorf("create post: %w", err)
	}

	// 4. Add targets for each page's account
	for _, page := range schedule.Pages {
		target := &store.SocialPostTargetData{
			PostID:    post.ID,
			AccountID: page.AccountID,
			Status:    store.SocialTargetStatusPending,
		}
		if err := h.socialStore.AddTarget(ctx, target); err != nil {
			h.logger.Warn("schedule_handler: failed to add target",
				"schedule_id", scheduleID,
				"post_id", post.ID,
				"account_id", page.AccountID,
				"error", err,
			)
		}
	}

	// 5. Publish the post
	if err := h.publishPost(ctx, post.ID); err != nil {
		dur := time.Since(start)
		h.logRun(ctx, schedule.ID, &post.ID, store.ContentScheduleStatusFailed, err.Error(), len(schedule.Pages), 0, dur)
		h.updateStats(ctx, schedule.ID, store.ContentScheduleStatusFailed, err.Error(), schedule.PostsCount)
		return nil, fmt.Errorf("publish post: %w", err)
	}

	// 6. Count results from targets
	targets, _ := h.socialStore.ListTargets(ctx, post.ID)
	published := 0
	for _, t := range targets {
		if t.Status == store.SocialTargetStatusPublished {
			published++
		}
	}

	// 7. Determine final status and log
	status := store.ContentScheduleStatusSuccess
	if published == 0 {
		status = store.ContentScheduleStatusFailed
	} else if published < len(schedule.Pages) {
		status = store.ContentScheduleStatusPartial
	}

	dur := time.Since(start)
	h.logRun(ctx, schedule.ID, &post.ID, status, "", len(schedule.Pages), published, dur)
	h.updateStats(ctx, schedule.ID, status, "", schedule.PostsCount)

	h.logger.Info("schedule_handler: run complete",
		"schedule_id", scheduleID,
		"post_id", post.ID,
		"targeted", len(schedule.Pages),
		"published", published,
		"status", status,
		"duration_ms", dur.Milliseconds(),
	)

	return &store.CronJobResult{Content: content}, nil
}

func (h *ScheduleHandler) generateContent(ctx context.Context, s *store.ContentScheduleData) (string, error) {
	prompt := "Generate a social media post."
	if s.Prompt != nil && *s.Prompt != "" {
		prompt = *s.Prompt
	}

	agentID := ""
	if s.AgentID != nil {
		agentID = s.AgentID.String()
	}

	return h.contentGen.GenerateContent(ctx, agentID, s.OwnerID, prompt)
}

// publishPost calls the social store to update post status without importing social.Manager
// (to avoid circular dependency). It delegates to the socialStore directly.
func (h *ScheduleHandler) publishPost(ctx context.Context, postID uuid.UUID) error {
	// Mark as publishing
	if err := h.socialStore.UpdatePost(ctx, postID, map[string]any{"status": store.SocialPostStatusPublishing}); err != nil {
		return err
	}

	// Mark all targets publishing
	targets, err := h.socialStore.ListTargets(ctx, postID)
	if err != nil {
		return err
	}
	for _, t := range targets {
		_ = h.socialStore.UpdateTarget(ctx, t.ID, map[string]any{"status": store.SocialTargetStatusPublishing})
	}
	return nil
}

func (h *ScheduleHandler) logRun(ctx context.Context, scheduleID uuid.UUID, postID *uuid.UUID, status, errMsg string, targeted, published int, duration time.Duration) {
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	durationMs := duration.Milliseconds()
	_ = h.scheduleStore.AddLog(ctx, &store.ContentScheduleLogData{
		ScheduleID:     scheduleID,
		PostID:         postID,
		Status:         status,
		Error:          errPtr,
		PagesTargeted:  targeted,
		PagesPublished: published,
		DurationMS:     &durationMs,
	})
}

func (h *ScheduleHandler) updateStats(ctx context.Context, scheduleID uuid.UUID, status, errMsg string, currentCount int) {
	updates := map[string]any{
		"last_run_at": time.Now().UTC(),
		"last_status": status,
	}
	if errMsg != "" {
		updates["last_error"] = errMsg
	}
	if status == store.ContentScheduleStatusSuccess {
		updates["posts_count"] = currentCount + 1
	}
	_ = h.scheduleStore.Update(ctx, scheduleID, updates)
}
