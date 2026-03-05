package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/claudecode"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ClaudeCodeMethods handles cc.* RPC methods.
type ClaudeCodeMethods struct {
	ccStore store.CCStore
	manager *claudecode.ProcessManager
	msgBus  *bus.MessageBus
}

func NewClaudeCodeMethods(ccStore store.CCStore, manager *claudecode.ProcessManager, msgBus *bus.MessageBus) *ClaudeCodeMethods {
	return &ClaudeCodeMethods{ccStore: ccStore, manager: manager, msgBus: msgBus}
}

func (m *ClaudeCodeMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodCCProjectsList, m.handleProjectsList)
	router.Register(protocol.MethodCCProjectsCreate, m.handleProjectsCreate)
	router.Register(protocol.MethodCCProjectsGet, m.handleProjectsGet)
	router.Register(protocol.MethodCCProjectsUpdate, m.handleProjectsUpdate)
	router.Register(protocol.MethodCCProjectsDelete, m.handleProjectsDelete)
	router.Register(protocol.MethodCCSessionsList, m.handleSessionsList)
	router.Register(protocol.MethodCCSessionsStart, m.handleSessionsStart)
	router.Register(protocol.MethodCCSessionsGet, m.handleSessionsGet)
	router.Register(protocol.MethodCCSessionsPrompt, m.handleSessionsPrompt)
	router.Register(protocol.MethodCCSessionsStop, m.handleSessionsStop)
	router.Register(protocol.MethodCCSessionsLogs, m.handleSessionsLogs)
}

// --- Projects ---

func (m *ClaudeCodeMethods) handleProjectsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		OwnerID string `json:"owner_id"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.OwnerID == "" {
		params.OwnerID = client.UserID()
	}
	projects, err := m.ccStore.ListProjects(context.Background(), params.OwnerID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if projects == nil {
		projects = []store.CCProjectData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"projects": projects, "count": len(projects)}))
}

func (m *ClaudeCodeMethods) handleProjectsCreate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var p store.CCProjectData
	if err := json.Unmarshal(req.Params, &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	if p.Name == "" || p.Slug == "" || p.WorkDir == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "name, slug, and work_dir required"))
		return
	}
	if err := claudecode.ValidateSlug(p.Slug); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
		return
	}
	if err := claudecode.ValidateWorkDir(p.WorkDir); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
		return
	}
	if p.OwnerID == "" {
		p.OwnerID = client.UserID()
	}
	if err := m.ccStore.CreateProject(context.Background(), &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"project": p}))
}

func (m *ClaudeCodeMethods) handleProjectsGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	p, err := m.ccStore.GetProject(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"project": p}))
}

func (m *ClaudeCodeMethods) handleProjectsUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID      string         `json:"id"`
		Updates map[string]any `json:"updates"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id and updates required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	delete(params.Updates, "id")
	delete(params.Updates, "owner_id")
	delete(params.Updates, "created_at")
	if wd, ok := params.Updates["work_dir"].(string); ok && wd != "" {
		if err := claudecode.ValidateWorkDir(wd); err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
			return
		}
	}
	if slug, ok := params.Updates["slug"].(string); ok && slug != "" {
		if err := claudecode.ValidateSlug(slug); err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
			return
		}
	}
	if err := m.ccStore.UpdateProject(context.Background(), id, params.Updates); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ClaudeCodeMethods) handleProjectsDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	if err := m.ccStore.DeleteProject(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

// --- Sessions ---

func (m *ClaudeCodeMethods) handleSessionsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ProjectID string `json:"project_id"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ProjectID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "project_id required"))
		return
	}
	projectID, err := uuid.Parse(params.ProjectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid project_id"))
		return
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}
	sessions, total, err := m.ccStore.ListSessions(context.Background(), projectID, params.Limit, params.Offset)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if sessions == nil {
		sessions = []store.CCSessionData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"sessions": sessions, "total": total}))
}

func (m *ClaudeCodeMethods) handleSessionsStart(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil || m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ProjectID    string   `json:"project_id"`
		Prompt       string   `json:"prompt"`
		AllowedTools []string `json:"allowed_tools"`
		Model        string   `json:"model"`
		MaxTurns     int      `json:"max_turns"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	if params.ProjectID == "" || params.Prompt == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "project_id and prompt required"))
		return
	}
	projectID, err := uuid.Parse(params.ProjectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid project_id"))
		return
	}

	proj, err := m.ccStore.GetProject(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "project not found"))
		return
	}

	allowedTools := params.AllowedTools
	if len(allowedTools) == 0 && proj.AllowedTools != nil {
		_ = json.Unmarshal(proj.AllowedTools, &allowedTools)
	}

	opts := claudecode.StartOpts{
		ProjectID:    projectID,
		WorkDir:      proj.WorkDir,
		Prompt:       params.Prompt,
		AllowedTools: allowedTools,
		Model:        params.Model,
		MaxTurns:     params.MaxTurns,
	}

	sessionID, err := m.manager.Start(context.Background(), opts, client.UserID())
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	sess, _ := m.ccStore.GetSession(context.Background(), sessionID)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"session": sess}))
}

func (m *ClaudeCodeMethods) handleSessionsGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	sess, err := m.ccStore.GetSession(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"session": sess}))
}

func (m *ClaudeCodeMethods) handleSessionsPrompt(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID     string `json:"id"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" || params.Prompt == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id and prompt required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	if err := m.manager.SendPrompt(context.Background(), id, params.Prompt); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ClaudeCodeMethods) handleSessionsStop(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	if err := m.manager.Stop(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "stopped"}))
}

func (m *ClaudeCodeMethods) handleSessionsLogs(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.ccStore == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "claude code not available"))
		return
	}
	var params struct {
		SessionID string `json:"session_id"`
		AfterSeq  int    `json:"after_seq"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.SessionID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "session_id required"))
		return
	}
	sessionID, err := uuid.Parse(params.SessionID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid session_id"))
		return
	}
	if params.Limit <= 0 {
		params.Limit = 500
	}
	logs, err := m.ccStore.GetLogs(context.Background(), sessionID, params.AfterSeq, params.Limit)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if logs == nil {
		logs = []store.CCSessionLogData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"logs": logs}))
}
