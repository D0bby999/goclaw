package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/stealth"
)

// Pool manages a bounded set of reusable sessions.
type Pool struct {
	mu       sync.Mutex
	sessions map[string]*Session
	cfg      PoolConfig
	fpMgr    *stealth.FingerprintManager
	proxyRot *stealth.ProxyRotator
}

// NewPool creates a Pool with the given config and stealth components.
func NewPool(cfg PoolConfig, fpMgr *stealth.FingerprintManager, proxyRot *stealth.ProxyRotator) *Pool {
	return &Pool{
		sessions: make(map[string]*Session),
		cfg:      cfg,
		fpMgr:    fpMgr,
		proxyRot: proxyRot,
	}
}

// GetSession returns a usable session or creates a new one.
// If pool is full and no usable session exists, retires worst first.
func (p *Pool) GetSession() *Session {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find first usable session within limits.
	for _, s := range p.sessions {
		if s.IsUsable &&
			s.ErrorScore < p.cfg.MaxErrorScore &&
			s.UsageCount < p.cfg.MaxUsageCount {
			return s
		}
	}

	// If pool is full, retire worst to make room.
	if len(p.sessions) >= p.cfg.MaxSessions {
		p.retireWorst()
	}

	return p.createSession()
}

// MarkGood records a successful use: decrements errorScore (min 0), increments usageCount.
func (p *Pool) MarkGood(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.sessions[id]
	if !ok {
		return
	}
	if s.ErrorScore > 0 {
		s.ErrorScore--
	}
	s.UsageCount++
}

// MarkBad records a failed use: increments errorScore; disables session if threshold reached.
func (p *Pool) MarkBad(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.sessions[id]
	if !ok {
		return
	}
	s.ErrorScore++
	if s.ErrorScore >= p.cfg.MaxErrorScore {
		s.IsUsable = false
	}
}

// Retire removes the session from the pool and invalidates its fingerprint.
func (p *Pool) Retire(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.sessions[id]
	if !ok {
		return
	}
	if p.fpMgr != nil && s.FingerprintID != "" {
		p.fpMgr.Invalidate(s.FingerprintID)
	}
	delete(p.sessions, id)
}

// Stats returns a snapshot of the pool state.
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()

	stats := PoolStats{Total: len(p.sessions)}
	for _, s := range p.sessions {
		if s.IsUsable &&
			s.ErrorScore < p.cfg.MaxErrorScore &&
			s.UsageCount < p.cfg.MaxUsageCount {
			stats.Usable++
		}
	}
	return stats
}

// retireWorst removes the session with highest errorScore*100 + usageCount.
// Caller must hold p.mu.
func (p *Pool) retireWorst() {
	var worstID string
	worstScore := -1

	for id, s := range p.sessions {
		score := s.ErrorScore*100 + s.UsageCount
		if score > worstScore {
			worstScore = score
			worstID = id
		}
	}

	if worstID == "" {
		return
	}
	s := p.sessions[worstID]
	if p.fpMgr != nil && s.FingerprintID != "" {
		p.fpMgr.Invalidate(s.FingerprintID)
	}
	delete(p.sessions, worstID)
}

// createSession builds a new Session with a generated fingerprint and optional proxy.
// Caller must hold p.mu.
func (p *Pool) createSession() *Session {
	s := &Session{
		ID:        uuid.New().String(),
		Cookies:   make(map[string]string),
		CreatedAt: time.Now(),
		IsUsable:  true,
	}

	if p.fpMgr != nil {
		fp := p.fpMgr.Generate()
		s.FingerprintID = fp.ID
		s.UserAgent = fp.UserAgent
	}

	if p.proxyRot != nil {
		s.Proxy = p.proxyRot.GetProxy()
	}

	p.sessions[s.ID] = s
	return s
}
