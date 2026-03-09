package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// mockAnalyticsStore implements store.AnalyticsStore for testing.
type mockAnalyticsStore struct {
	summary    *store.AnalyticsSummary
	agents     []store.AgentUsage
	models     []store.ModelUsage
	tools      []store.TopTool
	summaryErr error
	agentsErr  error
	modelsErr  error
	toolsErr   error

	// capture filter for assertions
	lastFilter store.AnalyticsFilter
	lastLimit  int
}

func (m *mockAnalyticsStore) Summary(ctx context.Context, f store.AnalyticsFilter) (*store.AnalyticsSummary, error) {
	m.lastFilter = f
	if m.summaryErr != nil {
		return nil, m.summaryErr
	}
	return m.summary, nil
}

func (m *mockAnalyticsStore) TopAgents(ctx context.Context, f store.AnalyticsFilter, limit int) ([]store.AgentUsage, error) {
	m.lastFilter = f
	m.lastLimit = limit
	if m.agentsErr != nil {
		return nil, m.agentsErr
	}
	return m.agents, nil
}

func (m *mockAnalyticsStore) TopModels(ctx context.Context, f store.AnalyticsFilter, limit int) ([]store.ModelUsage, error) {
	m.lastFilter = f
	m.lastLimit = limit
	if m.modelsErr != nil {
		return nil, m.modelsErr
	}
	return m.models, nil
}

func (m *mockAnalyticsStore) TopTools(ctx context.Context, f store.AnalyticsFilter, limit int) ([]store.TopTool, error) {
	m.lastFilter = f
	m.lastLimit = limit
	if m.toolsErr != nil {
		return nil, m.toolsErr
	}
	return m.tools, nil
}

func newTestAnalyticsTool() (*AnalyticsTool, *mockAnalyticsStore) {
	ms := &mockAnalyticsStore{
		summary: &store.AnalyticsSummary{
			TotalTraces:       100,
			CompletedTraces:   95,
			ErrorTraces:       5,
			TotalInputTokens:  500000,
			TotalOutputTokens: 200000,
			TotalLLMCalls:     150,
			TotalToolCalls:    300,
			ActiveSessions:    10,
			UniqueUsers:       3,
		},
		agents: []store.AgentUsage{
			{AgentID: uuid.New(), AgentKey: "agent-1", DisplayName: "My Agent", TraceCount: 50, InputTokens: 250000, OutputTokens: 100000, ErrorCount: 2},
			{AgentID: uuid.New(), AgentKey: "agent-2", DisplayName: "", TraceCount: 30, InputTokens: 150000, OutputTokens: 60000, ErrorCount: 1},
		},
		models: []store.ModelUsage{
			{Model: "claude-sonnet-4-20250514", Provider: "anthropic", CallCount: 80, InputTokens: 300000, OutputTokens: 120000},
			{Model: "gpt-4o", Provider: "openai", CallCount: 40, InputTokens: 100000, OutputTokens: 50000},
		},
		tools: []store.TopTool{
			{ToolName: "web_fetch", CallCount: 100, ErrorCount: 3},
			{ToolName: "exec", CallCount: 80, ErrorCount: 0},
		},
	}
	tool := NewAnalyticsTool(ms)
	return tool, ms
}

func TestAnalyticsTool_Metadata(t *testing.T) {
	tool, _ := newTestAnalyticsTool()

	if tool.Name() != "analytics" {
		t.Errorf("expected name 'analytics', got %q", tool.Name())
	}
	if !strings.Contains(tool.Description(), "summary") {
		t.Error("description should mention summary action")
	}

	params := tool.Parameters()
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters should have properties")
	}
	if _, ok := props["action"]; !ok {
		t.Error("should have action parameter")
	}
	if _, ok := props["period"]; !ok {
		t.Error("should have period parameter")
	}

	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "action" {
		t.Error("required should be [action]")
	}
}

func TestAnalyticsTool_InvalidAction(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "invalid",
	})
	if !result.IsError {
		t.Error("expected error for invalid action")
	}
	if !strings.Contains(result.ForLLM, "summary, report, query") {
		t.Error("error should list valid actions")
	}
}

func TestAnalyticsTool_Summary_DefaultPeriod(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "Usage Summary (Today)") {
		t.Error("should contain 'Usage Summary (Today)'")
	}
	if !strings.Contains(result.ForLLM, "100 total") {
		t.Error("should contain total traces count")
	}
	if !strings.Contains(result.ForLLM, "95 completed") {
		t.Error("should contain completed traces count")
	}
	if !strings.Contains(result.ForLLM, "5 errors") {
		t.Error("should contain error traces count")
	}
	if !strings.Contains(result.ForLLM, "500000 input") {
		t.Error("should contain input tokens")
	}
	if !strings.Contains(result.ForLLM, "200000 output") {
		t.Error("should contain output tokens")
	}
}

