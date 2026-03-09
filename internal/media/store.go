package media

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// LocalStorage provides persistent media file storage on the local filesystem.
// Files are organized as: {baseDir}/{sessionHash}/{uuid}.{ext}
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a local media store rooted at baseDir.
// The directory is created if it doesn't exist.
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("media: create base dir: %w", err)
	}
	return &LocalStorage{baseDir: baseDir}, nil
}

// SaveFile moves or copies a file to persistent storage.
// Returns the unique media ID.
func (s *LocalStorage) SaveFile(sessionKey, srcPath, mime string) (string, error) {
	dir := s.sessionDir(sessionKey)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("media: create session dir: %w", err)
	}

	mediaID := uuid.New().String()
	ext := extFromMime(mime)
	if ext == "" {
		ext = filepath.Ext(srcPath)
	}
	dstPath := filepath.Join(dir, mediaID+ext)

	// Try rename first (fast, same filesystem).
	if err := os.Rename(srcPath, dstPath); err == nil {
		return mediaID, nil
	}

	// Fallback: copy + remove source.
	if err := copyFile(srcPath, dstPath); err != nil {
		return "", fmt.Errorf("media: copy file: %w", err)
	}
	_ = os.Remove(srcPath) // best-effort cleanup of source
	return mediaID, nil
}

// PublicURL returns "" for local storage (served via gateway HTTP).
func (s *LocalStorage) PublicURL(id string) string { return "" }

// LoadPath returns the filesystem path for a media ID.
// Returns an error if the file doesn't exist.
func (s *LocalStorage) LoadPath(id string) (string, error) {
	// Media files are stored as {sessionHash}/{id}.{ext}.
	// Since we don't know the session hash, glob for the ID across all session dirs.
	matches, err := filepath.Glob(filepath.Join(s.baseDir, "*", id+".*"))
	if err != nil {
		return "", fmt.Errorf("media: glob for %s: %w", id, err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("media: file not found: %s", id)
	}
	return matches[0], nil
}

// DeleteSession removes all media files for a session.
func (s *LocalStorage) DeleteSession(sessionKey string) error {
	dir := s.sessionDir(sessionKey)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("media: failed to delete session dir", "dir", dir, "error", err)
		return err
	}
	return nil
}

// sessionDir returns the directory path for a session's media files.
// Uses first 12 chars of SHA-256 hash of sessionKey for filesystem safety.
func (s *LocalStorage) sessionDir(sessionKey string) string {
	h := sha256.Sum256([]byte(sessionKey))
	hash := fmt.Sprintf("%x", h[:6]) // 12 hex chars
	return filepath.Join(s.baseDir, hash)
}

// extFromMime returns a file extension (with dot) for a MIME type.
func extFromMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(mime, "image/png"):
		return ".png"
	case strings.HasPrefix(mime, "image/gif"):
		return ".gif"
	case strings.HasPrefix(mime, "image/webp"):
		return ".webp"
	case strings.HasPrefix(mime, "video/mp4"):
		return ".mp4"
	case strings.HasPrefix(mime, "audio/ogg"), strings.HasPrefix(mime, "audio/opus"):
		return ".ogg"
	case strings.HasPrefix(mime, "audio/mpeg"):
		return ".mp3"
	case strings.HasPrefix(mime, "audio/wav"):
		return ".wav"
	case strings.HasPrefix(mime, "application/pdf"):
		return ".pdf"
	case mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case mime == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	default:
		return ""
	}
}

// copyFile copies src to dst using buffered I/O.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
