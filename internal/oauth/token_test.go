package oauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenSourceSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens", "test.json")

	ts := NewTokenSource(path, "")

	resp := &OpenAITokenResponse{
		AccessToken:  "access-token-abc123",
		RefreshToken: "refresh-token-xyz789",
		ExpiresIn:    3600,
	}

	if err := ts.Save(resp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file was created with correct permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Load and verify token
	ts2 := NewTokenSource(path, "")
	token, err := ts2.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "access-token-abc123" {
		t.Errorf("Token() = %q, want %q", token, "access-token-abc123")
	}
}

func TestTokenSourceEncrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "encrypted.json")
	encKey := "test-encryption-key-32-bytes!XYZ" // exactly 32 bytes for AES-256

	ts := NewTokenSource(path, encKey)

	resp := &OpenAITokenResponse{
		AccessToken:  "secret-access-token",
		RefreshToken: "secret-refresh-token",
		ExpiresIn:    7200,
	}

	if err := ts.Save(resp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file content is not plaintext
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) == "" {
		t.Fatal("file is empty")
	}

	// Load with correct key should work
	ts2 := NewTokenSource(path, encKey)
	token, err := ts2.Token()
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if token != "secret-access-token" {
		t.Errorf("Token() = %q, want %q", token, "secret-access-token")
	}

	// Load with wrong key should fail
	ts3 := NewTokenSource(path, "wrong-key-that-is-32-bytes-long!")
	_, err = ts3.Token()
	if err == nil {
		t.Error("expected error with wrong key, got nil")
	}
}

func TestTokenSourceCaching(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cached.json")

	ts := NewTokenSource(path, "")

	resp := &OpenAITokenResponse{
		AccessToken:  "cached-token",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
	}

	if err := ts.Save(resp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// First call loads from disk
	token1, err := ts.Token()
	if err != nil {
		t.Fatalf("Token (1): %v", err)
	}

	// Delete the file — cached token should still work
	os.Remove(path)

	token2, err := ts.Token()
	if err != nil {
		t.Fatalf("Token (2): %v", err)
	}

	if token1 != token2 {
		t.Errorf("cached tokens differ: %q vs %q", token1, token2)
	}
}

func TestDefaultTokenPath(t *testing.T) {
	path := DefaultTokenPath()
	if path == "" {
		t.Error("DefaultTokenPath() returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultTokenPath() = %q, expected absolute path", path)
	}
	if filepath.Base(path) != "openai.json" {
		t.Errorf("DefaultTokenPath() base = %q, want openai.json", filepath.Base(path))
	}
}

func TestTokenFileExists(t *testing.T) {
	dir := t.TempDir()

	// Non-existent
	if TokenFileExists(filepath.Join(dir, "nonexistent.json")) {
		t.Error("TokenFileExists returned true for non-existent file")
	}

	// Create file
	path := filepath.Join(dir, "exists.json")
	os.WriteFile(path, []byte("{}"), 0600)
	if !TokenFileExists(path) {
		t.Error("TokenFileExists returned false for existing file")
	}
}

func TestTokenFileSaveCreatesExpiresAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "expiry.json")

	ts := NewTokenSource(path, "")
	before := time.Now()

	resp := &OpenAITokenResponse{
		AccessToken:  "token",
		RefreshToken: "refresh",
		ExpiresIn:    3600,
	}
	if err := ts.Save(resp); err != nil {
		t.Fatalf("Save: %v", err)
	}

	after := time.Now()

	// Load and check expiry
	ts2 := NewTokenSource(path, "")
	tf, err := ts2.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	expectedMin := before.Add(3600 * time.Second)
	expectedMax := after.Add(3600 * time.Second)

	if tf.ExpiresAt.Before(expectedMin) || tf.ExpiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt = %v, want between %v and %v", tf.ExpiresAt, expectedMin, expectedMax)
	}
}
