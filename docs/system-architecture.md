# System Architecture

Detailed architecture documentation for GoClaw components and subsystems.

## Package Structure

```
internal/
├── gateway/                  WS + HTTP server, client, method router
│   └── methods/              RPC handlers (chat, agents, sessions, config, skills, cron, pairing)
├── agent/                    Agent loop (think→act→observe), router, resolver, input guard
├── providers/                LLM providers: Anthropic (native HTTP+SSE), OpenAI-compat (HTTP+SSE)
├── tools/                    Tool registry, filesystem, exec, web, memory, subagent, MCP bridge
├── scraper/                  Web scraper tool with 11 platform actors
│   ├── actor/                Base actor interface, runner, error classification
│   ├── httpclient/           HTTP client with retry, proxy, SSRF protection
│   ├── stealth/              Anti-detection: fingerprints, UA pool, headers, proxy rotation, timing
│   ├── session/              Session pool with error-score lifecycle, ban detection, cookie encryption
│   ├── extractor/            Data extraction: JSON-LD, OpenGraph, Schema.org, CSS, XPath, tables, social handles
│   ├── discovery/            robots.txt, sitemap parsing, URL normalization, link following
│   ├── queue/                In-memory BFS/DFS queue with deduplication
│   ├── actors/               11 platform implementations: reddit, twitter, tiktok, youtube, instagram, instagram_reel, facebook, google_search, google_trends, ecommerce, website
│   ├── tool.go               GoClaw Tool interface wrapper
│   └── actor_registry.go     Actor factory and registry
├── social/                   Social media management: accounts, posts, pages, OAuth 2.0
├── media/                    Media file storage and serving
├── store/                    Store interfaces + pg/ (PostgreSQL) + file/ (standalone) implementations
├── bootstrap/                System prompt files (SOUL.md, IDENTITY.md) + seeding + per-user seed
├── config/                   Config loading (JSON5) + env var overlay
├── channels/                 Multi-channel manager: Telegram, Feishu/Lark, Zalo, Discord, WhatsApp, Slack
├── http/                     HTTP API (/v1/chat/completions, /v1/agents, /v1/skills, /v1/news, /v1/projects, etc.)
├── skills/                   SKILL.md loader + BM25 search
├── memory/                   Memory system (SQLite FTS5 / pgvector)
├── tracing/                  LLM call tracing + optional OTel export (build-tag gated)
├── scheduler/                Lane-based concurrency (main/subagent/cron)
├── cron/                     Cron scheduling (at/every/cron expr)
├── permissions/              RBAC (admin/operator/viewer)
├── pairing/                  Browser pairing (8-char codes)
├── crypto/                   AES-256-GCM encryption for API keys
├── sandbox/                  Docker-based code sandbox
├── tts/                      Text-to-Speech (OpenAI, ElevenLabs, Edge, MiniMax)
├── hooks/                    Quality gates (command/agent evaluators)
└── mcp/                      Model Context Protocol bridge

pkg/
├── browser/                  Browser automation (Rod + Chrome DevTools Protocol)
└── protocol/                 Wire types (frames, methods, errors, events)
```

## Scraper Tool Architecture

### Overview

Single composite `scraper` tool implementing the `tools.Tool` interface. Dynamically routes to 11 platform-specific actors based on `actor` parameter.

### Design Pattern: Base Actor + Platform Actors

```
tools.Tool (interface)
    ↓
scraper.Tool (wrapper)
    ├── Execute(ctx, args) → routes to actor by name
    └── GetActor(name) → *ActorInstance

actor.Actor (interface)
    ├── Initialize(ctx) error
    ├── Execute(ctx, stats) ([]json.RawMessage, error)
    └── Cleanup()

ActorRun
    ├── Status string (succeeded/failed/timed-out)
    ├── Items []json.RawMessage
    ├── Errors []actor.Error
    └── Stats actor.RunStats
```

### Subsystem Layering

1. **Actor Foundation** (`internal/scraper/actor/`)
   - `Actor` interface: 3-method lifecycle
   - `RunActor()` orchestrator: timeout, lifecycle, stats collection
   - Error classification: network, auth, rate-limit, parse, validation, unknown