func TestAnalyticsTool_Summary_Periods(t *testing.T) {
	tests := []struct {
		period string
		label  string
	}{
		{"today", "Today"},
		{"yesterday", "Yesterday"},
		{"7d", "Last 7 days"},
		{"30d", "Last 30 days"},
	}
	for _, tc := range tests {
		t.Run(tc.period, func(t *testing.T) {
			tool, _ := newTestAnalyticsTool()
			result := tool.Execute(context.Background(), map[string]interface{}{
				"action": "summary",
				"period": tc.period,
			})
			if result.IsError {
				t.Fatalf("unexpected error: %s", result.ForLLM)
			}
			if !strings.Contains(result.ForLLM, tc.label) {
				t.Errorf("expected label %q in output", tc.label)
			}
		})
	}
}

func TestAnalyticsTool_Summary_CustomPeriod(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
		"period": "custom",
		"from":   "2026-03-01T00:00:00Z",
		"to":     "2026-03-09T00:00:00Z",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "2026-03-01 to 2026-03-09") {
		t.Error("should contain custom date range in label")
	}
	// Verify filter dates
	expectedFrom, _ := time.Parse(time.RFC3339, "2026-03-01T00:00:00Z")
	expectedTo, _ := time.Parse(time.RFC3339, "2026-03-09T00:00:00Z")
	if !ms.lastFilter.From.Equal(expectedFrom) {
		t.Errorf("from: expected %v, got %v", expectedFrom, ms.lastFilter.From)
	}
	if !ms.lastFilter.To.Equal(expectedTo) {
		t.Errorf("to: expected %v, got %v", expectedTo, ms.lastFilter.To)
	}
}

func TestAnalyticsTool_Summary_CustomPeriod_MissingFrom(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
		"period": "custom",
	})
	if !result.IsError {
		t.Error("expected error when 'from' is missing for custom period")
	}
	if !strings.Contains(result.ForLLM, "'from' is required") {
		t.Error("should mention 'from' is required")
	}
}

func TestAnalyticsTool_Summary_CustomPeriod_InvalidFrom(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
		"period": "custom",
		"from":   "not-a-date",
	})
	if !result.IsError {
		t.Error("expected error for invalid 'from' date")
	}
	if !strings.Contains(result.ForLLM, "invalid 'from'") {
		t.Error("should mention invalid 'from'")
	}
}

func TestAnalyticsTool_Summary_CustomPeriod_InvalidTo(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
		"period": "custom",
		"from":   "2026-03-01T00:00:00Z",
		"to":     "bad-date",
	})
	if !result.IsError {
		t.Error("expected error for invalid 'to' date")
	}
	if !strings.Contains(result.ForLLM, "invalid 'to'") {
		t.Error("should mention invalid 'to'")
	}
}

func TestAnalyticsTool_Summary_UnknownPeriod(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
		"period": "hourly",
	})
	if !result.IsError {
		t.Error("expected error for unknown period")
	}
	if !strings.Contains(result.ForLLM, "unknown period") {
		t.Error("should mention unknown period")
	}
}

func TestAnalyticsTool_Summary_StoreError(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	ms.summaryErr = fmt.Errorf("db connection failed")
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
	})
	if !result.IsError {
		t.Error("expected error when store fails")
	}
	if !strings.Contains(result.ForLLM, "db connection failed") {
		t.Error("should contain store error message")
	}
}

func TestAnalyticsTool_Report(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "report",
		"period": "7d",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	output := result.ForLLM

	// Check sections
	if !strings.Contains(output, "Analytics Report (Last 7 days)") {
		t.Error("should contain report title with period")
	}
	if !strings.Contains(output, "### Overview") {
		t.Error("should contain overview section")
	}
	if !strings.Contains(output, "### Top Agents") {
		t.Error("should contain top agents section")
	}
	if !strings.Contains(output, "### Top Models") {
		t.Error("should contain top models section")
	}
	if !strings.Contains(output, "### Top Tools") {
		t.Error("should contain top tools section")
	}

	// Agent display
	if !strings.Contains(output, "My Agent") {
		t.Error("should show agent display name")
	}
	if !strings.Contains(output, "agent-2") {
		t.Error("should fall back to agent key when display name empty")
	}

	// Model display
	if !strings.Contains(output, "claude-sonnet-4-20250514") {
		t.Error("should show model name")
	}
	if !strings.Contains(output, "(anthropic)") {
		t.Error("should show provider")
	}

	// Tool display
	if !strings.Contains(output, "web_fetch") {
		t.Error("should show tool name")
	}
	if !strings.Contains(output, "(3 errors)") {
		t.Error("should show error count for tools with errors")
	}
	// exec has 0 errors - should NOT show error count
	if strings.Contains(output, "exec — 80 calls (0 errors)") {
		t.Error("should not show error count when 0")
	}
}

