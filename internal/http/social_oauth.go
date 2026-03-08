package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// oauthHTTPClient is a shared HTTP client with a reasonable timeout for OAuth requests.
var oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}

// maxOAuthResponseBody limits the size of OAuth API responses to prevent memory exhaustion.
const maxOAuthResponseBody = 1 << 20 // 1 MB

const (
	// Threads uses its own OAuth domain (version-independent).
	threadsOAuthBase = "https://threads.net/oauth/authorize"
	threadsTokenBase = "https://graph.threads.net/oauth"
)

// fbGraphBase returns the versioned Graph API base URL.
func fbGraphBase() string { return "https://graph.facebook.com/" + social.GraphVersion }

// fbOAuthBase returns the versioned Facebook OAuth dialog URL.
func fbOAuthBase() string { return "https://www.facebook.com/" + social.GraphVersion + "/dialog/oauth" }

// Meta OAuth scopes per platform.
var metaScopes = map[string]string{
	"facebook":  "pages_show_list,pages_read_engagement,pages_manage_posts,public_profile",
	"instagram": "instagram_basic,instagram_content_publish,pages_show_list,pages_read_engagement",
	"threads":   "threads_basic,threads_content_publish,threads_manage_replies",
}

// platformScopes holds scopes for non-Meta platforms (space-separated, except TikTok).
var platformScopes = map[string]string{
	"twitter":  "tweet.read tweet.write users.read offline.access",
	"linkedin": "openid profile w_member_social",
	"youtube":  "https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly",
	"tiktok":   "user.info.basic,video.publish,video.upload",
}

// SocialOAuthHandler handles OAuth flows for social platforms.
type SocialOAuthHandler struct {
	store    store.SocialStore
	manager  *social.Manager
	token    string
	// Meta (Facebook/Instagram/Threads)
	meta *social.OAuthConfig
	// Per-platform configs (nil if not configured)
	twitter  *social.OAuthConfig
	linkedin *social.OAuthConfig
	google   *social.OAuthConfig // YouTube uses Google OAuth
	tiktok   *social.OAuthConfig
}

// PlatformOAuthConfigs holds all per-platform configs for the handler constructor.
type PlatformOAuthConfigs struct {
	Meta     *social.OAuthConfig
	Twitter  *social.OAuthConfig
	LinkedIn *social.OAuthConfig
	Google   *social.OAuthConfig
	TikTok   *social.OAuthConfig
}

// NewSocialOAuthHandler creates a social OAuth handler.
func NewSocialOAuthHandler(st store.SocialStore, mgr *social.Manager, token string, cfgs PlatformOAuthConfigs) *SocialOAuthHandler {
	return &SocialOAuthHandler{
		store:    st,
		manager:  mgr,
		token:    token,
		meta:     cfgs.Meta,
		twitter:  cfgs.Twitter,
		linkedin: cfgs.LinkedIn,
		google:   cfgs.Google,
		tiktok:   cfgs.TikTok,
	}
}

// RegisterRoutes registers social OAuth routes.
func (h *SocialOAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/social/oauth/start", h.auth(h.handleOAuthStart))
	mux.HandleFunc("GET /v1/social/oauth/callback", h.handleOAuthCallback) // no auth — browser redirect
	mux.HandleFunc("GET /v1/social/oauth/status", h.auth(h.handleOAuthStatus))
}

