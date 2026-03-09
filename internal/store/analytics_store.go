package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AnalyticsSummary holds aggregated usage metrics for a time period.
type AnalyticsSummary struct {
	Period            string    `json:"period"`
	From              time.Time `json:"from"`
	To                time.Time `json:"to"`
	TotalTraces       int       `json:"totalTraces"`
	CompletedTraces   int       `json:"completedTraces"`
	ErrorTraces       int       `json:"errorTraces"`
	TotalInputTokens  int64     `json:"totalInputTokens"`
	TotalOutputTokens int64     `json:"totalOutputTokens"`
	TotalLLMCalls     int       `json:"totalLLMCalls"`
	TotalToolCalls    int       `json:"totalToolCalls"`
	ActiveSessions    int       `json:"activeSessions"`
	UniqueUsers       int       `json:"uniqueUsers"`
}

// AgentUsage holds per-agent usage metrics.
type AgentUsage struct {
	AgentID      uuid.UUID `json:"agentId"`
	AgentKey     string    `json:"agentKey"`
	DisplayName  string    `json:"displayName"`
	TraceCount   int       `json:"traceCount"`
	InputTokens  int64     `json:"inputTokens"`
	OutputTokens int64     `json:"outputTokens"`
	ErrorCount   int       `json:"errorCount"`
}

// ModelUsage holds per-model LLM usage metrics.
type ModelUsage struct {
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	CallCount    int    `json:"callCount"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
}

// TopTool holds tool usage metrics.
type TopTool struct {
	ToolName   string `json:"toolName"`
	CallCount  int    `json:"callCount"`
	ErrorCount int    `json:"errorCount"`
}

// AnalyticsFilter defines filters for analytics queries.
type AnalyticsFilter struct {
	AgentID *uuid.UUID
	UserID  string
	From    time.Time
	To      time.Time
}

// AnalyticsStore provides read-only aggregate analytics over traces/spans.
type AnalyticsStore interface {
	Summary(ctx context.Context, filter AnalyticsFilter) (*AnalyticsSummary, error)
	TopAgents(ctx context.Context, filter AnalyticsFilter, limit int) ([]AgentUsage, error)
	TopModels(ctx context.Context, filter AnalyticsFilter, limit int) ([]ModelUsage, error)
	TopTools(ctx context.Context, filter AnalyticsFilter, limit int) ([]TopTool, error)
}
