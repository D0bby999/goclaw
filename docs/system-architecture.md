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
├── store/                    Store interfaces + pg/ (PostgreSQL) + file/ (standalone) implementations
├── bootstrap/                System prompt files (SOUL.md, IDENTITY.md) + seeding + per-user seed
├── config/                   Config loading (JSON5) + env var overlay
├── channels/                 Channel manager: Telegram, Feishu/Lark, Zalo, Discord, WhatsApp
├── http/                     HTTP API (/v1/chat/completions, /v1/agents, /v1/skills, etc.)
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

## Related Documentation

- **Tools System**: `docs/03-tools-system.md` (tool registry, policies, execution flow)
- **Tool Inventory**: Listed in Section 2 (Complete Tool Inventory)
- **Plan**: `plans/260303-2311-scraper-tool-go-rewrite/plan.md` (11 phases, implementation details)
