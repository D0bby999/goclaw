package methods

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/claudecode"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ProjectsMethods handles projects.* RPC methods.
type ProjectsMethods struct {
	store     store.ProjectStore
	manager   *claudecode.ProcessManager
	msgBus    *bus.MessageBus
	teamStore store.TeamStore
	isOwner   func(string) bool
}

func NewProjectsMethods(store store.ProjectStore, manager *claudecode.ProcessManager, msgBus *bus.MessageBus, teamStore store.TeamStore, isOwner func(string) bool) *ProjectsMethods {
	return &ProjectsMethods{store: store, manager: manager, msgBus: msgBus, teamStore: teamStore, isOwner: isOwner}
}

func (m *ProjectsMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodProjectsList, m.handleProjectsList)
	router.Register(protocol.MethodProjectsCreate, m.handleProjectsCreate)
	router.Register(protocol.MethodProjectsGet, m.handleProjectsGet)
	router.Register(protocol.MethodProjectsUpdate, m.handleProjectsUpdate)
	router.Register(protocol.MethodProjectsDelete, m.handleProjectsDelete)
	router.Register(protocol.MethodProjectSessionsList, m.handleSessionsList)
	router.Register(protocol.MethodProjectSessionsStart, m.handleSessionsStart)
	router.Register(protocol.MethodProjectSessionsGet, m.handleSessionsGet)
	router.Register(protocol.MethodProjectSessionsPrompt, m.handleSessionsPrompt)
	router.Register(protocol.MethodProjectSessionsStop, m.handleSessionsStop)
	router.Register(protocol.MethodProjectSessionsDelete, m.handleSessionsDelete)
	router.Register(protocol.MethodProjectSessionsUpdate, m.handleSessionsUpdate)
	router.Register(protocol.MethodProjectSessionsLogs, m.handleSessionsLogs)
	router.Register(protocol.MethodProjectMembersList, m.handleMembersList)
	router.Register(protocol.MethodProjectMembersAdd, m.handleMembersAdd)
	router.Register(protocol.MethodProjectMembersRemove, m.handleMembersRemove)
}

// canAccess checks if a WS client user can access a project.
func (m *ProjectsMethods) canAccess(ctx context.Context, project *store.ProjectData, userID string) bool {
	if m.isOwner(userID) {
		return true
	}
	if project.OwnerID == userID {
		return true
	}
	if ok, _ := m.store.IsMember(ctx, project.ID, userID); ok {
		return true
	}
	if project.TeamID != nil && m.teamStore != nil {
		team, err := m.teamStore.GetTeam(ctx, *project.TeamID)
		if err == nil && team != nil {
			if team.CreatedBy == userID {
				return true
			}
			var settings struct {
				AllowUserIDs []string `json:"allow_user_ids"`
			}
			if team.Settings != nil {
				_ = json.Unmarshal(team.Settings, &settings)
			}
			if slices.Contains(settings.AllowUserIDs, userID) {
				return true
			}
		}
	}
	return false
}

// canModify checks if a user can modify a project (owner or system owner only).
func (m *ProjectsMethods) canModify(project *store.ProjectData, userID string) bool {
	return m.isOwner(userID) || project.OwnerID == userID
}

// --- Projects ---

