package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// TwitterActor scrapes profiles and tweets from Twitter/X via the FxTwitter public API.
type TwitterActor struct {
	input  TwitterInput
	client *httpclient.Client
	errors []actor.Error
}

// NewActor constructs a TwitterActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*TwitterActor, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var ti TwitterInput
	if err := json.Unmarshal(raw, &ti); err != nil {
		return nil, fmt.Errorf("decode input: %w", err)
	}
	if ti.MaxTweetsPerUser <= 0 {
		ti.MaxTweetsPerUser = 20
	}
	if ti.RequestDelayMs <= 0 {
		ti.RequestDelayMs = 1000
	}
	return &TwitterActor{input: ti, client: client}, nil
}

// Initialize is a no-op; FxTwitter API needs no auth.
func (a *TwitterActor) Initialize(_ context.Context) error { return nil }

// Cleanup is a no-op.
func (a *TwitterActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *TwitterActor) CollectedErrors() []actor.Error { return a.errors }

// Execute scrapes handles, tweet URLs, and search queries.
func (a *TwitterActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	var results []json.RawMessage
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond

	for _, handle := range a.input.Handles {
		normalized := NormalizeHandle(handle)
		actor.IncrementRequests(stats)
		profile, err := a.fetchProfile(ctx, normalized)
		if err != nil {
			actor.HandleURLError(err, handle, &a.errors, stats)
			continue
		}
		if raw, err := json.Marshal(profile); err == nil {
			results = append(results, raw)
		}
		// Also fetch latest tweets from this user.
		if a.input.MaxTweetsPerUser > 0 {
			if err := actor.Delay(ctx, delay); err != nil {
				return results, err
			}
			actor.IncrementRequests(stats)
			tweets, err := a.fetchUserTweets(ctx, normalized, a.input.MaxTweetsPerUser)
			if err != nil {
				a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			}
			for _, t := range tweets {
				if raw, err := json.Marshal(t); err == nil {
					results = append(results, raw)
				}
			}
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	for _, tweetURL := range a.input.TweetURLs {
		urlType, id := ResolveURL(tweetURL)
		if urlType != "tweet" || id == "" {
			a.errors = append(a.errors, actor.NewError("not a valid tweet URL: "+tweetURL, actor.ErrValidation))
			continue
		}
		actor.IncrementRequests(stats)
		tweet, err := a.fetchTweet(ctx, id)
		if err != nil {
			actor.HandleURLError(err, tweetURL, &a.errors, stats)
			continue
		}
		if raw, err := json.Marshal(tweet); err == nil {
			results = append(results, raw)
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	for _, q := range a.input.SearchQueries {
		actor.IncrementRequests(stats)
		tweets, err := a.searchTweets(ctx, q, a.input.MaxResults)
		if err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			continue
		}
		for _, t := range tweets {
			if raw, err := json.Marshal(t); err == nil {
				results = append(results, raw)
			}
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}
