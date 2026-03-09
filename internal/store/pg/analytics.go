package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGAnalyticsStore implements store.AnalyticsStore backed by Postgres.
type PGAnalyticsStore struct {
	db *sql.DB
}

func NewPGAnalyticsStore(db *sql.DB) *PGAnalyticsStore {
	return &PGAnalyticsStore{db: db}
}

// buildAnalyticsWhere constructs a WHERE clause from an AnalyticsFilter.
// It targets the traces table (aliased as t when joined).
func buildAnalyticsWhere(filter store.AnalyticsFilter, tablePrefix string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	col := func(name string) string {
		if tablePrefix != "" {
			return tablePrefix + "." + name
		}
		return name
	}

	if filter.AgentID != nil {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", col("agent_id"), idx))
		args = append(args, *filter.AgentID)
		idx++
	}
	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", col("user_id"), idx))
		args = append(args, filter.UserID)
		idx++
	}
	if !filter.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("%s >= $%d", col("created_at"), idx))
		args = append(args, filter.From)
		idx++
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("%s <= $%d", col("created_at"), idx))
		args = append(args, filter.To)
		idx++
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// Summary returns aggregated metrics over traces for the given filter period.
func (s *PGAnalyticsStore) Summary(ctx context.Context, filter store.AnalyticsFilter) (*store.AnalyticsSummary, error) {
	where, args := buildAnalyticsWhere(filter, "")

	q := `SELECT
		COUNT(*)                                          AS total_traces,
		COUNT(*) FILTER (WHERE status = 'completed')     AS completed_traces,
		COUNT(*) FILTER (WHERE status = 'error')         AS error_traces,
		COALESCE(SUM(total_input_tokens), 0)             AS total_input_tokens,
		COALESCE(SUM(total_output_tokens), 0)            AS total_output_tokens,
		COALESCE(SUM(llm_call_count), 0)                 AS total_llm_calls,
		COALESCE(SUM(tool_call_count), 0)                AS total_tool_calls,
		COUNT(DISTINCT user_id)                          AS unique_users
	FROM traces` + where

	var summary store.AnalyticsSummary
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&summary.TotalTraces,
		&summary.CompletedTraces,
		&summary.ErrorTraces,
		&summary.TotalInputTokens,
		&summary.TotalOutputTokens,
		&summary.TotalLLMCalls,
		&summary.TotalToolCalls,
		&summary.UniqueUsers,
	)
	if err != nil {
		return nil, err
	}

	// Active sessions: updated in the same time window (or all if no window set).
	sessionWhere, sessionArgs := buildSessionsWhere(filter)
	var activeSessions int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sessions"+sessionWhere, sessionArgs...,
	).Scan(&activeSessions); err != nil {
		activeSessions = 0
	}
	summary.ActiveSessions = activeSessions
	summary.From = filter.From
	summary.To = filter.To

	return &summary, nil
}

// buildSessionsWhere builds a WHERE clause for the sessions table using the filter's time window.
func buildSessionsWhere(filter store.AnalyticsFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	idx := 1

	if !filter.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("updated_at >= $%d", idx))
		args = append(args, filter.From)
		idx++
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("updated_at <= $%d", idx))
		args = append(args, filter.To)
		idx++
	}

	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// TopAgents returns agents ranked by trace count within the filter period.
func (s *PGAnalyticsStore) TopAgents(ctx context.Context, filter store.AnalyticsFilter, limit int) ([]store.AgentUsage, error) {
	if limit <= 0 {
		limit = 10
	}

	where, args := buildAnalyticsWhere(filter, "t")
	nextIdx := len(args) + 1

	q := fmt.Sprintf(`
		SELECT
			t.agent_id,
			COALESCE(a.agent_key, ''),
			COALESCE(a.display_name, ''),
			COUNT(*)                                      AS trace_count,
			COALESCE(SUM(t.total_input_tokens), 0)        AS input_tokens,
			COALESCE(SUM(t.total_output_tokens), 0)       AS output_tokens,
			COUNT(*) FILTER (WHERE t.status = 'error')   AS error_count
		FROM traces t
		LEFT JOIN agents a ON a.id = t.agent_id
		%s
		GROUP BY t.agent_id, a.agent_key, a.display_name
		ORDER BY trace_count DESC
		LIMIT $%d`, where, nextIdx)

	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.AgentUsage
	for rows.Next() {
		var u store.AgentUsage
		var agentID *uuid.UUID
		if err := rows.Scan(&agentID, &u.AgentKey, &u.DisplayName,
			&u.TraceCount, &u.InputTokens, &u.OutputTokens, &u.ErrorCount); err != nil {
			continue
		}
		if agentID != nil {
			u.AgentID = *agentID
		}
		result = append(result, u)
	}
	return result, nil
}