func (h *SocialOAuthHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatch(extractBearerToken(r), h.token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		userID := extractUserID(r)
		if userID != "" {
			ctx := store.WithUserID(r.Context(), userID)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

// handleOAuthStatus returns which platforms have OAuth configured.
func (h *SocialOAuthHandler) handleOAuthStatus(w http.ResponseWriter, _ *http.Request) {
	platforms := map[string]bool{}
	if h.meta != nil && h.meta.ClientID != "" && h.meta.ClientSecret != "" {
		platforms["facebook"] = true
		platforms["instagram"] = true
		platforms["threads"] = true
	}
	if h.twitter != nil && h.twitter.ClientID != "" && h.twitter.ClientSecret != "" {
		platforms["twitter"] = true
	}
	if h.linkedin != nil && h.linkedin.ClientID != "" && h.linkedin.ClientSecret != "" {
		platforms["linkedin"] = true
	}
	if h.google != nil && h.google.ClientID != "" && h.google.ClientSecret != "" {
		platforms["youtube"] = true
	}
	if h.tiktok != nil && h.tiktok.ClientID != "" && h.tiktok.ClientSecret != "" {
		platforms["tiktok"] = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"platforms": platforms,
	})
}

// handleOAuthStart begins the OAuth flow — returns the auth URL.
func (h *SocialOAuthHandler) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	platform := r.URL.Query().Get("platform")
	if platform == "" {
		platform = "facebook"
	}

	ownerID := store.UserIDFromContext(r.Context())
	redirectURI := h.buildRedirectURI(r)

	stateToken, err := generateState()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate state"})
		return
	}

	// Build auth URL + collect extra metadata (e.g. PKCE verifier).
	var authURL string
	var stateMeta json.RawMessage

	switch platform {
	case "facebook", "instagram", "threads":
		if h.meta == nil || h.meta.ClientID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "FACEBOOK_APP_ID not configured"})
			return
		}
		scopes := metaScopes[platform]
		switch platform {
		case "threads":
			authURL = fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s",
				threadsOAuthBase, h.meta.ClientID, url.QueryEscape(redirectURI), url.QueryEscape(scopes), stateToken)
		default:
			authURL = fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&scope=%s&response_type=code&state=%s",
				fbOAuthBase(), h.meta.ClientID, url.QueryEscape(redirectURI), url.QueryEscape(scopes), stateToken)
		}

	case "twitter":
		if h.twitter == nil || h.twitter.ClientID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "TWITTER_CLIENT_ID not configured"})
			return
		}
		authURL, stateMeta, err = buildTwitterAuthURL(h.twitter.ClientID, redirectURI, stateToken)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build Twitter auth URL"})
			return
		}

	case "linkedin":
		if h.linkedin == nil || h.linkedin.ClientID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "LINKEDIN_CLIENT_ID not configured"})
			return
		}
		authURL = buildLinkedInAuthURL(h.linkedin.ClientID, redirectURI, stateToken)

	case "youtube":
		if h.google == nil || h.google.ClientID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "GOOGLE_CLIENT_ID not configured"})
			return
		}
		authURL = buildYouTubeAuthURL(h.google.ClientID, redirectURI, stateToken)

	case "tiktok":
		if h.tiktok == nil || h.tiktok.ClientID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "TIKTOK_CLIENT_KEY not configured"})
			return
		}
		authURL = buildTikTokAuthURL(h.tiktok.ClientID, redirectURI, stateToken)

	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported platform: " + platform})
		return
	}

	// Persist state for CSRF verification (with optional PKCE metadata).
	st := &store.SocialOAuthStateData{
		Platform:    platform,
		State:       stateToken,
		OwnerID:     ownerID,
		RedirectURL: &redirectURI,
		Metadata:    stateMeta,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	if err := h.store.CreateOAuthState(r.Context(), st); err != nil {
		slog.Error("social.oauth: save state", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save state"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"auth_url": authURL,
		"state":    stateToken,
		"platform": platform,
	})
}

