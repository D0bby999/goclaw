package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// AnalyticsTool queries gateway usage analytics and generates reports.
type AnalyticsTool struct {
	analyticsStore store.AnalyticsStore
}

func NewAnalyticsTool(as store.AnalyticsStore) *AnalyticsTool {
	return &AnalyticsTool{analyticsStore: as}
}

func (t *AnalyticsTool) Name() string { return "analytics" }

func (t *AnalyticsTool) Description() string {
	return `Query gateway usage analytics and generate reports.

ACTIONS:
- summary: Quick usage stats for a time period (default: today)
- report: Full breakdown with top agents, models, and tools
- query: Custom analytics query with filters

TIME PERIODS (period param):
- "today": current day (midnight to now)
- "yesterday": previous day
- "7d": last 7 days
- "30d": last 30 days
- "custom": use from/to params (ISO 8601)

Use for daily digests, weekly summaries, cost estimation, and performance monitoring.`
}

func (t *AnalyticsTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"summary", "report", "query"},
				"description": "Analytics action to perform",
			},
			"period": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"today", "yesterday", "7d", "30d", "custom"},
				"description": "Time period for analytics. Default: 'today'",
			},
			"from": map[string]interface{}{
				"type":        "string",
				"description": "[custom period] Start date (ISO 8601)",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "[custom period] End date (ISO 8601)",
			},
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "Filter by specific agent UUID. Default: current agent",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "[report/query] Max items in top-N lists. Default: 10",
			},
		},
		"required": []string{"action"},
	}
}

func (t *AnalyticsTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	action, _ := args["action"].(string)
	switch action {
	case "summary":
		return t.execSummary(ctx, args)
	case "report", "query":
		return t.execReport(ctx, args)
	default:
		return ErrorResult("action must be one of: summary, report, query")
	}
}

func (t *AnalyticsTool) execSummary(ctx context.Context, args map[string]interface{}) *Result {
	from, to, label, err := resolveAnalyticsPeriod(args)
	if err != nil {
		return ErrorResult(err.Error())
	}

	filter := t.buildFilter(ctx, args, from, to)
	summary, err := t.analyticsStore.Summary(ctx, filter)
	if err != nil {
		return ErrorResult(fmt.Sprintf("analytics query failed: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Usage Summary (%s)\n\n", label))
	sb.WriteString(fmt.Sprintf("- Requests: %d total, %d completed, %d errors\n",
		summary.TotalTraces, summary.CompletedTraces, summary.ErrorTraces))
	sb.WriteString(fmt.Sprintf("- Tokens: %d input, %d output\n",
		summary.TotalInputTokens, summary.TotalOutputTokens))
	sb.WriteString(fmt.Sprintf("- LLM calls: %d, Tool calls: %d\n",
		summary.TotalLLMCalls, summary.TotalToolCalls))
	sb.WriteString(fmt.Sprintf("- Active sessions: %d, Unique users: %d\n",
		summary.ActiveSessions, summary.UniqueUsers))

	return NewResult(sb.String())
}

func (t *AnalyticsTool) execReport(ctx context.Context, args map[string]interface{}) *Result {
	from, to, label, err := resolveAnalyticsPeriod(args)
	if err != nil {
		return ErrorResult(err.Error())
	}

	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	filter := t.buildFilter(ctx, args, from, to)

	summary, err := t.analyticsStore.Summary(ctx, filter)
	if err != nil {
		return ErrorResult(fmt.Sprintf("analytics query failed: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Analytics Report (%s)\n\n", label))

	sb.WriteString("### Overview\n")
	sb.WriteString(fmt.Sprintf("- Requests: %d (%d errors)\n", summary.TotalTraces, summary.ErrorTraces))
	sb.WriteString(fmt.Sprintf("- Tokens: %d input + %d output\n",
		summary.TotalInputTokens, summary.TotalOutputTokens))
	sb.WriteString(fmt.Sprintf("- Active sessions: %d | Unique users: %d\n\n",
		summary.ActiveSessions, summary.UniqueUsers))

	agents, _ := t.analyticsStore.TopAgents(ctx, filter, limit)
	if len(agents) > 0 {
		sb.WriteString("### Top Agents\n")
		for i, a := range agents {
			name := a.DisplayName
			if name == "" {
				name = a.AgentKey
			}
			sb.WriteString(fmt.Sprintf("%d. %s — %d requests, %d+%d tokens\n",
				i+1, name, a.TraceCount, a.InputTokens, a.OutputTokens))
		}
		sb.WriteString("\n")
	}

	models, _ := t.analyticsStore.TopModels(ctx, filter, limit)
	if len(models) > 0 {
		sb.WriteString("### Top Models\n")
		for i, m := range models {
			sb.WriteString(fmt.Sprintf("%d. %s (%s) — %d calls, %d+%d tokens\n",
				i+1, m.Model, m.Provider, m.CallCount, m.InputTokens, m.OutputTokens))
		}
		sb.WriteString("\n")
	}

	toolsList, _ := t.analyticsStore.TopTools(ctx, filter, limit)
	if len(toolsList) > 0 {
		sb.WriteString("### Top Tools\n")
		for i, tl := range toolsList {
			sb.WriteString(fmt.Sprintf("%d. %s — %d calls", i+1, tl.ToolName, tl.CallCount))
			if tl.ErrorCount > 0 {
				sb.WriteString(fmt.Sprintf(" (%d errors)", tl.ErrorCount))
			}
			sb.WriteString("\n")
		}
	}

	return NewResult(sb.String())
}

func (t *AnalyticsTool) buildFilter(ctx context.Context, args map[string]interface{}, from, to time.Time) store.AnalyticsFilter {
	filter := store.AnalyticsFilter{From: from, To: to}

	if idStr, ok := args["agent_id"].(string); ok && idStr != "" {
		if id, err := uuid.Parse(idStr); err == nil {
			filter.AgentID = &id
		}
	} else {
		agentID := store.AgentIDFromContext(ctx)
		if agentID != uuid.Nil {
			filter.AgentID = &agentID
		}
	}

	return filter
}

// resolveAnalyticsPeriod parses the period arg and returns from/to times with a human label.
func resolveAnalyticsPeriod(args map[string]interface{}) (from, to time.Time, label string, err error) {
	period, _ := args["period"].(string)
	if period == "" {
		period = "today"
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	switch period {
	case "today":
		return today, now, "Today", nil
	case "yesterday":
		return today.Add(-24 * time.Hour), today, "Yesterday", nil
	case "7d":
		return now.Add(-7 * 24 * time.Hour), now, "Last 7 days", nil
	case "30d":
		return now.Add(-30 * 24 * time.Hour), now, "Last 30 days", nil
	case "custom":
		fromStr, _ := args["from"].(string)
		toStr, _ := args["to"].(string)
		if fromStr == "" {
			return time.Time{}, time.Time{}, "", fmt.Errorf("'from' is required for custom period")
		}
		f, parseErr := time.Parse(time.RFC3339, fromStr)
		if parseErr != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("invalid 'from' date: %v", parseErr)
		}
		end := now
		if toStr != "" {
			end, parseErr = time.Parse(time.RFC3339, toStr)
			if parseErr != nil {
				return time.Time{}, time.Time{}, "", fmt.Errorf("invalid 'to' date: %v", parseErr)
			}
		}
		lbl := fmt.Sprintf("%s to %s", f.Format("2006-01-02"), end.Format("2006-01-02"))
		return f, end, lbl, nil
	default:
		return time.Time{}, time.Time{}, "", fmt.Errorf("unknown period: %s", period)
	}
}