func TestAnalyticsTool_Report_CustomLimit(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	tool.Execute(context.Background(), map[string]interface{}{
		"action": "report",
		"limit":  float64(5),
	})
	if ms.lastLimit != 5 {
		t.Errorf("expected limit 5, got %d", ms.lastLimit)
	}
}

func TestAnalyticsTool_Report_DefaultLimit(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	tool.Execute(context.Background(), map[string]interface{}{
		"action": "report",
	})
	if ms.lastLimit != 10 {
		t.Errorf("expected default limit 10, got %d", ms.lastLimit)
	}
}

func TestAnalyticsTool_Query_SameAsReport(t *testing.T) {
	tool, _ := newTestAnalyticsTool()
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "query",
		"period": "30d",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, "Analytics Report (Last 30 days)") {
		t.Error("query action should produce same output as report")
	}
}

func TestAnalyticsTool_Report_EmptyData(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	ms.agents = nil
	ms.models = nil
	ms.tools = nil
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "report",
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ForLLM)
	}
	output := result.ForLLM
	if strings.Contains(output, "### Top Agents") {
		t.Error("should not show Top Agents section when empty")
	}
	if strings.Contains(output, "### Top Models") {
		t.Error("should not show Top Models section when empty")
	}
	if strings.Contains(output, "### Top Tools") {
		t.Error("should not show Top Tools section when empty")
	}
}

func TestAnalyticsTool_Report_StoreError(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	ms.summaryErr = fmt.Errorf("connection timeout")
	result := tool.Execute(context.Background(), map[string]interface{}{
		"action": "report",
	})
	if !result.IsError {
		t.Error("expected error when store fails")
	}
}

func TestAnalyticsTool_BuildFilter_ExplicitAgentID(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	agentID := uuid.New()
	tool.Execute(context.Background(), map[string]interface{}{
		"action":   "summary",
		"agent_id": agentID.String(),
	})
	if ms.lastFilter.AgentID == nil {
		t.Fatal("expected agent_id filter to be set")
	}
	if *ms.lastFilter.AgentID != agentID {
		t.Errorf("expected agent_id %v, got %v", agentID, *ms.lastFilter.AgentID)
	}
}

func TestAnalyticsTool_BuildFilter_ContextAgentID(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	agentID := uuid.New()
	ctx := store.WithAgentID(context.Background(), agentID)
	tool.Execute(ctx, map[string]interface{}{
		"action": "summary",
	})
	if ms.lastFilter.AgentID == nil {
		t.Fatal("expected agent_id from context to be set")
	}
	if *ms.lastFilter.AgentID != agentID {
		t.Errorf("expected agent_id %v, got %v", agentID, *ms.lastFilter.AgentID)
	}
}

func TestAnalyticsTool_BuildFilter_NoAgentID(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	// No agent_id in args or context
	tool.Execute(context.Background(), map[string]interface{}{
		"action": "summary",
	})
	if ms.lastFilter.AgentID != nil {
		t.Error("expected agent_id filter to be nil when not provided")
	}
}

func TestAnalyticsTool_BuildFilter_InvalidAgentID(t *testing.T) {
	tool, ms := newTestAnalyticsTool()
	tool.Execute(context.Background(), map[string]interface{}{
		"action":   "summary",
		"agent_id": "not-a-uuid",
	})
	// Invalid UUID should be ignored, fallback to no agent filter
	if ms.lastFilter.AgentID != nil {
		t.Error("invalid agent_id should be ignored")
	}
}

func TestResolveAnalyticsPeriod_Today(t *testing.T) {
	from, to, label, err := resolveAnalyticsPeriod(map[string]interface{}{"period": "today"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "Today" {
		t.Errorf("expected label 'Today', got %q", label)
	}
	now := time.Now().UTC()
	expectedMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if !from.Equal(expectedMidnight) {
		t.Errorf("from should be midnight today, got %v", from)
	}
	if to.After(time.Now().UTC().Add(time.Second)) {
		t.Error("to should be close to now")
	}
}

func TestResolveAnalyticsPeriod_Yesterday(t *testing.T) {
	from, to, label, err := resolveAnalyticsPeriod(map[string]interface{}{"period": "yesterday"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "Yesterday" {
		t.Errorf("expected label 'Yesterday', got %q", label)
	}
	now := time.Now().UTC()
	expectedTo := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	expectedFrom := expectedTo.Add(-24 * time.Hour)
	if !from.Equal(expectedFrom) {
		t.Errorf("from should be yesterday midnight, got %v", from)
	}
	if !to.Equal(expectedTo) {
		t.Errorf("to should be today midnight, got %v", to)
	}
}

func TestResolveAnalyticsPeriod_EmptyDefaultsToToday(t *testing.T) {
	_, _, label, err := resolveAnalyticsPeriod(map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != "Today" {
		t.Errorf("empty period should default to Today, got %q", label)
	}
}
