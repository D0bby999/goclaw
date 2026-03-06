# 13 - Scraper Tool

The `scraper` tool enables agents to extract structured data from websites and social media platforms. It exposes a single tool interface with 11 platform-specific **actors**.

---

## 1. Tool Interface

```json
{
  "actor": "<platform_name>",
  "input": { /* actor-specific parameters */ }
}
```

Both `actor` and `input` are required. The tool creates a stealth HTTP client (SSRF-safe, fingerprint rotation, exponential backoff retry), runs the actor lifecycle (`Initialize` → `Execute` → `Cleanup`), and returns formatted results.

**Global defaults:** 5-minute timeout, 3 retries, 2s request delay.

---

## 2. Actor Reference

### reddit

Scrape subreddit feeds, individual posts, and search results. No auth required.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `subreddits` | `[]string` | no* | — | Subreddit names (e.g. `["golang", "programming"]`) |
| `usernames` | `[]string` | no* | — | Reddit usernames to scrape submitted posts |
| `post_urls` | `[]string` | no* | — | Direct Reddit post URLs |
| `search_queries` | `[]string` | no* | — | Search terms |
| `sort_by` | `string` | no | `"hot"` | `hot`, `new`, `top`, `rising` |
| `time_filter` | `string` | no | — | `hour`, `day`, `week`, `month`, `year`, `all` |
| `max_posts_per_source` | `int` | no | 25 | Posts per subreddit/user/search |
| `max_comments_per_post` | `int` | no | 20 | Comments per post |
| `include_comments` | `bool` | no | false | Fetch comments for each post |
| `request_delay_ms` | `int` | no | 1000 | Delay between requests |

*At least one of `subreddits`, `usernames`, `post_urls`, or `search_queries` required.

```json
{
  "actor": "reddit",
  "input": {
    "subreddits": ["golang"],
    "sort_by": "top",
    "time_filter": "week",
    "max_posts_per_source": 10,
    "include_comments": true
  }
}
```

---

### twitter

Scrape Twitter/X profiles, tweets, and search. Uses FxTwitter public API for profiles/tweets and Brave Search for tweet discovery. No auth required.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `handles` | `[]string` | no* | — | Usernames or profile URLs (with/without `@`). Returns profile + latest tweets. |
| `tweet_urls` | `[]string` | no* | — | Direct tweet URLs (x.com or twitter.com) |
| `search_queries` | `[]string` | no* | — | Search terms (uses Brave Search with `site:x.com`) |
| `max_results` | `int` | no | 10 | Max search results |
| `max_tweets_per_user` | `int` | no | 20 | Latest tweets per handle |
| `sort_by` | `string` | no | `"latest"` | `latest` (newest first) or `top` (by engagement) |
| `time_filter` | `string` | no | `"all"` | `day`, `week`, `month`, `year`, `all` |
| `request_delay_ms` | `int` | no | 1000 | Delay between requests |

*At least one of `handles`, `tweet_urls`, or `search_queries` required.

```json
{
  "actor": "twitter",
  "input": {
    "handles": ["elonmusk"],
    "max_tweets_per_user": 5,
    "sort_by": "top",
    "time_filter": "week"
  }
}
```

---

### tiktok

Scrape TikTok videos, user profiles, and search results via tikwm.com API. No auth required.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `video_urls` | `[]string` | no* | — | TikTok video URLs (fetches video metadata + download URLs) |
| `usernames` | `[]string` | no* | — | TikTok usernames (fetches user profile info and recent videos) |
| `search_queries` | `[]string` | no* | — | Search keywords (fetches videos by keyword) |
| `max_results` | `int` | no | 20 | Max items per source |
| `request_delay_ms` | `int` | no | 1500 | Delay between requests |

*At least one of `video_urls`, `usernames`, or `search_queries` required.

Output fields per video:
- `id`, `url`, `title`, `author`, `author_url`
- `likes`, `comments`, `shares`, `views`, `duration`, `created_at`
- `music_title`, `music_author`, `cover_url`
- `video_url` — standard quality download link
- `video_hd_url` — high-definition download link
- `is_ad` — whether video is an advertisement

Output fields per profile:
- `username`, `display_name`, `bio`, `url`
- `followers`, `following`, `likes`, `video_count`
- `avatar_url`, `verified`

```json
{
  "actor": "tiktok",
  "input": {
    "search_queries": ["golang"],
    "max_results": 10
  }
}
```

---

### youtube

