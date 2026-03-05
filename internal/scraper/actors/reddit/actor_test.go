package reddit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

func TestRedditActorLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live test in short mode")
	}

	client := httpclient.NewClient(httpclient.WithTimeout(30 * time.Second))

	input := map[string]any{
		"subreddits":          []any{"golang"},
		"sort_by":             "hot",
		"max_posts_per_source": 3,
	}

	a, err := NewActor(input, client)
	if err != nil {
		t.Fatalf("NewActor: %v", err)
	}

	ctx := context.Background()
	if err := a.Initialize(ctx); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	defer a.Cleanup()

	// Direct HTTP test first
	resp, httpErr := client.Get(ctx, "https://www.reddit.com/r/golang/hot.json?limit=1&raw_json=1")
	if httpErr != nil {
		t.Logf("direct GET error: %v", httpErr)
	} else {
		t.Logf("direct GET: status=%d content-type=%s body_len=%d", resp.StatusCode, resp.ContentType, len(resp.Body))
		if len(resp.Body) > 200 {
			t.Logf("body preview: %s", resp.Body[:200])
		}
	}

	stats := &actor.RunStats{StartedAt: time.Now()}
	items, err := a.Execute(ctx, stats)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	t.Logf("items scraped: %d", len(items))
	t.Logf("requests: total=%d failed=%d", stats.RequestsTotal, stats.RequestsFailed)

	if len(items) == 0 {
		t.Fatal("expected at least 1 item, got 0")
	}

	// Print first item
	var post RedditPost
	if err := json.Unmarshal(items[0], &post); err != nil {
		t.Fatalf("unmarshal post: %v", err)
	}
	t.Logf("first post: [%s] %s (score: %d)", post.Subreddit, post.Title, post.Score)

	// Check errors
	errs := a.CollectedErrors()
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("error: [%s] %s", e.Category, e.Message)
		}
	}
}