// TopModels returns LLM models ranked by call count within the filter period.
func (s *PGAnalyticsStore) TopModels(ctx context.Context, filter store.AnalyticsFilter, limit int) ([]store.ModelUsage, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build WHERE for spans table (use trace_id join to apply agent/user filters)
	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, "s.span_type = 'llm_call'")
	conditions = append(conditions, "s.model IS NOT NULL")

	if filter.AgentID != nil {
		conditions = append(conditions, fmt.Sprintf("s.agent_id = $%d", idx))
		args = append(args, *filter.AgentID)
		idx++
	}
	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("t.user_id = $%d", idx))
		args = append(args, filter.UserID)
		idx++
	}
	if !filter.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at >= $%d", idx))
		args = append(args, filter.From)
		idx++
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at <= $%d", idx))
		args = append(args, filter.To)
		idx++
	}

	needsTraceJoin := filter.UserID != ""
	joinClause := ""
	if needsTraceJoin {
		joinClause = "JOIN traces t ON t.id = s.trace_id"
	}

	q := fmt.Sprintf(`
		SELECT
			COALESCE(s.model, ''),
			COALESCE(s.provider, ''),
			COUNT(*)                               AS call_count,
			COALESCE(SUM(s.input_tokens), 0)       AS input_tokens,
			COALESCE(SUM(s.output_tokens), 0)      AS output_tokens
		FROM spans s
		%s
		WHERE %s
		GROUP BY s.model, s.provider
		ORDER BY call_count DESC
		LIMIT $%d`,
		joinClause,
		strings.Join(conditions, " AND "),
		idx,
	)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.ModelUsage
	for rows.Next() {
		var m store.ModelUsage
		if err := rows.Scan(&m.Model, &m.Provider, &m.CallCount, &m.InputTokens, &m.OutputTokens); err != nil {
			continue
		}
		result = append(result, m)
	}
	return result, nil
}

// TopTools returns tools ranked by call count within the filter period.
func (s *PGAnalyticsStore) TopTools(ctx context.Context, filter store.AnalyticsFilter, limit int) ([]store.TopTool, error) {
	if limit <= 0 {
		limit = 10
	}

	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, "s.span_type = 'tool_call'")
	conditions = append(conditions, "s.tool_name IS NOT NULL")

	if filter.AgentID != nil {
		conditions = append(conditions, fmt.Sprintf("s.agent_id = $%d", idx))
		args = append(args, *filter.AgentID)
		idx++
	}
	if filter.UserID != "" {
		conditions = append(conditions, fmt.Sprintf("t.user_id = $%d", idx))
		args = append(args, filter.UserID)
		idx++
	}
	if !filter.From.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at >= $%d", idx))
		args = append(args, filter.From)
		idx++
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, fmt.Sprintf("s.created_at <= $%d", idx))
		args = append(args, filter.To)
		idx++
	}

	needsTraceJoin := filter.UserID != ""
	joinClause := ""
	if needsTraceJoin {
		joinClause = "JOIN traces t ON t.id = s.trace_id"
	}

	q := fmt.Sprintf(`
		SELECT
			COALESCE(s.tool_name, ''),
			COUNT(*)                                     AS call_count,
			COUNT(*) FILTER (WHERE s.status = 'error')  AS error_count
		FROM spans s
		%s
		WHERE %s
		GROUP BY s.tool_name
		ORDER BY call_count DESC
		LIMIT $%d`,
		joinClause,
		strings.Join(conditions, " AND "),
		idx,
	)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.TopTool
	for rows.Next() {
		var tt store.TopTool
		if err := rows.Scan(&tt.ToolName, &tt.CallCount, &tt.ErrorCount); err != nil {
			continue
		}
		result = append(result, tt)
	}
	return result, nil
}