Scrape YouTube videos, channels, shorts, and search. Uses InnerTube API. No auth required.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `start_urls` | `[]string` | no* | — | Video, Short, or Channel URLs |
| `search_keywords` | `string` | no* | — | Search query |
| `max_results` | `int` | no | 20 | Max results |
| `max_comments` | `int` | no | — | Max comments per video |
| `request_delay_ms` | `int` | no | 1000 | Delay between requests |

*At least one of `start_urls` or `search_keywords` required.

Supported URL types: `/watch?v=`, `/shorts/`, `/channel/`, `/@handle`.

```json
{
  "actor": "youtube",
  "input": {
    "start_urls": ["https://www.youtube.com/@ChannelName"],
    "max_results": 5
  }
}
```

---

### instagram

Scrape Instagram profiles and posts. GraphQL API, optional cookies for full access.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `profile_urls` | `[]string` | no* | — | Profile URLs (`instagram.com/username`) |
| `post_urls` | `[]string` | no* | — | Post/Reel URLs (`/p/CODE/`) |
| `max_results` | `int` | no | 20 | Max total results |
| `max_posts_per_user` | `int` | no | 12 | Posts fetched per profile |
| `cookies` | `string` | no | — | Cookie header for authenticated access |
| `request_delay_ms` | `int` | no | 1500 | Delay between requests |

*At least one of `profile_urls` or `post_urls` required.

**Note:** Without `cookies`, profile scraping is limited (may miss posts, private accounts inaccessible). Post URLs without cookies only return minimal metadata (shortcode + URL).

```json
{
  "actor": "instagram",
  "input": {
    "profile_urls": ["https://www.instagram.com/username/"],
    "cookies": "sessionid=abc123; csrftoken=xyz789",
    "max_posts_per_user": 6
  }
}
```

---

### instagram_reel

Scrape Instagram Reels by URL via HTML parsing. Separate from `instagram` actor for targeted reel extraction.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `reel_urls` | `[]string` | **yes** | — | Instagram Reel URLs |
| `max_results` | `int` | no | 20 | Max reels to process |
| `cookies` | `string` | no | — | Cookie header to bypass login wall |
| `request_delay_ms` | `int` | no | 1500 | Delay between requests |

```json
{
  "actor": "instagram_reel",
  "input": {
    "reel_urls": ["https://www.instagram.com/reel/ABC123/"]
  }
}
```

---

### facebook

Scrape Facebook page feeds via `mbasic.facebook.com` (lightweight HTML). Optional cookies for non-public pages.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `page_urls` | `[]string` | **yes** | — | Facebook page URLs |
| `max_results` | `int` | no | 20 | Max posts per page |
| `cookies` | `string` | no | — | Cookie header for non-public pages |
| `request_delay_ms` | `int` | no | 2000 | Delay between requests |

```json
{
  "actor": "facebook",
  "input": {
    "page_urls": ["https://www.facebook.com/PageName"]
  }
}
```

---

### google_search

Scrape Google SERP results. Returns organic results, People Also Ask, and related queries.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `queries` | `[]string` | **yes** | — | Search queries |
| `max_pages_per_query` | `int` | no | 1 | SERP pages to fetch (10 results/page) |
| `country_code` | `string` | no | — | Country code (`US`, `VN`, etc.) |
| `language_code` | `string` | no | `"en"` | Language code |
| `mobile_results` | `bool` | no | false | Use mobile User-Agent |
| `request_delay_ms` | `int` | no | 2000 | Delay between requests |

```json
{
  "actor": "google_search",
  "input": {
    "queries": ["best golang web framework 2026"],
    "max_pages_per_query": 2,
    "country_code": "US"
  }
}
```

---

### google_trends

Fetch Google Trends data: interest over time, related queries, daily trending. Requires session cookie (auto-initialized).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `keywords` | `[]string` | no* | — | Keywords to compare (max 5) |
| `geo` | `string` | no | — | Country code (`US`, `VN`) |
| `time_range` | `string` | no | `"today 12-m"` | Time range (e.g. `"today 3-m"`, `"today 5-y"`) |
| `include_interest_over_time` | `bool` | no | false | Fetch interest timeline |
| `include_related_queries` | `bool` | no | false | Fetch related/rising queries |
| `include_trending_searches` | `bool` | no | false | Fetch daily trending topics |
| `trending_searches_geo` | `string` | no | `geo` or `"US"` | Country for trending |
| `request_delay_ms` | `int` | no | 1500 | Delay between requests |

*`keywords` required if `include_interest_over_time` or `include_related_queries` is true. `include_trending_searches` works without keywords.

```json
{
  "actor": "google_trends",
  "input": {
    "keywords": ["golang", "rust"],
    "geo": "US",
    "include_interest_over_time": true,
    "include_related_queries": true
  }
}
```