2. **HTTP Layer** (`internal/scraper/httpclient/`, `internal/scraper/stealth/`)
   - HTTP client: retry (exponential backoff, 3 attempts), proxy support, SSRF check
   - Stealth: fingerprints, UA pool (30+ realistic agents), header manipulation, proxy rotation, jittered delays
   - Goal: evade bot detection

3. **Session Management** (`internal/scraper/session/`)
   - Pool: error-score lifecycle, auto-rotate at maxErrorScore (default 3), max usage count (default 50)
   - Ban detection: HTTP 403/429, Cloudflare challenges, CAPTCHA markers
   - Cookie encryption: AES-256-GCM (reuse `internal/crypto`)

4. **Data Extraction** (`internal/scraper/extractor/`)
   - Structured: JSON-LD, OpenGraph meta tags, Schema.org microdata
   - DOM-based: CSS selectors (goquery), XPath (htmlquery), HTML tables
   - Text: social handles, emails, phone numbers (regex)

5. **URL Discovery** (`internal/scraper/discovery/`)
   - robots.txt: directive parser, User-agent rules, Crawl-delay
   - Sitemap: XML parsing, index files, gzip support
   - URL normalizer: lowercase scheme/host, sort query params, remove fragments
   - Link follower: extract href, resolve relative, apply filters

6. **Crawl Queue** (`internal/scraper/queue/`)
   - In-memory BFS (FIFO) or DFS (LIFO) strategy
   - Deduplication via normalized URL as key
   - Stats: pending, completed, failed counts

7. **Platform Actors** (`internal/scraper/actors/`)
   - **Reddit**: JSON API (append `.json` to URLs)
   - **Twitter**: Guest token auth, search + timeline APIs
   - **TikTok**: Video metadata extraction from page source
   - **YouTube**: Innertube API (replacement for public API)
   - **Instagram**: GraphQL endpoint parsing
   - **Instagram Reels**: Dedicated metadata parser
   - **Facebook**: mbasic.facebook.com (lightweight HTML)
   - **Google Search**: SERP parser (organic results, ads, PAA)
   - **Google Trends**: API client (interest, related, trending)
   - **Ecommerce**: Platform detector + per-site parsers (Amazon, eBay, Walmart, generic)
   - **Website**: Multi-page crawler with smart rendering (HTTP-first, fallback to Rod), robots.txt compliance, readability extraction, content cleaning, markdown conversion

### Error Handling Flow

```
Actor Execute() → error
    ↓
ClassifyError(err) → ErrorCategory
    ↓
IsRetryable(cat) → bool
    ↓
RunActor retry logic: exponential backoff (0.1s, 0.3s, 1s)
    ↓
On exhaustion: return ActorRun with status "failed", errors list populated
```

### Concurrency Model

- Single-threaded per invocation (no goroutines within actor)
- Thread-safe session pool (sync.Mutex)
- Thread-safe stealth managers (sync.Map for fingerprint cache)
- Timeout via context.WithTimeout + context.DeadlineExceeded check

### Tool Registration

```go
// In cmd/ wiring:
scraperTool := scraper.NewScraperTool(browserMgr, httpClient)
registry.Register(scraperTool)

// Policy addition:
policyEngine.AddToGroup("web", "scraper")

// Available in profiles:
// - full: yes (all tools)
// - coding: yes (includes group:web)
// - minimal: no
```

### Output Format

```json
{
  "actor": "reddit",
  "status": "succeeded",
  "items_count": 42,
  "items": [
    { "title": "...", "url": "...", ... }
  ],
  "stats": {
    "duration_ms": 3200,
    "requests_total": 5,
    "requests_failed": 0,
    "retries": 1
  },
  "errors": []
}
```

### File Size Management

- Each file: <200 lines (Phase 02 exception: http client ~150, stealth modules ~100 each)
- Composition over inheritance: platform actors composed from shared layers
- Single responsibility: each file handles one concern (parsing, extraction, etc.)

## Social Media Module

### Overview

