package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// --- Posts ---

func (h *SocialHandler) handleListPosts(w http.ResponseWriter, r *http.Request) {
	ownerID := store.UserIDFromContext(r.Context())
	status := r.URL.Query().Get("status")
	limit := parseIntQuery(r, "limit", 50)
	offset := parseIntQuery(r, "offset", 0)

	posts, total, err := h.store.ListPosts(r.Context(), ownerID, status, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if posts == nil {
		posts = []store.SocialPostData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts, "total": total})
}

func (h *SocialHandler) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	var p store.SocialPostData
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if p.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}
	p.OwnerID = userID
	if err := h.store.CreatePost(r.Context(), &p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"post": p})
}

func (h *SocialHandler) handleGetPost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": p})
}

func (h *SocialHandler) handleUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
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
	if err := h.store.UpdatePost(r.Context(), id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SocialHandler) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := h.store.DeletePost(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *SocialHandler) handlePublishPost(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := h.manager.PublishPost(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	p, _ = h.store.GetPost(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{"post": p})
}

// --- Targets ---

func (h *SocialHandler) handleAddTarget(w http.ResponseWriter, r *http.Request) {
	postID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid post id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), postID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var t store.SocialPostTargetData
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if t.AccountID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id required"})
		return
	}
	t.PostID = postID
	if err := h.store.AddTarget(r.Context(), &t); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"target": t})
}

func (h *SocialHandler) handleRemoveTarget(w http.ResponseWriter, r *http.Request) {
	postID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid post id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), postID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	targetID, err := uuid.Parse(r.PathValue("targetId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid target id"})
		return
	}
	if err := h.store.RemoveTarget(r.Context(), targetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Media ---

func (h *SocialHandler) handleAddMedia(w http.ResponseWriter, r *http.Request) {
	postID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid post id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), postID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var m store.SocialPostMediaData
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if m.URL == "" || m.MediaType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url and media_type required"})
		return
	}
	m.PostID = postID
	if err := h.store.AddMedia(r.Context(), &m); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"media": m})
}

func (h *SocialHandler) handleRemoveMedia(w http.ResponseWriter, r *http.Request) {
	postID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid post id"})
		return
	}
	p, err := h.store.GetPost(r.Context(), postID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "post not found"})
		return
	}
	if p.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid media id"})
		return
	}
	if err := h.store.RemoveMedia(r.Context(), mediaID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
