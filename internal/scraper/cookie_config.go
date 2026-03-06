package scraper

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

const cookieKeyPrefix = "scraper.cookies."

// ScraperCookieEntry represents a stored scraper cookie set.
type ScraperCookieEntry struct {
	Label     string `json:"label"`
	Platform  string `json:"platform"`
	Cookies   string `json:"cookies"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ScraperCookieStore manages scraper cookies via the encrypted ConfigSecretsStore.
type ScraperCookieStore struct {
	secrets store.ConfigSecretsStore
}

// NewScraperCookieStore creates a new ScraperCookieStore.
func NewScraperCookieStore(secrets store.ConfigSecretsStore) *ScraperCookieStore {
	return &ScraperCookieStore{secrets: secrets}
}

func cookieKey(platform, label string) string {
	slug := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(label)), " ", "_")
	return cookieKeyPrefix + platform + "." + slug
}

// List returns all stored cookie entries.
func (s *ScraperCookieStore) List(ctx context.Context) ([]ScraperCookieEntry, error) {
	all, err := s.secrets.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	var entries []ScraperCookieEntry
	for key, val := range all {
		if !strings.HasPrefix(key, cookieKeyPrefix) {
			continue
		}
		var entry ScraperCookieEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Get returns a single cookie entry by platform and label.
func (s *ScraperCookieStore) Get(ctx context.Context, platform, label string) (*ScraperCookieEntry, error) {
	val, err := s.secrets.Get(ctx, cookieKey(platform, label))
	if err != nil {
		return nil, err
	}
	if val == "" {
		return nil, nil
	}
	var entry ScraperCookieEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Set stores or updates a cookie entry.
func (s *ScraperCookieStore) Set(ctx context.Context, entry ScraperCookieEntry) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if entry.CreatedAt == "" {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.secrets.Set(ctx, cookieKey(entry.Platform, entry.Label), string(b))
}

// Delete removes a cookie entry.
func (s *ScraperCookieStore) Delete(ctx context.Context, platform, label string) error {
	return s.secrets.Delete(ctx, cookieKey(platform, label))
}

// GetDefault returns the cookies string for the first available entry for a platform.
func (s *ScraperCookieStore) GetDefault(ctx context.Context, platform string) (string, error) {
	all, err := s.secrets.GetAll(ctx)
	if err != nil {
		return "", err
	}
	prefix := cookieKeyPrefix + platform + "."
	for key, val := range all {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		var entry ScraperCookieEntry
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			continue
		}
		if entry.Cookies != "" {
			return entry.Cookies, nil
		}
	}
	return "", nil
}
