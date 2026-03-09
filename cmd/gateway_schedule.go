package cmd

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
)

// scheduleContentGen adapts the scheduler to implement social.ScheduleContentGenerator.
// It routes content generation requests through the cron lane so they benefit from
// per-session concurrency control and /stop integration.
type scheduleContentGen struct {
	sched *scheduler.Scheduler
	cfg   *config.Config
}

// GenerateContent runs an agent turn to produce social media post content.
// agentID may be empty — falls back to the configured default agent.
func (g *scheduleContentGen) GenerateContent(ctx context.Context, agentID, ownerID, prompt string) (string, error) {
	if agentID == "" {
		agentID = g.cfg.ResolveDefaultAgentID()
	} else {
		agentID = config.NormalizeAgentID(agentID)
	}

	sessionKey := fmt.Sprintf("agent:%s:schedule-gen:%s", agentID, ownerID)

	outCh := g.sched.Schedule(ctx, scheduler.LaneCron, agent.RunRequest{
		SessionKey: sessionKey,
		Message:    prompt,
		Channel:    "social",
		UserID:     ownerID,
		RunID:      fmt.Sprintf("schedule-gen:%s", uuid.New().String()[:8]),
		Stream:     false,
		TraceName:  "ScheduleContentGen",
		TraceTags:  []string{"schedule", "social"},
	})

	outcome := <-outCh
	if outcome.Err != nil {
		return "", outcome.Err
	}
	if outcome.Result == nil {
		return "", fmt.Errorf("schedule content gen: nil result")
	}
	return outcome.Result.Content, nil
}
