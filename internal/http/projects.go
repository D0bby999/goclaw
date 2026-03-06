package http

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/claudecode"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ProjectsHandler handles project and session endpoints.
type ProjectsHandler struct {
	store     store.ProjectStore
	manager   *claudecode.ProcessManager
	token     string
	msgBus    *bus.MessageBus
	isOwner   func(string) bool
	teamStore store.TeamStore
}

func NewProjectsHandler(store store.ProjectStore, manager *claudecode.ProcessManager, token string, msgBus *bus.MessageBus, isOwner func(string) bool, teamStore store.TeamStore) *ProjectsHandler {
	return &ProjectsHandler{store: store, manager: manager, token: token, msgBus: msgBus, isOwner: isOwner, teamStore: teamStore}
}

func (h *ProjectsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/projects", h.auth(h.handleListProjects))
	mux.HandleFunc("POST /v1/projects", h.auth(h.handleCreateProject))
	mux.HandleFunc("GET /v1/projects/{id}", h.auth(h.handleGetProject))
	mux.HandleFunc("PUT /v1/projects/{id}", h.auth(h.handleUpdateProject))
	mux.HandleFunc("DELETE /v1/projects/{id}", h.auth(h.handleDeleteProject))
	mux.HandleFunc("GET /v1/projects/{id}/sessions", h.auth(h.handleListSessions))
	mux.HandleFunc("POST /v1/projects/{id}/sessions", h.auth(h.handleStartSession))
	mux.HandleFunc("GET /v1/project-sessions/{id}", h.auth(h.handleGetSession))
	mux.HandleFunc("PATCH /v1/project-sessions/{id}", h.auth(h.handleUpdateSession))
	mux.HandleFunc("DELETE /v1/project-sessions/{id}", h.auth(h.handleDeleteSession))
	mux.HandleFunc("POST /v1/project-sessions/{id}/prompt", h.auth(h.handleSendPrompt))
	mux.HandleFunc("POST /v1/project-sessions/{id}/stop", h.auth(h.handleStopSession))
	mux.HandleFunc("GET /v1/project-sessions/{id}/logs", h.auth(h.handleGetLogs))
	// Member management
	mux.HandleFunc("GET /v1/projects/{id}/members", h.auth(h.handleListMembers))
	mux.HandleFunc("POST /v1/projects/{id}/members", h.auth(h.handleAddMember))
	mux.HandleFunc("DELETE /v1/projects/{id}/members/{user_id}", h.auth(h.handleRemoveMember))
}

func (h *ProjectsHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" {
			if extractBearerToken(r) != h.token {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		userID := extractUserID(r)
		if userID != "" {
			ctx := store.WithUserID(r.Context(), userID)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

// canAccess checks if a user can access a project (read).
func (h *ProjectsHandler) canAccess(r *http.Request, project *store.ProjectData) bool {
	userID := store.UserIDFromContext(r.Context())
	// 1. System owner
	if h.isOwner(userID) {
		return true
	}
	// 2. Project owner
	if project.OwnerID == userID {
		return true
	}
	// 3. Explicit member
	if ok, _ := h.store.IsMember(r.Context(), project.ID, userID); ok {
		return true
	}
	// 4. Team-linked access
	if project.TeamID != nil && h.teamStore != nil {
		team, err := h.teamStore.GetTeam(r.Context(), *project.TeamID)
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
func (h *ProjectsHandler) canModify(r *http.Request, project *store.ProjectData) bool {
	userID := store.UserIDFromContext(r.Context())
	return h.isOwner(userID) || project.OwnerID == userID
}

// --- Projects ---

func (h *ProjectsHandler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())

	var projects []store.ProjectData
	var err error
	if h.isOwner(userID) {
		// System owner sees all active projects
		projects, err = h.store.ListProjects(r.Context(), "")
	} else if userID != "" {
		projects, err = h.store.ListAccessibleProjects(r.Context(), userID)
	} else {
		projects = []store.ProjectData{}
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if projects == nil {
		projects = []store.ProjectData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects, "count": len(projects)})
}

func (h *ProjectsHandler) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-GoClaw-User-Id header required"})
		return
	}

	var p store.ProjectData
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if p.Name == "" || p.Slug == "" || p.WorkDir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, slug, and work_dir are required"})
		return
	}
	if err := claudecode.ValidateSlug(p.Slug); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := claudecode.ValidateWorkDir(p.WorkDir); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p.OwnerID = userID

	if err := h.store.CreateProject(r.Context(), &p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.emitCacheInvalidate()
	writeJSON(w, http.StatusCreated, map[string]any{"project": p})
}

func (h *ProjectsHandler) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canAccess(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p})
}