// handleOAuthCallback processes the redirect from any OAuth provider.
func (h *SocialOAuthHandler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	if code == "" || stateParam == "" {
		errMsg := r.URL.Query().Get("error_description")
		if errMsg == "" {
			errMsg = "missing code or state"
		}
		h.renderCallbackResult(w, r, false, errMsg, "")
		return
	}

	// Verify CSRF state.
	oauthState, err := h.store.GetOAuthState(r.Context(), stateParam)
	if err != nil {
		slog.Warn("social.oauth: invalid state", "state", stateParam, "error", err)
		h.renderCallbackResult(w, r, false, "invalid or expired state", "")
		return
	}

	// Check expiry.
	if time.Now().After(oauthState.ExpiresAt) {
		_ = h.store.DeleteOAuthState(r.Context(), stateParam)
		h.renderCallbackResult(w, r, false, "state expired, please try again", "")
		return
	}

	// Delete state (one-time use).
	_ = h.store.DeleteOAuthState(r.Context(), stateParam)

	redirectURI := ""
	if oauthState.RedirectURL != nil {
		redirectURI = *oauthState.RedirectURL
	}

	platform := oauthState.Platform

	// Exchange code for access token (platform-specific).
	tokenResp, err := h.dispatchTokenExchange(r, platform, code, redirectURI, oauthState.Metadata)
	if err != nil {
		slog.Error("social.oauth: token exchange", "platform", platform, "error", err)
		h.renderCallbackResult(w, r, false, "token exchange failed", platform)
		return
	}

	// For Facebook/Instagram: exchange short-lived token for long-lived token.
	if platform == "facebook" || platform == "instagram" {
		longLived, err := h.exchangeLongLivedToken(r, tokenResp.AccessToken)
		if err != nil {
			slog.Warn("social.oauth: long-lived token exchange failed, using short-lived", "error", err)
		} else {
			tokenResp = longLived
		}
	}

	// Fetch profile info.
	profile, err := h.dispatchFetchProfile(r, platform, tokenResp)
	if err != nil {
		slog.Warn("social.oauth: fetch profile", "platform", platform, "error", err)
		// Non-fatal — continue with what we have.
	}

	// Build account metadata based on platform.
	metadata, err := h.buildMetadata(r, platform, tokenResp)
	if err != nil {
		slog.Warn("social.oauth: build metadata", "platform", platform, "error", err)
	}

	// For Threads, set threads_user_id from profile.
	if platform == "threads" && profile != nil && profile.ID != "" {
		var m map[string]any
		if json.Unmarshal(metadata, &m) == nil {
			m["threads_user_id"] = profile.ID
			metadata, _ = json.Marshal(m)
		}
	}

	// For TikTok, carry open_id from token response into metadata.
	if platform == "tiktok" && tokenResp.OpenID != "" {
		var m map[string]any
		if json.Unmarshal(metadata, &m) == nil {
			m["open_id"] = tokenResp.OpenID
			metadata, _ = json.Marshal(m)
		}
	}

	// Build scopes list.
	scopes := splitScopes(metaScopes[platform])
	if ps, ok := platformScopes[platform]; ok {
		if platform == "tiktok" {
			scopes = splitComma(ps)
		} else {
			scopes = splitSpaces(ps)
		}
	}

	// Guard against nil profile (fetch may have failed).
	if profile == nil {
		profile = &social.ProfileResult{ID: "unknown"}
	}

	// Create or update account.
	account := &store.SocialAccountData{
		OwnerID:        oauthState.OwnerID,
		Platform:       platform,
		PlatformUserID: profile.ID,
		AccessToken:    tokenResp.AccessToken,
		Status:         store.SocialAccountStatusActive,
		Scopes:         scopes,
		Metadata:       metadata,
	}
	if tokenResp.RefreshToken != "" {
		account.RefreshToken = &tokenResp.RefreshToken
	}
	if profile.Username != "" {
		account.PlatformUsername = &profile.Username
	}
	if profile.Name != "" {
		account.DisplayName = &profile.Name
	}
	if profile.Avatar != "" {
		account.AvatarURL = &profile.Avatar
	}
	if tokenResp.ExpiresAt != nil {
		account.TokenExpiresAt = tokenResp.ExpiresAt
	}

	if err := h.store.CreateAccount(r.Context(), account); err != nil {
		slog.Error("social.oauth: create account", "error", err)
		h.renderCallbackResult(w, r, false, "failed to save account", platform)
		return
	}

	// Store all pages to social_pages table (Facebook/Instagram).
	h.storeAccountPages(r, platform, account.ID, tokenResp.AccessToken)

	displayName := profile.Name
	if displayName == "" {
		displayName = platform
	}

	slog.Info("social.oauth: account connected",
		"platform", platform,
		"user_id", profile.ID,
		"username", profile.Username,
	)

	h.renderCallbackResult(w, r, true, displayName+" connected successfully", platform)
}

// tokenResponse holds the token exchange result.
type tokenResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresIn    int        `json:"expires_in"`
	ExpiresAt    *time.Time `json:"-"`
	OpenID       string     `json:"-"` // TikTok open_id
}

// dispatchTokenExchange routes token exchange to the appropriate platform handler.
func (h *SocialOAuthHandler) dispatchTokenExchange(r *http.Request, platform, code, redirectURI string, stateMeta json.RawMessage) (*tokenResponse, error) {
	switch platform {
	case "twitter":
		var codeVerifier string
		if stateMeta != nil {
			var m map[string]string
			if json.Unmarshal(stateMeta, &m) == nil {
				codeVerifier = m["code_verifier"]
			}
		}
		return exchangeTwitterCode(r.Context(), h.twitter, code, redirectURI, codeVerifier)

	case "linkedin":
		return exchangeLinkedInCode(r.Context(), h.linkedin, code, redirectURI)

	case "youtube":
		return exchangeYouTubeCode(r.Context(), h.google, code, redirectURI)

	case "tiktok":
		return exchangeTikTokCode(r.Context(), h.tiktok, code, redirectURI)

	default: // facebook, instagram, threads
		return h.exchangeMetaCode(r, platform, code, redirectURI)
	}
}

