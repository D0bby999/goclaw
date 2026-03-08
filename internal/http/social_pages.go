package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// SocialPagesHandler handles social page CRUD endpoints.
type SocialPagesHandler struct {
	store   store.SocialStore
	manager *social.Manager
	oauth   *SocialOAuthHandler
	token   string
}

func NewSocialPagesHandler(st store.SocialStore, mgr *social.Manager, oauth *SocialOAuthHandler, token string) *SocialPagesHandler {
	return &SocialPagesHandler{store: st, manager: mgr, oauth: oauth, token: token}
}

func (h *SocialPagesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/social/accounts/{id}/pages", h.auth(h.handleListPages))
	mux.HandleFunc("POST /v1/social/accounts/{id}/pages", h.auth(h.handleCreatePage))
	mux.HandleFunc("POST /v1/social/accounts/{id}/pages/sync", h.auth(h.handleSyncPages))
	mux.HandleFunc("PUT /v1/social/pages/{id}/default", h.auth(h.handleSetDefault))
	mux.HandleFunc("DELETE /v1/social/pages/{id}", h.auth(h.handleDeletePage))
}

func (h *SocialPagesHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" {
			if !tokenMatch(extractBearerToken(r), h.token) {
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

func (h *SocialPagesHandler) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}
	account, err := h.store.GetAccount(r.Context(), accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if account.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		PageID    string `json:"page_id"`
		PageName  string `json:"page_name"`
		PageToken string `json:"page_token"`
		PageType  string `json:"page_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if body.PageID == "" || body.PageToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "page_id and page_token required"})
		return
	}

	p := &store.SocialPageData{
		AccountID: accountID,
		PageID:    body.PageID,
		PageToken: body.PageToken,
		PageType:  body.PageType,
		Status:    "active",
	}
	if body.PageName != "" {
		p.PageName = &body.PageName
	}
	if p.PageType == "" {
		p.PageType = "page"
	}

	if err := h.store.CreatePage(r.Context(), p); err != nil {
		slog.Error("social.create_page", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create page"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"page": p})
}

func (h *SocialPagesHandler) handleListPages(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}
	// Verify ownership.
	account, err := h.store.GetAccount(r.Context(), accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if account.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	pages, err := h.store.ListPages(r.Context(), accountID)
	if err != nil {
		slog.Error("social.list_pages", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pages"})
		return
	}
	if pages == nil {
		pages = []store.SocialPageData{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages, "count": len(pages)})
}

func (h *SocialPagesHandler) handleSyncPages(w http.ResponseWriter, r *http.Request) {
	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}
	account, err := h.store.GetAccount(r.Context(), accountID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
		return
	}
	if account.OwnerID != store.UserIDFromContext(r.Context()) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	// Only Facebook and Instagram have pages to sync.
	switch account.Platform {
	case store.PlatformFacebook:
		if h.oauth == nil || h.oauth.meta == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "facebook oauth not configured"})
			return
		}
		pages, err := h.oauth.fetchFacebookPages(r, account.AccessToken)
		if err != nil {
			slog.Error("social.fetch_facebook_pages", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch pages"})
			return
		}
		if err := storeFacebookPages(r.Context(), h.store, accountID, pages); err != nil {
			slog.Error("social.store_facebook_pages", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store pages"})
			return
		}
		synced, _ := h.store.ListPages(r.Context(), accountID)
		if synced == nil {
			synced = []store.SocialPageData{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"pages": synced, "count": len(synced)})

	case store.PlatformInstagram:
		if h.oauth == nil || h.oauth.meta == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instagram oauth not configured"})
			return
		}
		if err := syncInstagramPages(r, h.oauth, h.store, account); err != nil {
			slog.Error("social.sync_instagram_pages", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to sync pages"})
			return
		}
		synced, _ := h.store.ListPages(r.Context(), accountID)
		if synced == nil {
			synced = []store.SocialPageData{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"pages": synced, "count": len(synced)})

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "page sync not supported for " + account.Platform})
	}
}

func (h *SocialPagesHandler) handleSetDefault(w http.ResponseWriter, r *http.Request) {
	pageID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid page id"})
		return
	}

	// Get pages to find the account and verify ownership.
	// We need to find which account this page belongs to.
	// Query the page directly via a small helper.
	pages, accountID, err := h.findPageAccount(r, pageID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		return
	}
	_ = pages

	if err := h.store.SetDefaultPage(r.Context(), accountID, pageID); err != nil {
		slog.Error("social.set_default_page", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set default page"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SocialPagesHandler) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	pageID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid page id"})
		return
	}

	if _, _, err := h.findPageAccount(r, pageID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		return
	}

	if err := h.store.DeletePage(r.Context(), pageID); err != nil {
		slog.Error("social.delete_page", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete page"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// findPageAccount locates a page's parent account and verifies ownership.
func (h *SocialPagesHandler) findPageAccount(r *http.Request, pageID uuid.UUID) ([]store.SocialPageData, uuid.UUID, error) {
	pg, err := h.store.GetPage(r.Context(), pageID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("page not found")
	}
	account, err := h.store.GetAccount(r.Context(), pg.AccountID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("page not found")
	}
	userID := store.UserIDFromContext(r.Context())
	if account.OwnerID != userID {
		return nil, uuid.Nil, fmt.Errorf("page not found")
	}
	return nil, pg.AccountID, nil
}

// storeFacebookPages creates social_pages rows from fetched Facebook pages.
// The first page is marked as default if no existing default exists.
func storeFacebookPages(ctx context.Context, st store.SocialStore, accountID uuid.UUID, pages []fbPage) error {
	existing, _ := st.ListPages(ctx, accountID)
	hasDefault := false
	for _, p := range existing {
		if p.IsDefault {
			hasDefault = true
			break
		}
	}

	for i, fp := range pages {
		name := fp.Name
		p := &store.SocialPageData{
			AccountID: accountID,
			PageID:    fp.ID,
			PageName:  &name,
			PageToken: fp.Token,
			PageType:  "page",
			IsDefault: !hasDefault && i == 0,
			Status:    "active",
		}
		if err := st.CreatePage(ctx, p); err != nil {
			return fmt.Errorf("create page %s: %w", fp.ID, err)
		}
	}
	return nil
}

// syncInstagramPages fetches Facebook pages with IG business accounts and stores them.
func syncInstagramPages(r *http.Request, oauth *SocialOAuthHandler, st store.SocialStore, account *store.SocialAccountData) error {
	apiURL := fmt.Sprintf("%s/me/accounts?fields=id,name,access_token,instagram_business_account{id,username,name,profile_picture_url}&access_token=%s",
		fbGraphBase(), account.AccessToken)
	var resp struct {
		Data []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Token string `json:"access_token"`
			IG    *struct {
				ID      string `json:"id"`
				User    string `json:"username"`
				Name    string `json:"name"`
				Picture string `json:"profile_picture_url"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return err
	}

	existing, _ := st.ListPages(r.Context(), account.ID)
	hasDefault := false
	for _, p := range existing {
		if p.IsDefault {
			hasDefault = true
			break
		}
	}

	for i, page := range resp.Data {
		if page.IG == nil {
			continue
		}
		name := page.IG.Name
		if name == "" {
			name = page.Name
		}
		avatar := page.IG.Picture
		meta, _ := json.Marshal(map[string]string{
			"ig_user_id":    page.IG.ID,
			"ig_username":   page.IG.User,
			"fb_page_id":    page.ID,
			"fb_page_name":  page.Name,
		})
		p := &store.SocialPageData{
			AccountID: account.ID,
			PageID:    page.IG.ID,
			PageName:  &name,
			PageToken: page.Token,
			PageType:  "business",
			AvatarURL: &avatar,
			IsDefault: !hasDefault && i == 0,
			Metadata:  meta,
			Status:    "active",
		}
		if err := st.CreatePage(r.Context(), p); err != nil {
			return fmt.Errorf("create ig page %s: %w", page.IG.ID, err)
		}
	}
	return nil
}
