package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Project status constants.
const (
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
)

// Project session status constants.
const (
	ProjectSessionStatusStarting  = "starting"
	ProjectSessionStatusRunning   = "running"
	ProjectSessionStatusStopped   = "stopped"
	ProjectSessionStatusFailed    = "failed"
	ProjectSessionStatusCompleted = "completed"
)

// ProjectData represents a project.
type ProjectData struct {
	BaseModel
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	WorkDir      string          `json:"work_dir"`
	Description  string          `json:"description,omitempty"`
	AllowedTools json.RawMessage `json:"allowed_tools,omitempty"`
	ClaudeConfig json.RawMessage `json:"claude_config,omitempty"`
	MaxSessions  int             `json:"max_sessions"`
	MaxDuration  int             `json:"max_duration"`  // max session duration in seconds (0 = unlimited)
	OwnerID      string          `json:"owner_id"`
	TeamID       *uuid.UUID      `json:"team_id,omitempty"`
	Status       string          `json:"status"`
}

// ProjectSessionData represents a project session.
type ProjectSessionData struct {
	BaseModel
	ProjectID       uuid.UUID  `json:"project_id"`
	ClaudeSessionID *string    `json:"claude_session_id,omitempty"`
	Label           string     `json:"label,omitempty"`
	Status          string     `json:"status"`
	PID             *int       `json:"pid,omitempty"`
	StartedBy       string     `json:"started_by"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CacheReadTokens      int64   `json:"cache_read_tokens"`
	CacheCreationTokens  int64   `json:"cache_creation_tokens"`
	CostUSD              float64 `json:"cost_usd"`
	Error           *string    `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	StoppedAt       *time.Time `json:"stopped_at,omitempty"`
	// Joined fields
	ProjectName string `json:"project_name,omitempty"`
	ProjectSlug string `json:"project_slug,omitempty"`
}

// ProjectSessionLogData represents a log entry from session stream output.
type ProjectSessionLogData struct {
	ID        uuid.UUID       `json:"id"`
	SessionID uuid.UUID       `json:"session_id"`
	EventType string          `json:"event_type"`
	Content   json.RawMessage `json:"content"`
	Seq       int             `json:"seq"`
	CreatedAt time.Time       `json:"created_at"`
}

// ProjectMemberData represents a project member (explicit access grant).
type ProjectMemberData struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	AddedBy   string    `json:"added_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectStore manages projects, sessions, and logs.
type ProjectStore interface {
	// Projects
	CreateProject(ctx context.Context, p *ProjectData) error
	GetProject(ctx context.Context, id uuid.UUID) (*ProjectData, error)
	GetProjectBySlug(ctx context.Context, slug string) (*ProjectData, error)
	UpdateProject(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeleteProject(ctx context.Context, id uuid.UUID) error
	ListProjects(ctx context.Context, ownerID string) ([]ProjectData, error)
	ListProjectsByTeam(ctx context.Context, teamID uuid.UUID) ([]ProjectData, error)
	ListAccessibleProjects(ctx context.Context, userID string) ([]ProjectData, error)

	// Members
	AddMember(ctx context.Context, projectID uuid.UUID, userID, role, addedBy string) error
	RemoveMember(ctx context.Context, projectID uuid.UUID, userID string) error
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMemberData, error)
	IsMember(ctx context.Context, projectID uuid.UUID, userID string) (bool, error)

	// Sessions
	CreateSession(ctx context.Context, s *ProjectSessionData) error
	GetSession(ctx context.Context, id uuid.UUID) (*ProjectSessionData, error)
	UpdateSession(ctx context.Context, id uuid.UUID, updates map[string]any) error
	DeleteSession(ctx context.Context, id uuid.UUID) error
	ListSessions(ctx context.Context, projectID uuid.UUID, limit, offset int) ([]ProjectSessionData, int, error)
	ActiveSessionCount(ctx context.Context, projectID uuid.UUID) (int, error)

	// Logs
	AppendLog(ctx context.Context, log *ProjectSessionLogData) error
	GetLogs(ctx context.Context, sessionID uuid.UUID, afterSeq, limit int) ([]ProjectSessionLogData, error)
}
