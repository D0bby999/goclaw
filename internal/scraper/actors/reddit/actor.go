package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// RedditActor scrapes posts and comments from Reddit.
type RedditActor struct {
	input  RedditInput
	client *httpclient.Client
	errors []actor.Error
}

// NewActor constructs a RedditActor by decoding the generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*RedditActor, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var ri RedditInput
	if err := json.Unmarshal(raw, &ri); err != nil {
		return nil, fmt.Errorf("decode input: %w", err)
	}
	if ri.MaxPostsPerSource <= 0 {
		ri.MaxPostsPerSource = 25
	}
	if ri.MaxCommentsPerPost <= 0 {
		ri.MaxCommentsPerPost = 20
	}
	if ri.SortBy == "" {
		ri.SortBy = "hot"
	}
	if ri.RequestDelayMs <= 0 {
		ri.RequestDelayMs = 1000
	}
	return &RedditActor{input: ri, client: client}, nil
}

// Initialize is a no-op; Reddit JSON API needs no auth.
func (a *RedditActor) Initialize(_ context.Context) error { return nil }

// Cleanup is a no-op.
func (a *RedditActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *RedditActor) CollectedErrors() []actor.Error { return a.errors }

// Execute scrapes subreddits, post URLs, and search queries.
func (a *RedditActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	var results []json.RawMessage
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond

	for _, sub := range a.input.Subreddits {
		posts, err := a.scrapeSubreddit(ctx, sub, stats, delay)
		if err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			continue
		}
		for _, p := range posts {
			if raw, err := json.Marshal(p); err == nil {
				results = append(results, raw)
			}
		}
	}

	for _, postURL := range a.input.PostURLs {
		if err := a.validateRedditURL(postURL); err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrValidation))
			continue
		}
		post, err := a.scrapePostURL(ctx, postURL, stats, delay)
		if err != nil {
			actor.HandleURLError(err, postURL, &a.errors, stats)
			continue
		}
		if raw, err := json.Marshal(post); err == nil {
			results = append(results, raw)
		}
	}

	for _, user := range a.input.Usernames {
		posts, err := a.scrapeUser(ctx, user, stats, delay)
		if err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			continue
		}
		for _, p := range posts {
			if raw, err := json.Marshal(p); err == nil {
				results = append(results, raw)
			}
		}
	}

	for _, q := range a.input.SearchQueries {
		posts, err := a.scrapeSearch(ctx, q, stats, delay)
		if err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			continue
		}
		for _, p := range posts {
			if raw, err := json.Marshal(p); err == nil {
				results = append(results, raw)
			}
		}
	}

	return results, nil
}
