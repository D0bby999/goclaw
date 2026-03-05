package stealth

import (
	crand "crypto/rand"
	"encoding/hex"
	mrand "math/rand/v2"
	"sync"
)

var commonScreens = []ScreenSize{
	{1920, 1080},
	{1440, 900},
	{1366, 768},
	{1536, 864},
	{1280, 800},
	{1600, 900},
	{2560, 1440},
	{1280, 1024},
	{1024, 768},
	{1920, 1200},
}

// FingerprintManager generates and caches browser fingerprints.
type FingerprintManager struct {
	mu           sync.RWMutex
	fingerprints map[string]Fingerprint
	uaPool       *UserAgentPool
}

// NewFingerprintManager creates a manager backed by the given UA pool.
func NewFingerprintManager(uaPool *UserAgentPool) *FingerprintManager {
	return &FingerprintManager{
		fingerprints: make(map[string]Fingerprint),
		uaPool:       uaPool,
	}
}

// Generate creates a new Fingerprint with random UA, matching headers, and screen size.
func (m *FingerprintManager) Generate() Fingerprint {
	id := generateID()
	ua := m.uaPool.Random()
	screen := commonScreens[mrand.IntN(len(commonScreens))]

	fp := Fingerprint{
		ID:        id,
		UserAgent: ua,
		Screen:    screen,
		Headers:   BuildStealthHeaders(Fingerprint{UserAgent: ua}),
	}

	m.mu.Lock()
	m.fingerprints[id] = fp
	m.mu.Unlock()

	return fp
}

// Get retrieves a cached fingerprint by ID.
func (m *FingerprintManager) Get(id string) (Fingerprint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fp, ok := m.fingerprints[id]
	return fp, ok
}

// Invalidate removes a fingerprint from the cache.
func (m *FingerprintManager) Invalidate(id string) {
	m.mu.Lock()
	delete(m.fingerprints, id)
	m.mu.Unlock()
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = crand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return hex.EncodeToString(b[:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:])
}
