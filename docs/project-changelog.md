# Project Changelog

All notable changes to GoClaw project. Format: YYYY-MM-DD | Type | Brief description.

## 2026

### 2026-03-07

**feat:** Project Access Control for Team Members

- New `project_members` table (migration 000013) for explicit project access grants
- 4-level access check: system owner → project owner → explicit member → team-linked access
- `ListAccessibleProjects` store method: UNION of owned + member + team-linked projects
- Member management: `AddMember`, `RemoveMember`, `ListMembers`, `IsMember` store methods
- HTTP API: 3 new endpoints (`GET/POST/DELETE /v1/projects/{id}/members`)
- WebSocket RPC: 3 new methods (`projects.members.list/add/remove`)
- Access control enforced on all 16 project/session HTTP handlers and 16 WS RPC handlers
- `canAccess()` (read) and `canModify()` (owner-only) helpers in both HTTP and WS layers
- `ListProjects("")` fixed to return all active projects (system owner / management UI)
- New `ErrForbidden` protocol error code
- Files: 2 migrations, updated project_store.go, pg/project_store.go, http/projects.go, methods/projects.go, gateway.go, protocol/methods.go, protocol/errors.go

### 2026-03-05

**feat:** Claude Code Orchestration — process manager + HTTP/WebSocket APIs for Claude Code CLI

- New package `internal/claudecode/`: ProcessManager for spawning and monitoring Claude Code CLI child processes
- ProcessManager lifecycle: spawn, stream event parsing, concurrent session enforcement, signal handling
- Database migration 000010: three new tables (cc_projects, cc_sessions, cc_session_logs) with indices
- CCStore interface: 15 methods for projects (CRUD), sessions (lifecycle), logs (event streaming)
- HTTP API: 11 endpoints under `/v1/cc/` (project CRUD, session start/stop/logs, prompt sending)
- WebSocket RPC: 11 methods (cc.projects.*, cc.sessions.*) for real-time project and session management
- WebSocket events: `cc.output` (stream events), `cc.session.status` (status changes)
- React UI: 8 components for project management, session terminal, real-time process output monitoring
- Features: git worktree isolation, env var encryption (AES-256-GCM), per-project session limits, owner/team access control
- Files: 7 Go files in `internal/claudecode/`, HTTP handler, RPC handler, 8 React components

### 2026-03-04

**feat:** Scraper tool Go rewrite — 11-actor composite web scraper

- Complete rewrite of TypeScript web crawler (`webapp/creatorstudio/packages/crawler/`) as native Go tool
- Single `scraper` tool with `actor` parameter for platform selection
- 11 platform actors: Reddit, Twitter, TikTok, YouTube, Instagram, Instagram Reels, Facebook, Google Search, Google Trends, Ecommerce, Website content crawler
- Core subsystems: HTTP client with stealth headers + anti-detection, session pool with error-score rotation, cookie encryption (AES-256-GCM), extractors pipeline (JSON-LD, OpenGraph, Schema.org, CSS, XPath, tables), discovery layer (robots.txt, sitemap, URL normalization), in-memory queue (BFS/DFS + dedup)
- Registered in tool registry under `web` group
- Accessible via gateway WebSocket + HTTP APIs
- Files: 40+ Go files in `internal/scraper/` (~5000 LOC)

---

## Previous Versions

(Earlier versions tracked in git commit history)
