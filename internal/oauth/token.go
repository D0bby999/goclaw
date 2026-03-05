package oauth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/crypto"
)

// TokenFile stores OAuth tokens on disk.
type TokenFile struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TokenSource provides a valid access token, refreshing automatically if needed.
type TokenSource struct {
	path   string // file path for token storage
	encKey string // encryption key (empty = plaintext)
	mu     sync.Mutex
	cached *TokenFile
}

// NewTokenSource creates a TokenSource that reads/writes tokens from the given path.
func NewTokenSource(path, encryptionKey string) *TokenSource {
	return &TokenSource{
		path:   path,
		encKey: encryptionKey,
	}
}

// Token returns a valid access token, refreshing if expired or about to expire.
func (ts *TokenSource) Token() (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Load from disk if not cached
	if ts.cached == nil {
		tf, err := ts.load()
		if err != nil {
			return "", fmt.Errorf("load oauth token: %w", err)
		}
		ts.cached = tf
	}

	// Refresh if expired or expiring within 5 minutes
	if time.Until(ts.cached.ExpiresAt) < 5*time.Minute {
		slog.Info("refreshing OpenAI OAuth token")
		oldRefreshToken := ts.cached.RefreshToken
		newToken, err := RefreshOpenAIToken(oldRefreshToken)
		if err != nil {
			return "", fmt.Errorf("refresh oauth token: %w", err)
		}
		refreshToken := newToken.RefreshToken
		if refreshToken == "" {
			refreshToken = oldRefreshToken // keep old if server didn't issue a new one
		}
		ts.cached = &TokenFile{
			AccessToken:  newToken.AccessToken,
			RefreshToken: refreshToken,
			ExpiresAt:    time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second),
		}
		if err := ts.save(ts.cached); err != nil {
			slog.Warn("failed to save refreshed token", "error", err)
		}
	}

	return ts.cached.AccessToken, nil
}

// Save persists a token response to disk.
func (ts *TokenSource) Save(resp *OpenAITokenResponse) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	tf := &TokenFile{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	ts.cached = tf
	return ts.save(tf)
}

func (ts *TokenSource) save(tf *TokenFile) error {
	if err := os.MkdirAll(filepath.Dir(ts.path), 0700); err != nil {
		return err
	}

	data, err := json.Marshal(tf)
	if err != nil {
		return err
	}

	// Encrypt if key is available
	content := string(data)
	if ts.encKey != "" {
		encrypted, err := crypto.Encrypt(content, ts.encKey)
		if err != nil {
			return fmt.Errorf("encrypt token: %w", err)
		}
		content = encrypted
	}

	return os.WriteFile(ts.path, []byte(content), 0600)
}

func (ts *TokenSource) load() (*TokenFile, error) {
	data, err := os.ReadFile(ts.path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Decrypt if encrypted
	if crypto.IsEncrypted(content) {
		if ts.encKey == "" {
			return nil, fmt.Errorf("token file is encrypted but GOCLAW_ENCRYPTION_KEY is not set")
		}
		decrypted, err := crypto.Decrypt(content, ts.encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt token: %w", err)
		}
		content = decrypted
	}

	var tf TokenFile
	if err := json.Unmarshal([]byte(content), &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// DefaultTokenPath returns the default path for storing OpenAI OAuth tokens.
func DefaultTokenPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".goclaw", "oauth", "openai.json")
}

// TokenFileExists returns true if a token file exists at the given path.
func TokenFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
