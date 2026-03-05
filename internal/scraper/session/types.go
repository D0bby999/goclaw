package session

import (
	"net/url"
	"time"
)

// Session represents a scraped web session with cookies and identity.
type Session struct {
	ID            string
	Cookies       map[string]string
	UserAgent     string
	Proxy         *url.URL
	ErrorScore    int
	UsageCount    int
	CreatedAt     time.Time
	IsUsable      bool
	FingerprintID string
}

// PoolConfig controls session pool limits.
type PoolConfig struct {
	MaxSessions   int // default 10
	MaxErrorScore int // default 3
	MaxUsageCount int // default 50
}

// PoolStats summarizes the current pool state.
type PoolStats struct {
	Total  int
	Usable int
}

// DefaultPoolConfig returns sensible defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxSessions:   10,
		MaxErrorScore: 3,
		MaxUsageCount: 50,
	}
}