func (m *ProjectsMethods) handleProjectsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var params struct {
		TeamID string `json:"team_id"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	userID := client.UserID()
	var projects []store.ProjectData
	var err error
	if params.TeamID != "" {
		teamID, parseErr := uuid.Parse(params.TeamID)
		if parseErr != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid team_id"))
			return
		}
		projects, err = m.store.ListProjectsByTeam(context.Background(), teamID)
	} else if m.isOwner(userID) {
		// System owner sees all active projects
		projects, err = m.store.ListProjects(context.Background(), "")
	} else if userID != "" {
		projects, err = m.store.ListAccessibleProjects(context.Background(), userID)
	} else {
		projects = []store.ProjectData{}
	}
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if projects == nil {
		projects = []store.ProjectData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"projects": projects, "count": len(projects)}))
}

func (m *ProjectsMethods) handleProjectsCreate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var p store.ProjectData
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
	if err := m.store.CreateProject(context.Background(), &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"project": p}))
}

func (m *ProjectsMethods) handleProjectsGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	p, err := m.store.GetProject(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if !m.canAccess(context.Background(), p, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"project": p}))
}

func (m *ProjectsMethods) handleProjectsUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	p, pErr := m.store.GetProject(context.Background(), id)
	if pErr != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canModify(p, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "only project owner can update"))
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
	if err := m.store.UpdateProject(context.Background(), id, params.Updates); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ProjectsMethods) handleProjectsDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	p, pErr := m.store.GetProject(context.Background(), id)
	if pErr != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canModify(p, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "only project owner can delete"))
		return
	}
	if err := m.store.DeleteProject(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

// --- Sessions ---

func (m *ProjectsMethods) handleSessionsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	proj, pErr := m.store.GetProject(context.Background(), projectID)
	if pErr != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canAccess(context.Background(), proj, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
		return
	}

	if params.Limit <= 0 {
		params.Limit = 50
	}
	sessions, total, err := m.store.ListSessions(context.Background(), projectID, params.Limit, params.Offset)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if sessions == nil {
		sessions = []store.ProjectSessionData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"sessions": sessions, "total": total}))
}

func (m *ProjectsMethods) handleSessionsStart(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil || m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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

	proj, err := m.store.GetProject(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "project not found"))
		return
	}
	if !m.canAccess(context.Background(), proj, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
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

	sess, _ := m.store.GetSession(context.Background(), sessionID)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"session": sess}))
}

func (m *ProjectsMethods) handleSessionsGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	sess, err := m.store.GetSession(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if proj, pErr := m.store.GetProject(context.Background(), sess.ProjectID); pErr == nil {
		if !m.canAccess(context.Background(), proj, client.UserID()) {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
			return
		}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"session": sess}))
}

func (m *ProjectsMethods) handleSessionsPrompt(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	if sess, sErr := m.store.GetSession(context.Background(), id); sErr == nil {
		if proj, pErr := m.store.GetProject(context.Background(), sess.ProjectID); pErr == nil {
			if !m.canAccess(context.Background(), proj, client.UserID()) {
				client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
				return
			}
		}
	}

	if err := m.manager.SendPrompt(context.Background(), id, params.Prompt); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ProjectsMethods) handleSessionsStop(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	if sess, sErr := m.store.GetSession(context.Background(), id); sErr == nil {
		if proj, pErr := m.store.GetProject(context.Background(), sess.ProjectID); pErr == nil {
			if !m.canAccess(context.Background(), proj, client.UserID()) {
				client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
				return
			}
		}
	}

	if err := m.manager.Stop(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "stopped"}))
}

func (m *ProjectsMethods) handleSessionsDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.manager == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	if sess, sErr := m.store.GetSession(context.Background(), id); sErr == nil {
		if proj, pErr := m.store.GetProject(context.Background(), sess.ProjectID); pErr == nil {
			if !m.canAccess(context.Background(), proj, client.UserID()) {
				client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
				return
			}
		}
	}

	if err := m.manager.Delete(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

func (m *ProjectsMethods) handleSessionsUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var params struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" || params.Label == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id and label required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	if err := m.store.UpdateSession(context.Background(), id, map[string]any{"label": params.Label}); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ProjectsMethods) handleSessionsLogs(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
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
	if sess, sErr := m.store.GetSession(context.Background(), sessionID); sErr == nil {
		if proj, pErr := m.store.GetProject(context.Background(), sess.ProjectID); pErr == nil {
			if !m.canAccess(context.Background(), proj, client.UserID()) {
				client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
				return
			}
		}
	}

	if params.Limit <= 0 {
		params.Limit = 500
	}
	logs, err := m.store.GetLogs(context.Background(), sessionID, params.AfterSeq, params.Limit)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if logs == nil {
		logs = []store.ProjectSessionLogData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"logs": logs}))
}

// --- Members ---

func (m *ProjectsMethods) handleMembersList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var params struct {
		ProjectID string `json:"project_id"`
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
	proj, err := m.store.GetProject(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canAccess(context.Background(), proj, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "access denied"))
		return
	}
	members, err := m.store.ListMembers(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if members == nil {
		members = []store.ProjectMemberData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"members": members}))
}

func (m *ProjectsMethods) handleMembersAdd(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var params struct {
		ProjectID string `json:"project_id"`
		UserID    string `json:"user_id"`
		Role      string `json:"role"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ProjectID == "" || params.UserID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "project_id and user_id required"))
		return
	}
	projectID, err := uuid.Parse(params.ProjectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid project_id"))
		return
	}
	proj, err := m.store.GetProject(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canModify(proj, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "only project owner can add members"))
		return
	}
	role := params.Role
	if role == "" {
		role = "member"
	}
	if err := m.store.AddMember(context.Background(), projectID, params.UserID, role, client.UserID()); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *ProjectsMethods) handleMembersRemove(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	if m.store == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "projects not available"))
		return
	}
	var params struct {
		ProjectID string `json:"project_id"`
		UserID    string `json:"user_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ProjectID == "" || params.UserID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "project_id and user_id required"))
		return
	}
	projectID, err := uuid.Parse(params.ProjectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid project_id"))
		return
	}
	proj, err := m.store.GetProject(context.Background(), projectID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "project not found"))
		return
	}
	if !m.canModify(proj, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "only project owner can remove members"))
		return
	}
	if err := m.store.RemoveMember(context.Background(), projectID, params.UserID); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}
