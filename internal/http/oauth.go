package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/oauth"
)

// OAuthHandler handles OAuth-related HTTP endpoints for web UI.
// Available in both standalone and managed modes.
type OAuthHandler struct {
	token  string // gateway auth token
	encKey string // encryption key for token storage

	mu      sync.Mutex
	pending *oauth.PendingLogin // active OAuth flow (if any)
}

// NewOAuthHandler creates a handler for OAuth endpoints.
func NewOAuthHandler(token, encryptionKey string) *OAuthHandler {
	return &OAuthHandler{token: token, encKey: encryptionKey}
}

// RegisterRoutes registers OAuth routes on the given mux.
func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/auth/openai/status", h.auth(h.handleStatus))
	mux.HandleFunc("POST /v1/auth/openai/start", h.auth(h.handleStart))
	mux.HandleFunc("POST /v1/auth/openai/callback", h.auth(h.handleManualCallback))
	mux.HandleFunc("POST /v1/auth/openai/logout", h.auth(h.handleLogout))
}

func (h *OAuthHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatch(extractBearerToken(r), h.token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (h *OAuthHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	tokenPath := oauth.DefaultTokenPath()
	if !oauth.TokenFileExists(tokenPath) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"authenticated": false,
		})
		return
	}

	ts := oauth.NewTokenSource(tokenPath, h.encKey)
	if _, err := ts.Token(); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"authenticated": false,
			"error":         "token invalid or expired",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"provider_name": "openai-codex",
	})
}

// handleStart initiates the OAuth PKCE flow from the web UI.
// Starts a local callback server and returns the auth URL for the frontend to open.
func (h *OAuthHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Already authenticated? (check inside mutex to avoid TOCTOU)
	tokenPath := oauth.DefaultTokenPath()
	if oauth.TokenFileExists(tokenPath) {
		ts := oauth.NewTokenSource(tokenPath, h.encKey)
		if _, err := ts.Token(); err == nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "already_authenticated",
			})
			return
		}
	}

	// Shut down any previous pending flow to release port 1455
	if h.pending != nil {
		h.pending.Shutdown()
		h.pending = nil
	}

	// Start new OAuth flow
	pending, err := oauth.StartLoginOpenAI()
	if err != nil {
		slog.Error("oauth.start", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to start OAuth flow (is port 1455 available?)",
		})
		return
	}

	h.pending = pending

	// Wait for callback in background, save token when done
	go h.waitForCallback(pending)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"auth_url": pending.AuthURL,
	})
}

// waitForCallback waits for the OAuth callback and saves the token.
func (h *OAuthHandler) waitForCallback(pending *oauth.PendingLogin) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	tokenResp, err := pending.Wait(ctx)

	h.mu.Lock()
	if h.pending == pending {
		h.pending = nil
	}
	h.mu.Unlock()

	if err != nil {
		slog.Warn("oauth.callback failed", "error", err)
		return
	}

	// Save token
	tokenPath := oauth.DefaultTokenPath()
	ts := oauth.NewTokenSource(tokenPath, h.encKey)
	if err := ts.Save(tokenResp); err != nil {
		slog.Error("oauth.save_token", "error", err)
		return
	}

	slog.Info("oauth: OpenAI token saved via web UI", "path", tokenPath)
}

// handleManualCallback accepts a pasted redirect URL from the frontend
// for remote/VPS environments where localhost:1455 callback can't be reached.
func (h *OAuthHandler) handleManualCallback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RedirectURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "redirect_url is required"})
		return
	}

	h.mu.Lock()
	pending := h.pending
	h.mu.Unlock()

	if pending == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no pending OAuth flow"})
		return
	}

	tokenResp, err := pending.ExchangeRedirectURL(body.RedirectURL)
	if err != nil {
		slog.Warn("oauth.manual_callback", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Shut down the callback server and clear pending
	pending.Shutdown()
	h.mu.Lock()
	if h.pending == pending {
		h.pending = nil
	}
	h.mu.Unlock()

	// Save token
	tokenPath := oauth.DefaultTokenPath()
	ts := oauth.NewTokenSource(tokenPath, h.encKey)
	if err := ts.Save(tokenResp); err != nil {
		slog.Error("oauth.save_token", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save token"})
		return
	}

	slog.Info("oauth: OpenAI token saved via manual callback", "path", tokenPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"provider_name": "openai-codex",
	})
}

func (h *OAuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	tokenPath := oauth.DefaultTokenPath()
	if err := os.Remove(tokenPath); err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "no token found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}