// exchangeMetaCode handles Meta (Facebook/Instagram/Threads) token exchange.
func (h *SocialOAuthHandler) exchangeMetaCode(r *http.Request, platform, code, redirectURI string) (*tokenResponse, error) {
	var tokenURL string
	switch platform {
	case "threads":
		tokenURL = fmt.Sprintf("%s/access_token", threadsTokenBase)
	default:
		tokenURL = fmt.Sprintf("%s/oauth/access_token", fbGraphBase())
	}

	params := url.Values{
		"client_id":     {h.meta.ClientID},
		"client_secret": {h.meta.ClientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	if platform == "threads" {
		params.Set("grant_type", "authorization_code")
	}

	var resp tokenResponse
	apiURL := tokenURL + "?" + params.Encode()
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		resp.ExpiresAt = &t
	}
	return &resp, nil
}

// exchangeLongLivedToken exchanges a short-lived token for a long-lived one (60 days).
func (h *SocialOAuthHandler) exchangeLongLivedToken(r *http.Request, shortToken string) (*tokenResponse, error) {
	apiURL := fmt.Sprintf("%s/oauth/access_token?grant_type=fb_exchange_token&client_id=%s&client_secret=%s&fb_exchange_token=%s",
		fbGraphBase(), h.meta.ClientID, h.meta.ClientSecret, shortToken)

	var resp tokenResponse
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	if resp.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		resp.ExpiresAt = &t
	}
	return &resp, nil
}

// dispatchFetchProfile routes profile fetching to the appropriate platform handler.
func (h *SocialOAuthHandler) dispatchFetchProfile(r *http.Request, platform string, tok *tokenResponse) (*social.ProfileResult, error) {
	switch platform {
	case "threads":
		return h.fetchThreadsProfile(r, tok.AccessToken)
	case "instagram":
		return h.fetchInstagramProfile(r, tok.AccessToken)
	case "twitter":
		return fetchTwitterProfile(r.Context(), tok.AccessToken)
	case "linkedin":
		return fetchLinkedInProfile(r.Context(), tok.AccessToken)
	case "youtube":
		return fetchYouTubeProfile(r.Context(), tok.AccessToken)
	case "tiktok":
		return fetchTikTokProfile(r.Context(), tok.AccessToken)
	default:
		return h.fetchFacebookProfile(r, tok.AccessToken)
	}
}

func (h *SocialOAuthHandler) fetchFacebookProfile(r *http.Request, token string) (*social.ProfileResult, error) {
	apiURL := fmt.Sprintf("%s/me?fields=id,name,picture.width(200)&access_token=%s", fbGraphBase(), token)
	var resp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &social.ProfileResult{ID: resp.ID, Name: resp.Name, Avatar: resp.Picture.Data.URL}, nil
}

