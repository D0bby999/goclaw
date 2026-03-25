package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/scheduler"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// schedulePrefix is the internal message prefix used to trigger content schedule runs via cron.
const schedulePrefix = "{internal:content_schedule:"

// makeCronJobHandler creates a cron job handler that routes through the scheduler's cron lane.
// This ensures per-session concurrency control (same job can't run concurrently)
// and integration with /stop, /stopall commands.
// cronHeartbeatWakeFn holds the heartbeat wake function, set after ticker creation.
// Safe because cron jobs only fire after Start(), well after this is set.
var cronHeartbeatWakeFn func(agentID string)

// scheduleHandler is optional; if nil, content schedule jobs are skipped.
func makeCronJobHandler(sched *scheduler.Scheduler, msgBus *bus.MessageBus, cfg *config.Config, channelMgr *channels.Manager, sessionMgr store.SessionStore, scheduleHandler *social.ScheduleHandler) func(job *store.CronJob) (*store.CronJobResult, error) {
	return func(job *store.CronJob) (*store.CronJobResult, error) {
		// Content schedule detection — intercept before normal agent-turn flow.
		if strings.HasPrefix(job.Payload.Message, schedulePrefix) {
			idStr := strings.TrimPrefix(job.Payload.Message, schedulePrefix)
			idStr = strings.TrimSuffix(idStr, "}")
			scheduleID, parseErr := uuid.Parse(idStr)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid schedule ID in cron message: %w", parseErr)
			}
			if scheduleHandler == nil {
				return &store.CronJobResult{Content: "skipped: no schedule handler"}, nil
			}
			return scheduleHandler.Handle(context.Background(), scheduleID)
		}

		agentID := job.AgentID
		if agentID == "" {
			agentID = cfg.ResolveDefaultAgentID()
		} else {
			agentID = config.NormalizeAgentID(agentID)
		}

		sessionKey := sessions.BuildCronSessionKey(agentID, job.ID)
		channel := job.Payload.Channel
		if channel == "" {
			channel = "cron"
		}

		// Infer peer kind from the stored session metadata (group chats need it
		// so that tools like message can route correctly via group APIs).
		peerKind := resolveCronPeerKind(job)

		// Resolve channel type for system prompt context.
		channelType := resolveChannelType(channelMgr, channel)

		// Build cron context so the agent knows delivery target and requester.
		var extraPrompt string
		if job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
			extraPrompt = fmt.Sprintf(
				"[Cron Job]\nThis is scheduled job \"%s\" (ID: %s).\n"+
					"Requester: user %s on channel \"%s\" (chat %s).\n"+
					"Your response will be automatically delivered to that chat — just produce the content directly.",
				job.Name, job.ID, job.UserID, job.Payload.Channel, job.Payload.To,
			)
		} else {
			extraPrompt = fmt.Sprintf(
				"[Cron Job]\nThis is scheduled job \"%s\" (ID: %s), created by user %s.\n"+
					"Delivery is not configured — respond normally.",
				job.Name, job.ID, job.UserID,
			)
		}

		// Build context with tenant scope so agent loop events are scoped correctly.
		cronCtx := store.WithTenantID(context.Background(), job.TenantID)

		// Reset session before each cron run to prevent tool errors from previous
		// runs from polluting the context and blocking future executions (#294).
		// Save() persists the empty session to DB so stale data won't reload after restart.
		sessionMgr.Reset(cronCtx, sessionKey)
		sessionMgr.Save(cronCtx, sessionKey)

		// Schedule through cron lane — scheduler handles agent resolution and concurrency
		outCh := sched.Schedule(cronCtx, scheduler.LaneCron, agent.RunRequest{
			SessionKey:        sessionKey,
			Message:           job.Payload.Message,
			Channel:           channel,
			ChannelType:       channelType,
			ChatID:            job.Payload.To,
			PeerKind:          peerKind,
			UserID:            job.UserID,
			RunID:             fmt.Sprintf("cron:%s", job.ID),
			Stream:            false,
			ExtraSystemPrompt: extraPrompt,
			TraceName:         fmt.Sprintf("Cron [%s] - %s", job.Name, agentID),
			TraceTags:         []string{"cron"},
		})

		// Block until the scheduled run completes
		outcome := <-outCh
		if outcome.Err != nil {
			return nil, outcome.Err
		}

		result := outcome.Result

		// If job wants delivery, send the agent's response to each recipient.
		// Channel and To may be comma-separated for multi-recipient delivery.
		if job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
			chList := strings.Split(job.Payload.Channel, ",")
			toList := strings.Split(job.Payload.To, ",")
			for i, to := range toList {
				to = strings.TrimSpace(to)
				if to == "" {
					continue
				}
				ch := strings.TrimSpace(chList[0])
				if i < len(chList) {
					ch = strings.TrimSpace(chList[i])
				}
				outMsg := bus.OutboundMessage{
					Channel: ch,
					ChatID:  to,
					Content: result.Content,
				}
				if strings.HasPrefix(to, "-") {
					outMsg.Metadata = map[string]string{"group_id": to}
				}
				msgBus.PublishOutbound(outMsg)
			}
		} else if job.Payload.Deliver {
			slog.Warn("cron: delivery configured but channel/chatID missing — output discarded",
				"job_id", job.ID, "job_name", job.Name, "channel", job.Payload.Channel, "to", job.Payload.To)
		}

		cronResult := &store.CronJobResult{
			Content: result.Content,
		}
		if result.Usage != nil {
			cronResult.InputTokens = result.Usage.PromptTokens
			cronResult.OutputTokens = result.Usage.CompletionTokens
		}

		// wakeMode: trigger heartbeat after cron job completes
		if job.Payload.WakeHeartbeat && cronHeartbeatWakeFn != nil {
			cronHeartbeatWakeFn(agentID)
		}

		return cronResult, nil
	}
}

// resolveCronPeerKind infers peer kind from the cron job's user ID.
// Group cron jobs have userID prefixed with "group:" or "guild:" (set during job creation).
func resolveCronPeerKind(job *store.CronJob) string {
	if strings.HasPrefix(job.UserID, "group:") || strings.HasPrefix(job.UserID, "guild:") {
		return "group"
	}
	return ""
}
