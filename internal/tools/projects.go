package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/claudecode"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ============================================================
// project_session_start
// ============================================================

type ProjectSessionStartTool struct {
	store   store.ProjectStore
	manager *claudecode.ProcessManager
}

func NewProjectSessionStartTool(s store.ProjectStore, m *claudecode.ProcessManager) *ProjectSessionStartTool {
	return &ProjectSessionStartTool{store: s, manager: m}
}

func (t *ProjectSessionStartTool) Name() string { return "project_session_start" }
func (t *ProjectSessionStartTool) Description() string {
	return "Start a new Claude Code session in a project, or resume an existing one by providing session_id."
}

func (t *ProjectSessionStartTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "Project UUID to start session in",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Task prompt for Claude",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Resume an existing session (UUID). If omitted, starts a new session.",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Override the default model",
			},
			"max_turns": map[string]interface{}{
				"type":        "number",
				"description": "Limit the number of agent turns",
			},
		},
		"required": []string{"project_id", "prompt"},
	}
}

func (t *ProjectSessionStartTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	projectIDStr, _ := args["project_id"].(string)
	if projectIDStr == "" {
		return ErrorResult("project_id is required")
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return ErrorResult("invalid project_id: " + err.Error())
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return ErrorResult("prompt is required")
	}

	startedBy := resolveAgentIDString(ctx)

	// Resume existing session
	if sessionIDStr, ok := args["session_id"].(string); ok && sessionIDStr != "" {
		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			return ErrorResult("invalid session_id: " + err.Error())
		}
		if err := t.manager.SendPrompt(ctx, sessionID, prompt); err != nil {
			return ErrorResult("resume failed: " + err.Error())
		}
		out, _ := json.Marshal(map[string]interface{}{
			"session_id": sessionID.String(),
			"status":     "resumed",
		})
		return SilentResult(string(out))
	}

	// Start new session
	proj, err := t.store.GetProject(ctx, projectID)
	if err != nil {
		return ErrorResult("get project: " + err.Error())
	}

	var allowedTools []string
	if proj.AllowedTools != nil {
		_ = json.Unmarshal(proj.AllowedTools, &allowedTools)
	}

	opts := claudecode.StartOpts{
		ProjectID:    projectID,
		WorkDir:      proj.WorkDir,
		Prompt:       prompt,
		AllowedTools: allowedTools,
	}
	if model, ok := args["model"].(string); ok && model != "" {
		opts.Model = model
	}
	if maxTurns, ok := args["max_turns"].(float64); ok && int(maxTurns) > 0 {
		opts.MaxTurns = int(maxTurns)
	}

	sessionID, err := t.manager.Start(ctx, opts, startedBy)
	if err != nil {
		return ErrorResult("start failed: " + err.Error())
	}

	out, _ := json.Marshal(map[string]interface{}{
		"session_id": sessionID.String(),
		"status":     "started",
		"project":    proj.Name,
	})
	return SilentResult(string(out))
}

// ============================================================
// project_session_status
// ============================================================

type ProjectSessionStatusTool struct {
	store   store.ProjectStore
	manager *claudecode.ProcessManager
}

func NewProjectSessionStatusTool(s store.ProjectStore, m *claudecode.ProcessManager) *ProjectSessionStatusTool {
	return &ProjectSessionStatusTool{store: s, manager: m}
}

func (t *ProjectSessionStatusTool) Name() string { return "project_session_status" }
func (t *ProjectSessionStatusTool) Description() string {
	return "Check status of a project session (running/completed/failed, tokens, cost)."
}

func (t *ProjectSessionStatusTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Session UUID to check",
			},
		},
		"required": []string{"session_id"},
	}
}

func (t *ProjectSessionStatusTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	sessionIDStr, _ := args["session_id"].(string)
	if sessionIDStr == "" {
		return ErrorResult("session_id is required")
	}
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return ErrorResult("invalid session_id: " + err.Error())
	}

	sess, err := t.store.GetSession(ctx, sessionID)
	if err != nil {
		return ErrorResult("get session: " + err.Error())
	}

	status := sess.Status
	if t.manager.IsRunning(sessionID) && status != store.ProjectSessionStatusRunning {
		status = store.ProjectSessionStatusRunning
	}

	result := map[string]interface{}{
		"session_id":    sess.ID.String(),
		"project_id":    sess.ProjectID.String(),
		"status":         status,
		"label":          sess.Label,
		"started_by":     sess.StartedBy,
		"input_tokens":   sess.InputTokens,
		"output_tokens":  sess.OutputTokens,
		"cost_usd":       fmt.Sprintf("%.4f", sess.CostUSD),
		"started_at":     sess.StartedAt.Format(time.RFC3339),
		"is_running":     t.manager.IsRunning(sessionID),
	}
	if sess.StoppedAt != nil {
		result["stopped_at"] = sess.StoppedAt.Format(time.RFC3339)
	}
	if sess.Error != nil {
		result["error"] = *sess.Error
	}

	out, _ := json.Marshal(result)
	return SilentResult(string(out))
}

// ============================================================
// project_sessions_list
// ============================================================

type ProjectSessionsListTool struct {
	store store.ProjectStore
}

func NewProjectSessionsListTool(s store.ProjectStore) *ProjectSessionsListTool {
	return &ProjectSessionsListTool{store: s}
}

func (t *ProjectSessionsListTool) Name() string { return "project_sessions_list" }
func (t *ProjectSessionsListTool) Description() string {
	return "List sessions for a project."
}

func (t *ProjectSessionsListTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "Project UUID to list sessions for",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Max sessions to return (default 10)",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Filter by status (starting, running, stopped, failed, completed)",
			},
		},
		"required": []string{"project_id"},
	}
}

func (t *ProjectSessionsListTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	projectIDStr, _ := args["project_id"].(string)
	if projectIDStr == "" {
		return ErrorResult("project_id is required")
	}
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return ErrorResult("invalid project_id: " + err.Error())
	}

	limit := 10
	if v, ok := args["limit"].(float64); ok && int(v) > 0 {
		limit = int(v)
	}

	statusFilter, _ := args["status"].(string)

	sessions, total, err := t.store.ListSessions(ctx, projectID, limit, 0)
	if err != nil {
		return ErrorResult("list sessions: " + err.Error())
	}

	type sessionEntry struct {
		ID        string  `json:"id"`
		Label     string  `json:"label"`
		Status    string  `json:"status"`
		StartedBy string  `json:"started_by"`
		Tokens    int64   `json:"tokens"`
		CostUSD   string  `json:"cost_usd"`
		StartedAt string  `json:"started_at"`
		StoppedAt string  `json:"stopped_at,omitempty"`
	}

	entries := make([]sessionEntry, 0, len(sessions))
	for _, s := range sessions {
		if statusFilter != "" && s.Status != statusFilter {
			continue
		}
		e := sessionEntry{
			ID:        s.ID.String(),
			Label:     s.Label,
			Status:    s.Status,
			StartedBy: s.StartedBy,
			Tokens:    s.InputTokens + s.OutputTokens,
			CostUSD:   fmt.Sprintf("%.4f", s.CostUSD),
			StartedAt: s.StartedAt.Format(time.RFC3339),
		}
		if s.StoppedAt != nil {
			e.StoppedAt = s.StoppedAt.Format(time.RFC3339)
		}
		entries = append(entries, e)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"total":    total,
		"count":    len(entries),
		"sessions": entries,
	})
	return SilentResult(string(out))
}
