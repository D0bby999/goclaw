package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// CC project status constants.
const (
	CCProjectStatusActive   = "active"
	CCProjectStatusArchived = "archived"
)

// CC session status constants.
const (
	CCSessionStatusStarting  = "starting"
	CCSessionStatusRunning   = "running"
	CCSessionStatusStopped   = "stopped"
	CCSessionStatusFailed    = "failed"
	CCSessionStatusCompleted = "completed"
)

// CCProjectData represents a Claude Code project.
type CCProjectData struct {
	BaseModel
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	WorkDir      string          `json:"work_dir"`
	Description  string          `json:"description,omitempty"`
	AllowedTools json.RawMessage `json:"allowed_tools,omitempty"`
	ClaudeConfig json.RawMessage `json:"claude_config,omitempty"`
	MaxSessions  int             `json:"max_sessions"`
	OwnerID      string          `json:"owner_id"`
	TeamID       *uuid.UUID      `json:"team_id,omitempty"`
	Status       string          `json:"status"`
}

// CCSessionData represents a Claude Code session.
type CCSessionData struct {
	BaseModel
	ProjectID       uuid.UUID  `json:"project_id"`
	ClaudeSessionID *string    `json:"claude_session_id,omitempty"`
	Label           string     `json:"label,omitempty"`
	Status          string     `json:"status"`
	PID             *int       `json:"pid,omitempty"`
	StartedBy       string     `json:"started_by"`
	InputTokens     int64      `json:"input_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	CostUSD         float64    `json:"cost_usd"`
	Error           *string    `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	StoppedAt       *time.Time `json:"stopped_at,omitempty"`
	// Joined fields
	ProjectName string `json:"project_name,omitempty"`
	ProjectSlug string `json:"project_slug,omitempty"`
}

// CCSessionLogData represents a log entry from Claude Code stream output.
type CCSessionLogData struct {
	ID        uuid.UUID       `json:"id"`
	SessionID uuid.UUID       `json:"session_id"`
	EventType string          `json:"event_type"`
	Content   json.RawMessage `json:"content"`
	Seq       int             `json:"seq"`
	CreatedAt time.Time       `json:"created_at"`
}

// CCStore manages Claude Code projects, sessions, and logs.
type CCStore interface {
	// Projects
	CreateProject(ctx context.Context, p *CCProjectData) error
	GetProject(ctx context.Context, id uuid.UUID) (*CCProjectData, error)
	GetProjectBySlug(ctx context.Context, slug string) (*CCProjectData, error)
	UpdateProject(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeleteProject(ctx context.Context, id uuid.UUID) error
	ListProjects(ctx context.Context, ownerID string) ([]CCProjectData, error)
	ListProjectsByTeam(ctx context.Context, teamID uuid.UUID) ([]CCProjectData, error)

	// Sessions
	CreateSession(ctx context.Context, s *CCSessionData) error
	GetSession(ctx context.Context, id uuid.UUID) (*CCSessionData, error)
	UpdateSession(ctx context.Context, id uuid.UUID, updates map[string]any) error
	ListSessions(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]CCSessionData, int, error)
	ActiveSessionCount(ctx context.Context, projectID uuid.UUID) (int, error)

	// Logs
	AppendLog(ctx context.Context, log *CCSessionLogData) error
	GetLogs(ctx context.Context, sessionID uuid.UUID, afterSeq, limit int) ([]CCSessionLogData, error)
}