The social module (`internal/social/`) provides multi-platform social media publishing, scheduling, and account management with OAuth 2.0 authentication.

### Supported Platforms

| Platform | Auth Type | Implementation | Status |
|----------|-----------|-----------------|--------|
| **Facebook** | OAuth 2.0 | Meta Graph API (versioned) | Implemented |
| **Instagram** | OAuth 2.0 | Meta Graph API (Business Account) | Implemented |
| **Threads** | OAuth 2.0 | Threads API (threads.net) | Implemented |
| **Twitter/X** | OAuth 2.0 (PKCE) | Twitter API v2 | Implemented |
| **LinkedIn** | OAuth 2.0 | LinkedIn OAuth endpoint | Implemented |
| **YouTube** | OAuth 2.0 | Google OAuth (YouTube Data API v3) | Implemented |
| **TikTok** | OAuth 2.0 | TikTok OAuth token endpoint | Implemented |
| **Bluesky** | Token-based | No OAuth (app password only) | Implemented |

### OAuth Configuration

Environment variables for each platform:

```env
# Meta (Facebook/Instagram/Threads)
FACEBOOK_APP_ID=your_app_id
FACEBOOK_APP_SECRET=your_app_secret
FACEBOOK_APP_VERSION=24          # Default Graph API version

# Twitter
TWITTER_CLIENT_ID=your_client_id
TWITTER_CLIENT_SECRET=your_client_secret

# LinkedIn
LINKEDIN_CLIENT_ID=your_client_id
LINKEDIN_CLIENT_SECRET=your_client_secret

# Google (YouTube)
GOOGLE_CLIENT_ID=your_client_id
GOOGLE_CLIENT_SECRET=your_client_secret

# TikTok
TIKTOK_CLIENT_KEY=your_client_key
TIKTOK_CLIENT_SECRET=your_client_secret
```

### HTTP OAuth Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/social/oauth/start` | GET | Initiate OAuth flow (`?platform=X`) → returns provider auth URL |
| `/v1/social/oauth/callback` | GET | OAuth provider callback (`?state=X&code=Y`) → redirects from provider |
| `/v1/social/oauth/status` | GET | List configured platforms with authorization status |

### Architecture

**Handler:** `internal/http/social_oauth.go`
- `SocialOAuthHandler`: routes OAuth flows per platform
- `PlatformOAuthConfigs`: holds all per-platform OAuth config structs
- Platform-specific handlers: `social_oauth_twitter.go`, `social_oauth_linkedin.go`, `social_oauth_youtube.go`

**Configuration:** `internal/config/config_channels.go` (SocialConfig struct)
- Loads env vars at startup
- Default Graph API version "24" for Meta platforms

**Database:**
- `social_oauth_states` table: CSRF protection, state→user_id mapping, TTL enforcement
- `social_accounts` table: stored credentials (token encrypted AES-256-GCM)

### OAuth Flow Per Platform

| Platform | Auth URL | Token Endpoint | Method |
|----------|----------|---|--------|
| Meta (FB/IG/Threads) | Facebook OAuth Dialog | `graph.facebook.com/{version}/oauth/access_token` | Authorization Code |
| Twitter | `api.twitter.com/2/oauth2/authorize` | `api.twitter.com/2/oauth2/token` | PKCE (no client secret in request) |
| LinkedIn | LinkedIn authorization endpoint | `linkedin.com/oauth/v2/accessToken` | Authorization Code |
| Google (YouTube) | Google accounts consent | `oauth2.googleapis.com/token` | Authorization Code |
| TikTok | TikTok OAuth authorize | `graph.tiktok.com/oauth/token` | Authorization Code |

## Media Management

### Overview
File upload and serving for channels + social media. Located in `internal/media/` and `internal/channels/media/`. Supports pluggable storage backends: AWS S3, Cloudflare R2, or local filesystem (default).

### Storage Backends
**Priority:** S3 > R2 > Local (factory auto-detects based on config).

| Backend | Config | Use Case |
|---------|--------|----------|
| **AWS S3** | `MediaS3Config` (bucket, region, credentials) | Standard cloud object storage |
| **Cloudflare R2** | `MediaR2Config` (bucket, account_id, credentials) | Low-cost R2 alternative |
| **Local Filesystem** | `localDir` (default: `~/.goclaw/media/`) | Fallback, single-instance deployments |