---

### ecommerce

Scrape product pages from Amazon, eBay, Walmart, or generic sites. Auto-detects platform from URL.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `start_urls` | `[]string` | **yes** | — | Product page URLs |
| `max_results` | `int` | no | 20 | Max products to process |
| `request_delay_ms` | `int` | no | 2000 | Delay between requests |

Extraction strategy by platform:
- **Amazon:** CSS selectors (`#productTitle`, `.a-price-whole`, etc.)
- **eBay:** CSS selectors (`h1.x-item-title__mainTitle`, etc.)
- **Walmart:** Microdata (`itemprop` attributes)
- **Generic:** JSON-LD structured data → OpenGraph fallback

```json
{
  "actor": "ecommerce",
  "input": {
    "start_urls": [
      "https://www.amazon.com/dp/B0EXAMPLE",
      "https://www.ebay.com/itm/123456"
    ]
  }
}
```

---

### website

General-purpose web crawler with configurable depth, domain restrictions, and content extraction.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `start_urls` | `[]string` | **yes** | — | Seed URLs to begin crawl |
| `max_pages` | `int` | no | — | Max pages to crawl |
| `max_depth` | `int` | no | — | Max link-follow depth |
| `same_domain_only` | `bool` | no | false | Stay on same domain |
| `respect_robots_txt` | `bool` | no | false | Honor robots.txt |
| `use_sitemap` | `bool` | no | false | Discover URLs from sitemap.xml |
| `include_patterns` | `[]string` | no | — | URL patterns to include |
| `exclude_patterns` | `[]string` | no | — | URL patterns to exclude |
| `extract_mode` | `string` | no | `"text"` | `"text"` or `"html"` |
| `request_delay_ms` | `int` | no | — | Delay between requests |

```json
{
  "actor": "website",
  "input": {
    "start_urls": ["https://example.com"],
    "max_pages": 50,
    "max_depth": 3,
    "same_domain_only": true,
    "respect_robots_txt": true,
    "extract_mode": "text"
  }
}
```

---

## 3. Architecture

```
scraper/
├── tool.go                  # Tool interface (Name, Description, Parameters, Execute)
├── actor_registry.go        # Actor factory registry + CreateActor()
├── format.go                # FormatRunForLLM() — output formatting for LLM
├── actor/                   # Core framework
│   ├── actor.go             # Actor + ErrorReporter interfaces
│   ├── runner.go            # RunActor lifecycle (init → execute → cleanup)
│   ├── types.go             # Run, RunStats, Config, Error types
│   ├── errors.go            # Error classification + retry logic
│   └── base_helpers.go      # Shared helpers (Delay, HandleURLError, IncrementRequests)
├── actors/                  # Platform implementations
│   ├── reddit/              twitter/    tiktok/    youtube/
│   ├── instagram/           instagram_reel/        facebook/
│   ├── google_search/       google_trends/         ecommerce/
│   └── website/
├── httpclient/              # Stealth HTTP client (SSRF protection, retry, body limit)
├── stealth/                 # Fingerprint, UA rotation, proxy, request timing
├── extractor/               # HTML extractors (JSON-LD, OpenGraph, CSS selectors, tables)
├── discovery/               # robots.txt, sitemap, URL normalizer, link follower
├── queue/                   # Crawl queue with dedup
└── session/                 # Cookie store, ban detection, session pool
```

### Actor Lifecycle

1. **`Initialize(ctx)`** — Setup (e.g. fetch guest token, session cookie). Most actors are no-op.
2. **`Execute(ctx, stats)`** — Main scraping logic. Returns `[]json.RawMessage`.
3. **`Cleanup()`** — Release resources. Currently no-op for all actors.

### Security

- **SSRF protection:** Dialer blocks private/loopback IPs
- **Body limit:** 10MB max per response
- **Redirect cap:** Max 10 redirects
- **Stealth:** Rotating User-Agent, browser-like headers (Sec-CH-UA, Sec-Fetch-*)
- **Rate limiting:** Configurable per-request delay + exponential backoff on retry

---

## 4. Error Handling

Errors are classified into categories:

| Category | Examples | Retryable |
|----------|----------|-----------|
| `network` | timeout, connection refused | yes |
| `rate-limit` | 429, "too many requests" | yes |
| `auth` | 401, 403, unauthorized | no |
| `parse` | JSON unmarshal, unexpected format | no |
| `validation` | invalid URL, missing parameter | no |

Actors implement `ErrorReporter` to surface per-URL errors in the final run output.
