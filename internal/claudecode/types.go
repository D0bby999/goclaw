package claudecode

import (
	"encoding/json"

	"github.com/google/uuid"
)

// StreamEvent represents a parsed line from Claude Code stream-json output.
type StreamEvent struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Raw       json.RawMessage `json:"raw"`
	// Extracted from "result" events
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// StartOpts configures a new Claude Code process.
type StartOpts struct {
	ProjectID    uuid.UUID
	WorkDir      string
	Prompt       string
	ResumeID     string            // claude session_id for --resume (empty = new)
	AllowedTools []string          // tool names for --allowedTools
	EnvVars      map[string]string // extra env vars
	Model        string            // optional model override
	MaxTurns     int               // --max-turns (0 = unlimited)
	UseWorktree  bool              // create git worktree for isolation
	Scope        string            // optional scope hint
}

// EventCallback receives parsed stream events for a session.
type EventCallback func(sessionID uuid.UUID, event StreamEvent)