func (h *SocialOAuthHandler) fetchInstagramProfile(r *http.Request, token string) (*social.ProfileResult, error) {
	apiURL := fmt.Sprintf("%s/me/accounts?fields=id,name,instagram_business_account{id,username,name,profile_picture_url}&access_token=%s", fbGraphBase(), token)
	var resp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			IG   *struct {
				ID      string `json:"id"`
				User    string `json:"username"`
				Name    string `json:"name"`
				Picture string `json:"profile_picture_url"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	for _, page := range resp.Data {
		if page.IG != nil {
			return &social.ProfileResult{
				ID:       page.IG.ID,
				Username: page.IG.User,
				Name:     page.IG.Name,
				Avatar:   page.IG.Picture,
			}, nil
		}
	}
	return &social.ProfileResult{ID: "unknown"}, fmt.Errorf("no Instagram business account found on any page")
}

func (h *SocialOAuthHandler) fetchThreadsProfile(r *http.Request, token string) (*social.ProfileResult, error) {
	apiURL := fmt.Sprintf("https://graph.threads.net/v1.0/me?fields=id,username,name,threads_profile_picture_url&access_token=%s", token)
	var resp struct {
		ID      string `json:"id"`
		User    string `json:"username"`
		Name    string `json:"name"`
		Picture string `json:"threads_profile_picture_url"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	return &social.ProfileResult{ID: resp.ID, Username: resp.User, Name: resp.Name, Avatar: resp.Picture}, nil
}

// buildMetadata constructs platform-specific metadata for the account.
func (h *SocialOAuthHandler) buildMetadata(r *http.Request, platform string, tok *tokenResponse) (json.RawMessage, error) {
	meta := map[string]any{}

	switch platform {
	case "facebook":
		pages, err := h.fetchFacebookPages(r, tok.AccessToken)
		if err == nil && len(pages) > 0 {
			meta["page_id"] = pages[0].ID
			meta["page_token"] = pages[0].Token
			meta["page_name"] = pages[0].Name
		}
	case "instagram":
		igID, pageToken, err := h.fetchInstagramAccountID(r, tok.AccessToken)
		if err == nil {
			meta["ig_user_id"] = igID
			if pageToken != "" {
				meta["page_token"] = pageToken
			}
		}
	case "threads":
		// threads_user_id set after profile fetch in callback handler.
	}

	b, err := json.Marshal(meta)
	return b, err
}

type fbPage struct {
	ID    string
	Name  string
	Token string
}

func (h *SocialOAuthHandler) fetchFacebookPages(r *http.Request, token string) ([]fbPage, error) {
	apiURL := fmt.Sprintf("%s/me/accounts?fields=id,name,access_token&access_token=%s", fbGraphBase(), token)
	var resp struct {
		Data []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return nil, err
	}
	pages := make([]fbPage, len(resp.Data))
	for i, p := range resp.Data {
		pages[i] = fbPage{ID: p.ID, Name: p.Name, Token: p.Token}
	}
	return pages, nil
}

func (h *SocialOAuthHandler) fetchInstagramAccountID(r *http.Request, token string) (igID, pageToken string, err error) {
	apiURL := fmt.Sprintf("%s/me/accounts?fields=id,access_token,instagram_business_account{id}&access_token=%s", fbGraphBase(), token)
	var resp struct {
		Data []struct {
			ID    string `json:"id"`
			Token string `json:"access_token"`
			IG    *struct {
				ID string `json:"id"`
			} `json:"instagram_business_account"`
		} `json:"data"`
	}
	if err := social.DoGraphJSON(r.Context(), "GET", apiURL, nil, nil, &resp); err != nil {
		return "", "", err
	}
	for _, page := range resp.Data {
		if page.IG != nil {
			return page.IG.ID, page.Token, nil
		}
	}
	return "", "", fmt.Errorf("no Instagram business account found")
}

// buildRedirectURI constructs the callback URL from the request.
func (h *SocialOAuthHandler) buildRedirectURI(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s/v1/social/oauth/callback", scheme, r.Host)
}

// renderCallbackResult renders a small HTML page that posts the result back to the opener window.
func (h *SocialOAuthHandler) renderCallbackResult(w http.ResponseWriter, r *http.Request, success bool, message, platform string) {
	// Derive origin from request for postMessage target (not "*").
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	origin := fmt.Sprintf("%s://%s", scheme, r.Host)

	// HTML-escape message to prevent XSS.
	escaped := html.EscapeString(message)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>OAuth Result</title></head>
<body>
<p>%s</p>
<script>
  var result = {success: %t, message: %q, platform: %q};
  if (window.opener) {
    window.opener.postMessage({type: "social-oauth-result", ...result}, %q);
    window.close();
  } else {
    document.body.innerText = result.success ? 'Connected! You can close this window.' : 'Error: ' + result.message;
  }
</script>
</body></html>`, escaped, success, escaped, platform, origin)
}

// generateState creates a cryptographically random state token.
func generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// splitScopes splits a comma-separated scope string.
func splitScopes(s string) []string {
	var result []string
	for _, scope := range splitComma(s) {
		if scope != "" {
			result = append(result, scope)
		}
	}
	return result
}

// splitSpaces splits a space-separated scope string.
func splitSpaces(s string) []string {
	var result []string
	for _, scope := range splitSpace(s) {
		if scope != "" {
			result = append(result, scope)
		}
	}
	return result
}

func splitComma(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func splitSpace(s string) []string {
	var parts []string
	start := 0
	inWord := false
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' {
			if inWord {
				parts = append(parts, s[start:i])
				inWord = false
			}
		} else {
			if !inWord {
				start = i
				inWord = true
			}
		}
	}
	return parts
}

// storeAccountPages fetches and stores pages for Facebook/Instagram accounts.
func (h *SocialOAuthHandler) storeAccountPages(r *http.Request, platform string, accountID uuid.UUID, accessToken string) {
	switch platform {
	case "facebook":
		pages, err := h.fetchFacebookPages(r, accessToken)
		if err != nil {
			slog.Warn("social.oauth: fetch pages for storage", "platform", platform, "error", err)
			return
		}
		if err := storeFacebookPages(r.Context(), h.store, accountID, pages); err != nil {
			slog.Warn("social.oauth: store pages", "platform", platform, "error", err)
		} else {
			slog.Info("social.oauth: stored pages", "platform", platform, "count", len(pages))
		}

	case "instagram":
		account, err := h.store.GetAccount(r.Context(), accountID)
		if err != nil {
			slog.Warn("social.oauth: get account for ig pages", "error", err)
			return
		}
		if err := syncInstagramPages(r, h, h.store, account); err != nil {
			slog.Warn("social.oauth: store ig pages", "error", err)
		} else {
			slog.Info("social.oauth: stored instagram pages", "account_id", accountID)
		}
	}
}