func (h *ProjectsHandler) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	p, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canModify(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only project owner can update"})
		return
	}

	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Prevent updating immutable fields
	delete(updates, "id")
	delete(updates, "owner_id")
	delete(updates, "created_at")

	// Validate work_dir if being changed
	if wd, ok := updates["work_dir"].(string); ok && wd != "" {
		if err := claudecode.ValidateWorkDir(wd); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}
	// Validate slug if being changed
	if slug, ok := updates["slug"].(string); ok && slug != "" {
		if err := claudecode.ValidateSlug(slug); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	if err := h.store.UpdateProject(r.Context(), id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.emitCacheInvalidate()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProjectsHandler) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	p, err := h.store.GetProject(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canModify(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only project owner can delete"})
		return
	}

	if err := h.store.DeleteProject(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.emitCacheInvalidate()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Sessions ---

func (h *ProjectsHandler) handleListSessions(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	p, err := h.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canAccess(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	sessions, total, err := h.store.ListSessions(r.Context(), projectID, 50, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if sessions == nil {
		sessions = []store.ProjectSessionData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "total": total})
}

type startSessionReq struct {
	Prompt       string   `json:"prompt"`
	Label        string   `json:"label"`
	AllowedTools []string `json:"allowed_tools"`
	Model        string   `json:"model"`
	MaxTurns     int      `json:"max_turns"`
}

func (h *ProjectsHandler) handleStartSession(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}
	userID := store.UserIDFromContext(r.Context())

	var req startSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	proj, err := h.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canAccess(r, proj) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	// Parse allowed tools from project if not overridden
	allowedTools := req.AllowedTools
	if len(allowedTools) == 0 && proj.AllowedTools != nil {
		_ = json.Unmarshal(proj.AllowedTools, &allowedTools)
	}

	opts := claudecode.StartOpts{
		ProjectID:    projectID,
		WorkDir:      proj.WorkDir,
		Prompt:       req.Prompt,
		AllowedTools: allowedTools,
		Model:        req.Model,
		MaxTurns:     req.MaxTurns,
	}

	sessionID, err := h.manager.Start(r.Context(), opts, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if err == claudecode.ErrMaxSessionsReached {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	sess, _ := h.store.GetSession(r.Context(), sessionID)
	writeJSON(w, http.StatusCreated, map[string]any{"session": sess})
}

func (h *ProjectsHandler) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	if proj, pErr := h.store.GetProject(r.Context(), sess.ProjectID); pErr == nil {
		if !h.canAccess(r, proj) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}
	sess.ProjectName = "" // re-fetch joined if needed
	writeJSON(w, http.StatusOK, map[string]any{"session": sess})
}

func (h *ProjectsHandler) handleUpdateSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Label == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "label is required"})
		return
	}
	if err := h.store.UpdateSession(r.Context(), id, map[string]any{"label": body.Label}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProjectsHandler) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.manager.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ProjectsHandler) handleSendPrompt(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if sess, sErr := h.store.GetSession(r.Context(), id); sErr == nil {
		if proj, pErr := h.store.GetProject(r.Context(), sess.ProjectID); pErr == nil {
			if !h.canAccess(r, proj) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}
		}
	}

	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	if err := h.manager.SendPrompt(r.Context(), id, body.Prompt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProjectsHandler) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if sess, sErr := h.store.GetSession(r.Context(), id); sErr == nil {
		if proj, pErr := h.store.GetProject(r.Context(), sess.ProjectID); pErr == nil {
			if !h.canAccess(r, proj) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}
		}
	}

	if err := h.manager.Stop(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *ProjectsHandler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if sess, sErr := h.store.GetSession(r.Context(), id); sErr == nil {
		if proj, pErr := h.store.GetProject(r.Context(), sess.ProjectID); pErr == nil {
			if !h.canAccess(r, proj) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}
		}
	}

	logs, err := h.store.GetLogs(r.Context(), id, -1, 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if logs == nil {
		logs = []store.ProjectSessionLogData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

// --- Members ---

func (h *ProjectsHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canAccess(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}
	members, err := h.store.ListMembers(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if members == nil {
		members = []store.ProjectMemberData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *ProjectsHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canModify(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only project owner can add members"})
		return
	}

	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	if body.Role == "" {
		body.Role = "member"
	}

	addedBy := store.UserIDFromContext(r.Context())
	if err := h.store.AddMember(r.Context(), projectID, body.UserID, body.Role, addedBy); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (h *ProjectsHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetProject(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if !h.canModify(r, p) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only project owner can remove members"})
		return
	}

	memberUserID := r.PathValue("user_id")
	if memberUserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	if err := h.store.RemoveMember(r.Context(), projectID, memberUserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ProjectsHandler) emitCacheInvalidate() {
	if h.msgBus == nil {
		return
	}
	h.msgBus.Broadcast(bus.Event{
		Name:    protocol.EventCacheInvalidate,
		Payload: bus.CacheInvalidatePayload{Kind: "projects"},
	})
}
