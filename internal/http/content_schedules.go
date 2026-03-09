package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ContentScheduleHandler handles /v1/schedules HTTP endpoints.
type ContentScheduleHandler struct {
	store   store.ContentScheduleStore
	cronSvc store.CronStore
	token   string
}

func NewContentScheduleHandler(s store.ContentScheduleStore, cron store.CronStore, token string) *ContentScheduleHandler {
	return &ContentScheduleHandler{store: s, cronSvc: cron, token: token}
}

func (h *ContentScheduleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/schedules", h.auth(h.handleList))
	mux.HandleFunc("POST /v1/schedules", h.auth(h.handleCreate))
	mux.HandleFunc("GET /v1/schedules/{id}", h.auth(h.handleGet))
	mux.HandleFunc("PUT /v1/schedules/{id}", h.auth(h.handleUpdate))
	mux.HandleFunc("DELETE /v1/schedules/{id}", h.auth(h.handleDelete))
	mux.HandleFunc("POST /v1/schedules/{id}/toggle", h.auth(h.handleToggle))
	mux.HandleFunc("GET /v1/schedules/{id}/logs", h.auth(h.handleLogs))
}

func (h *ContentScheduleHandler) auth(next http.HandlerFunc) http.HandlerFunc {
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

func (h *ContentScheduleHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ownerID := store.UserIDFromContext(r.Context())
	var enabled *bool
	if v := r.URL.Query().Get("enabled"); v != "" {
		b := v == "true"
		enabled = &b
	}

	schedules, err := h.store.List(r.Context(), ownerID, enabled)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if schedules == nil {
		schedules = []store.ContentScheduleData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": schedules, "count": len(schedules)})
}

func (h *ContentScheduleHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	var body struct {
		Name           string      `json:"name"`
		CronExpression string      `json:"cron_expression"`
		Timezone       string      `json:"timezone"`
		ContentSource  string      `json:"content_source"`
		AgentID        *uuid.UUID  `json:"agent_id"`
		Prompt         *string     `json:"prompt"`
		PageIDs        []uuid.UUID `json:"page_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if body.CronExpression == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cron_expression required"})
		return
	}
	tz := body.Timezone
	if tz == "" {
		tz = "UTC"
	}

	data := store.ContentScheduleData{
		OwnerID:        userID,
		Name:           body.Name,
		Enabled:        true,
		ContentSource:  body.ContentSource,
		AgentID:        body.AgentID,
		Prompt:         body.Prompt,
		CronExpression: body.CronExpression,
		Timezone:       tz,
	}
	if err := h.store.Create(r.Context(), &data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	jobName := "sched-" + data.ID.String()[:8]
	msg := "{internal:content_schedule:" + data.ID.String() + "}"
	sched := store.CronSchedule{Kind: "cron", Expr: body.CronExpression, TZ: tz}
	if job, err := h.cronSvc.AddJob(jobName, sched, msg, false, "", "", "", ""); err == nil {
		_ = h.store.Update(r.Context(), data.ID, map[string]any{"cron_job_id": job.ID})
		data.CronJobID = &job.ID
	}

	if len(body.PageIDs) > 0 {
		_ = h.store.SetPages(r.Context(), data.ID, body.PageIDs)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"schedule": data})
}

func (h *ContentScheduleHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	s, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
		return
	}
	if s.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedule": s})
}

func (h *ContentScheduleHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	existing, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
		return
	}
	if existing.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	delete(updates, "id")
	delete(updates, "owner_id")
	delete(updates, "created_at")
	delete(updates, "cron_job_id")

	if err := h.store.Update(r.Context(), id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Sync cron job schedule if expression/timezone changed
	_, exprChanged := updates["cron_expression"]
	_, tzChanged := updates["timezone"]
	if (exprChanged || tzChanged) && existing.CronJobID != nil {
		expr := existing.CronExpression
		tz := existing.Timezone
		if v, ok := updates["cron_expression"].(string); ok {
			expr = v
		}
		if v, ok := updates["timezone"].(string); ok {
			tz = v
		}
		patch := store.CronJobPatch{
			Schedule: &store.CronSchedule{Kind: "cron", Expr: expr, TZ: tz},
		}
		_, _ = h.cronSvc.UpdateJob(*existing.CronJobID, patch)
	}

	if pageIDs, ok := updates["page_ids"]; ok {
		if ids, ok := toUUIDSlice(pageIDs); ok {
			_ = h.store.SetPages(r.Context(), id, ids)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ContentScheduleHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	existing, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
		return
	}
	if existing.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing.CronJobID != nil {
		_ = h.cronSvc.RemoveJob(*existing.CronJobID)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ContentScheduleHandler) handleToggle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	existing, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
		return
	}
	if existing.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := h.store.Update(r.Context(), id, map[string]any{"enabled": body.Enabled}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing.CronJobID != nil {
		_ = h.cronSvc.EnableJob(*existing.CronJobID, body.Enabled)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "enabled": body.Enabled})
}

func (h *ContentScheduleHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	existing, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "schedule not found"})
		return
	}
	if existing.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	limit := parseIntQuery(r, "limit", 20)
	offset := parseIntQuery(r, "offset", 0)

	logs, total, err := h.store.ListLogs(r.Context(), id, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if logs == nil {
		logs = []store.ContentScheduleLogData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs, "total": total})
}

// toUUIDSlice attempts to convert an any value (typically []any from JSON decode) to []uuid.UUID.
func toUUIDSlice(v any) ([]uuid.UUID, bool) {
	raw, ok := v.([]any)
	if !ok {
		return nil, false
	}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}
