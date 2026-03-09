package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NewsHandler handles news management HTTP endpoints.
type NewsHandler struct {
	news  store.NewsStore
	token string
}

func NewNewsHandler(news store.NewsStore, token string) *NewsHandler {
	return &NewsHandler{news: news, token: token}
}

func (h *NewsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/news/sources", h.auth(h.handleSourcesList))
	mux.HandleFunc("POST /v1/news/sources", h.auth(h.handleSourcesCreate))
	mux.HandleFunc("PUT /v1/news/sources/{id}", h.auth(h.handleSourcesUpdate))
	mux.HandleFunc("DELETE /v1/news/sources/{id}", h.auth(h.handleSourcesDelete))
	mux.HandleFunc("GET /v1/news/items", h.auth(h.handleItemsList))
	mux.HandleFunc("GET /v1/news/items/{id}", h.auth(h.handleItemsGet))
}

func (h *NewsHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" && extractBearerToken(r) != h.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (h *NewsHandler) handleSourcesList(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(r.URL.Query().Get("agentId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid agentId query param required"})
		return
	}

	enabledOnly := r.URL.Query().Get("enabledOnly") != "false"
	sources, err := h.news.ListSources(r.Context(), agentID, enabledOnly)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"sources": sources})
}

func (h *NewsHandler) handleSourcesCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentID        string          `json:"agentId"`
		Name           string          `json:"name"`
		SourceType     string          `json:"sourceType"`
		Config         json.RawMessage `json:"config"`
		ScrapeInterval string          `json:"scrapeInterval"`
		Category       string          `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	agentID, err := uuid.Parse(body.AgentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid agentId required"})
		return
	}
	if body.Name == "" || body.SourceType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and sourceType required"})
		return
	}

	src := &store.NewsSource{
		AgentID:        agentID,
		Name:           body.Name,
		SourceType:     body.SourceType,
		Config:         body.Config,
		Enabled:        true,
		ScrapeInterval: body.ScrapeInterval,
	}
	if body.Category != "" {
		src.Category = &body.Category
	}
	if err := h.news.CreateSource(r.Context(), src); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, src)
}

func (h *NewsHandler) handleSourcesUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid source id required"})
		return
	}

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := h.news.UpdateSource(r.Context(), id, patch); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *NewsHandler) handleSourcesDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid source id required"})
		return
	}

	if err := h.news.DeleteSource(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *NewsHandler) handleItemsList(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(r.URL.Query().Get("agentId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid agentId query param required"})
		return
	}

	filter := store.NewsItemFilter{
		AgentID: agentID,
		Limit:   50,
	}

	if srcID := r.URL.Query().Get("sourceId"); srcID != "" {
		if u, err := uuid.Parse(srcID); err == nil {
			filter.SourceID = &u
		}
	}

	items, err := h.news.ListItems(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items, "count": len(items)})
}

func (h *NewsHandler) handleItemsGet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid item id required"})
		return
	}

	item, err := h.news.GetItem(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	writeJSON(w, http.StatusOK, item)
}