### Configuration
All backends configured via `config.Media.S3` / `config.Media.R2` in JSON5 config or env vars:

**S3 env vars:**
- `GOCLAW_S3_BUCKET`, `GOCLAW_S3_REGION`, `GOCLAW_S3_ACCESS_KEY_ID`, `GOCLAW_S3_SECRET_ACCESS_KEY`
- `GOCLAW_S3_ENDPOINT` (optional, for S3-compatible services)
- `GOCLAW_S3_PREFIX` (path prefix in bucket)
- `GOCLAW_S3_PUBLIC_URL` (custom public URL, overrides default)
- `GOCLAW_S3_URL_EXPIRY` (signed URL expiration, default: 1h)
- `GOCLAW_S3_FORCE_PATH_STYLE` (bool, for MinIO/non-AWS S3)

**R2 env vars:**
- `GOCLAW_R2_BUCKET`, `GOCLAW_R2_ACCOUNT_ID`, `GOCLAW_R2_ACCESS_KEY_ID`, `GOCLAW_R2_SECRET_ACCESS_KEY`
- `GOCLAW_R2_PREFIX`, `GOCLAW_R2_PUBLIC_URL`, `GOCLAW_R2_URL_EXPIRY` (same as S3)

### Storage Interface
```go
type Storage interface {
	SaveFile(sessionKey, srcPath, mimeType string) (id string, err error)
	LoadPath(id string) (string, error)          // returns local path or temp file from cloud
	PublicURL(id string) string                   // returns public URL or empty for local
	DeleteSession(sessionKey string) error        // cleanup on session end
}
```

Implementations: `LocalStorage` (`store.go`), `S3Storage` (`s3.go`).

### HTTP Endpoints
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/media/upload` | POST | Upload media file (50MB limit), returns media ID |
| `/v1/media/{id}` | GET | Serve media by ID with proper MIME type |
| `/v1/storage/files` | GET | List workspace files in `~/.goclaw/` |
| `/v1/storage/files/{path}` | GET/DELETE | Browse/delete storage (skills protected) |

## News Management

### Tools & HTTP API
Located in `internal/tools/news.go` and `internal/http/news.go`.

**NewsTool** (unified actions):
- `save` — Save scraped news item (URL dedup)
- `query` — Search/filter by categories, date, keywords
- `sources` — List configured sources
- `ideas` — Extract business/app ideas from news

**HTTP Endpoints**:
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/news/sources` | GET/POST/PUT/DELETE | Manage news sources |
| `/v1/news/items` | GET | List saved news items |
| `/v1/news/items/{id}` | GET | Get specific news item |

## Project Sessions

### Overview
Project-scoped AI sessions with team management. Located in `internal/http/projects.go`.

**Features**:
- Create/list/update/delete projects
- Isolated sessions per project
- Team member management (add/remove/list)
- Session lifecycle: start, stop, prompt, log streaming
- Persistent session logs

**HTTP Endpoints**:
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/projects` | GET/POST | List/create projects |
| `/v1/projects/{id}` | GET/PUT/DELETE | Manage project |
| `/v1/projects/{id}/sessions` | GET/POST | List/start sessions |
| `/v1/project-sessions/{id}` | GET/PATCH/DELETE | Manage session |
| `/v1/project-sessions/{id}/prompt` | POST | Send prompt |
| `/v1/project-sessions/{id}/stop` | POST | Stop session |
| `/v1/project-sessions/{id}/logs` | GET | Get session logs |
| `/v1/projects/{id}/members` | GET/POST/DELETE | Manage team |

**Tools**:
- `projects_list` — Discover available projects
- `projects_start_session` — Start new session in project
- `projects_send_prompt` — Send prompt to running session

## Storage Management

### Overview
Workspace file browser for `~/.goclaw/` directory. Located in `internal/http/storage.go`.

**Features**:
- Browse directory tree
- Read/delete files
- Skills directories (read-only)
- Path traversal protection

## Social Media Integration

### Architecture
Multi-platform social media publishing, scheduling, and OAuth. Located in `internal/social/` and `internal/http/social*.go`.

**Handlers**:
- `SocialHandler` — Account & post management
- `SocialOAuthHandler` — OAuth flows per platform
- `SocialPagesHandler` — Page CRUD (Facebook, Instagram)

**HTTP Endpoints**:

*Accounts & Posts*:
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/social/accounts` | GET/POST/PUT/DELETE | Manage social accounts |
| `/v1/social/posts` | GET/POST/PUT/DELETE | Manage posts |
| `/v1/social/posts/{id}/publish` | POST | Publish post |
| `/v1/social/posts/{id}/targets` | POST/DELETE | Add/remove publish targets |
| `/v1/social/posts/{id}/media` | POST/DELETE | Attach/remove media |
| `/v1/social/adapt` | POST | Content platform adaptation |
| `/v1/social/platforms` | GET | List supported platforms |

