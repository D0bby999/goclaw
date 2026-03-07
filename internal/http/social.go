package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// SocialHandler handles social account and post endpoints.
type SocialHandler struct {
	store   store.SocialStore
	manager *social.Manager
	token   string
}

func NewSocialHandler(store store.SocialStore, manager *social.Manager, token string) *SocialHandler {
	return &SocialHandler{store: store, manager: manager, token: token}
}

func (h *SocialHandler) RegisterRoutes(mux *http.ServeMux) {
	// Accounts
	mux.HandleFunc("GET /v1/social/accounts", h.auth(h.handleListAccounts))
	mux.HandleFunc("POST /v1/social/accounts", h.auth(h.handleCreateAccount))
	mux.HandleFunc("GET /v1/social/accounts/{id}", h.auth(h.handleGetAccount))
	mux.HandleFunc("PUT /v1/social/accounts/{id}", h.auth(h.handleUpdateAccount))
	mux.HandleFunc("DELETE /v1/social/accounts/{id}", h.auth(h.handleDeleteAccount))
	// Posts
	mux.HandleFunc("GET /v1/social/posts", h.auth(h.handleListPosts))
	mux.HandleFunc("POST /v1/social/posts", h.auth(h.handleCreatePost))
	mux.HandleFunc("GET /v1/social/posts/{id}", h.auth(h.handleGetPost))
	mux.HandleFunc("PUT /v1/social/posts/{id}", h.auth(h.handleUpdatePost))
	mux.HandleFunc("DELETE /v1/social/posts/{id}", h.auth(h.handleDeletePost))
	mux.HandleFunc("POST /v1/social/posts/{id}/publish", h.auth(h.handlePublishPost))
	// Targets + Media
	mux.HandleFunc("POST /v1/social/posts/{id}/targets", h.auth(h.handleAddTarget))
	mux.HandleFunc("DELETE /v1/social/posts/{id}/targets/{targetId}", h.auth(h.handleRemoveTarget))
	mux.HandleFunc("POST /v1/social/posts/{id}/media", h.auth(h.handleAddMedia))
	mux.HandleFunc("DELETE /v1/social/posts/{id}/media/{mediaId}", h.auth(h.handleRemoveMedia))
	// Utilities
	mux.HandleFunc("POST /v1/social/adapt", h.auth(h.handleAdaptContent))
	mux.HandleFunc("GET /v1/social/platforms", h.auth(h.handleListPlatforms))
}

func (h *SocialHandler) auth(next http.HandlerFunc) http.HandlerFunc {
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

// --- Accounts ---

func (h *SocialHandler) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	ownerID := store.UserIDFromContext(r.Context())
	platform := r.URL.Query().Get("platform")

	var accounts []store.SocialAccountData
	var err error
	if platform != "" {
		accounts, err = h.store.ListAccountsByPlatform(r.Context(), ownerID, platform)
	} else {
		accounts, err = h.store.ListAccounts(r.Context(), ownerID)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if accounts == nil {
		accounts = []store.SocialAccountData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts, "count": len(accounts)})
}

func (h *SocialHandler) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	var a store.SocialAccountData
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if a.Platform == "" || a.PlatformUserID == "" || a.AccessToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "platform, platform_user_id, access_token required"})
		return
	}
	a.OwnerID = userID
	if err := h.store.CreateAccount(r.Context(), &a); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.AccessToken = "" // never expose token in response
	a.RefreshToken = nil
	writeJSON(w, http.StatusCreated, map[string]any{"account": a})
}

func (h *SocialHandler) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	a, err := h.store.GetAccount(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if a.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	a.AccessToken = ""
	a.RefreshToken = nil
	writeJSON(w, http.StatusOK, map[string]any{"account": a})
}

func (h *SocialHandler) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	a, err := h.store.GetAccount(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if a.OwnerID != store.UserIDFromContext(r.Context()) {
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
	if err := h.store.UpdateAccount(r.Context(), id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SocialHandler) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	a, err := h.store.GetAccount(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if a.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := h.store.DeleteAccount(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Utilities ---

func (h *SocialHandler) handleAdaptContent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content   string   `json:"content"`
		Platforms []string `json:"platforms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}
	results := make(map[string]any)
	for _, p := range body.Platforms {
		adapted, warnings := social.AdaptContent(body.Content, p)
		results[p] = map[string]any{"adapted": adapted, "warnings": warnings}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (h *SocialHandler) handleListPlatforms(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"platforms": social.PlatformLimits})
}

// parseIntQuery parses an integer query param with a fallback default.
func parseIntQuery(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
