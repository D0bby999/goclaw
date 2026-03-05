# Project Changelog

All notable changes to GoClaw project. Format: YYYY-MM-DD | Type | Brief description.

## 2026

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
