package scraper

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/stealth"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

// ScraperTool implements tools.Tool for the composite scraper.
type ScraperTool struct{}

// NewScraperTool creates the scraper tool.
func NewScraperTool() *ScraperTool {
	return &ScraperTool{}
}

func (t *ScraperTool) Name() string { return "scraper" }

func (t *ScraperTool) Description() string {
	return `Scrape websites and social media platforms.

Actor input formats:
- reddit: {"subreddits":["golang"], "usernames":["spez"], "post_urls":["https://reddit.com/r/..."], "search_queries":["query"], "include_comments":true, "max_comments_per_post":10, "max_posts_per_source":25, "sort_by":"hot|new|top", "time_filter":"day|week|month|year|all"}
- twitter: {"handles":["elonmusk"], "tweet_urls":["https://x.com/user/status/123"], "search_queries":["query"], "max_tweets_per_user":10, "sort_by":"top|latest", "time_filter":"day|week|month|year|all"}
- youtube: {"video_urls":["https://youtube.com/watch?v=..."], "search_queries":["query"], "channel_urls":["..."], "max_results":10}
- tiktok: {"video_urls":["https://tiktok.com/@user/video/..."], "usernames":["user"], "search_queries":["golang"], "max_results":10}
- instagram: {"usernames":["user"], "post_urls":["https://instagram.com/p/..."]}
- instagram_reel: {"reel_urls":["https://instagram.com/reel/..."]}
- facebook: {"post_urls":["https://facebook.com/..."], "page_urls":["..."]}
- google_search: {"queries":["search term"], "max_results":10}
- google_trends: {"keywords":["keyword"], "geo":"US", "timeframe":"today 12-m"}
- ecommerce: {"product_urls":["https://amazon.com/..."]}
- website: {"urls":["https://example.com"], "max_pages":5}

Set 'actor' to the platform name and 'input' to the matching format above.`
}

func (t *ScraperTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"actor": map[string]interface{}{
				"type":        "string",
				"description": "Platform to scrape",
				"enum": []string{
					"reddit", "twitter", "tiktok", "youtube",
					"instagram", "instagram_reel", "facebook",
					"google_search", "google_trends", "ecommerce", "website",
				},
			},
			"input": map[string]interface{}{
				"type":        "object",
				"description": "Actor-specific input parameters. See actor documentation for required fields.",
			},
		},
		"required": []string{"actor", "input"},
	}
}

func (t *ScraperTool) Execute(ctx context.Context, args map[string]interface{}) *tools.Result {
	actorName, _ := args["actor"].(string)
	if actorName == "" {
		return tools.ErrorResult("missing required parameter 'actor'")
	}

	input, _ := args["input"].(map[string]interface{})
	if input == nil {
		return tools.ErrorResult("missing required parameter 'input'")
	}


	// Create a stealth HTTP client with optional proxy support.
	opts := []httpclient.Option{httpclient.WithTimeout(5 * time.Minute)}
	if rot := loadProxiesFromEnv(); rot != nil {
		opts = append(opts, httpclient.WithProxy(rot))
	}
	client := httpclient.NewClient(opts...)

	a, err := CreateActor(actorName, input, client)
	if err != nil {
		return tools.ErrorResult(err.Error())
	}

	cfg := actor.DefaultConfig()
	run := actor.RunActor(ctx, a, cfg)

	formatted := FormatRunForLLM(run, actorName)
	return tools.NewResult(formatted)
}

// loadProxiesFromEnv reads SCRAPER_PROXY_URLS (comma-separated) and returns
// a ProxyRotator, or nil if no proxies are configured.
func loadProxiesFromEnv() *stealth.ProxyRotator {
	raw := os.Getenv("SCRAPER_PROXY_URLS")
	if raw == "" {
		return nil
	}
	rot := stealth.NewProxyRotator()
	for _, u := range strings.Split(raw, ",") {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if err := rot.Add(u); err != nil {
			slog.Warn("scraper.proxy: invalid proxy URL", "url", u, "error", err)
			continue
		}
	}
	if rot.Count() == 0 {
		return nil
	}
	slog.Info("scraper.proxy: loaded proxies", "count", rot.Count())
	return rot
}