*OAuth*:
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/social/oauth/start` | GET | Initiate OAuth (`?platform=X`) |
| `/v1/social/oauth/callback` | GET | OAuth provider callback |
| `/v1/social/oauth/status` | GET | List platforms + auth status |

*Pages*:
| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/social/accounts/{id}/pages` | GET/POST | List/create pages |
| `/v1/social/accounts/{id}/pages/sync` | POST | Sync pages from platform |
| `/v1/social/pages/{id}/default` | PUT | Set default page |
| `/v1/social/pages/{id}` | DELETE | Delete page |

**Platforms**: Facebook, Instagram, Threads, Twitter/X, LinkedIn, YouTube, TikTok, Bluesky (via OAuth or token).

**Credential Storage**: Tokens encrypted AES-256-GCM in `social_accounts` table.

## Browser Automation

### Overview
Headless browser control via Rod + Chrome DevTools Protocol. Located in `pkg/browser/`.

**Features**:
- Tab/page lifecycle management
- JavaScript evaluation
- Screenshot/PDF generation
- Form automation
- DOM interaction
- Network inspection

**Social Platform Implementations**:
- TikTok browser upload (`internal/social/client_tiktok_browser.go`)
- Multi-platform fingerprinting

## Channels (Multi-Platform Messaging)

### Supported Platforms
Telegram, Discord, Feishu/Lark, Zalo, WhatsApp, Slack.

### Architecture
Located in `internal/channels/`:
- `manager.go` — Channel factory + lifecycle
- `channel.go` — Base interface
- Platform-specific dirs with protocol handlers
- `media/` — Media handling per platform
- `history.go` — Message history + persistence
- `ratelimit.go` — Per-channel rate limiting
- `quota.go` — Usage quotas + enforcement

**Features**:
- Connection pooling per platform
- Webhook-based inbound + polling
- Message encryption where applicable
- Media conversion per platform specs
- Type safety via `internal/channels/typing/`

## Sandbox (Docker-based Code Execution)

### Overview
Isolated code execution environment. Located in `internal/sandbox/`.

**Components**:
- `docker.go` — Docker container lifecycle
- `sandbox.go` — Execution harness
- `fsbridge.go` — Container ↔ Host file sharing

**Features**:
- Timeout enforcement
- Resource limits (CPU/memory)
- Network isolation (opt-out)
- Filesystem sandbox with tmpdir
- Language support: Python, Node.js, Go, Bash

## Text-to-Speech (TTS)

### Overview
Multiple TTS provider support. Located in `internal/tts/`.

**Providers**:
- OpenAI (voice synthesis)
- ElevenLabs (quality TTS)
- Edge (local, zero-cost)
- MiniMax (Chinese optimized)

**Manager**:
- `manager.go` — Provider selection + fallback
- `types.go` — Common voice/format types

## Related Documentation

- **Tools System**: `docs/03-tools-system.md` (tool registry, policies, execution flow)
- **Tool Inventory**: Listed in Section 2 (Complete Tool Inventory)
- **Plan**: `plans/260303-2311-scraper-tool-go-rewrite/plan.md` (11 phases, implementation details)
