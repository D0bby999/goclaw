package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// TikTokActor scrapes videos and profiles from TikTok via tikwm.com API.
type TikTokActor struct {
	input  TikTokInput
	client *httpclient.Client
	errors []actor.Error
}

// NewActor constructs a TikTokActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*TikTokActor, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var ti TikTokInput
	if err := json.Unmarshal(raw, &ti); err != nil {
		return nil, fmt.Errorf("decode input: %w", err)
	}
	if ti.MaxResults <= 0 {
		ti.MaxResults = 20
	}
	if ti.RequestDelayMs <= 0 {
		ti.RequestDelayMs = 1500
	}
	return &TikTokActor{input: ti, client: client}, nil
}

// Initialize is a no-op.
func (a *TikTokActor) Initialize(_ context.Context) error { return nil }

// Cleanup is a no-op.
func (a *TikTokActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *TikTokActor) CollectedErrors() []actor.Error { return a.errors }

// Execute scrapes video URLs, usernames, and search queries.
func (a *TikTokActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	var results []json.RawMessage
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond

	for _, videoURL := range a.input.VideoURLs {
		actor.IncrementRequests(stats)
		video, err := fetchVideo(ctx, a.client, videoURL)
		if err != nil {
			actor.HandleURLError(err, videoURL, &a.errors, stats)
			continue
		}
		if raw, err := json.Marshal(video); err == nil {
			results = append(results, raw)
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	for _, username := range a.input.Usernames {
		username = strings.TrimPrefix(username, "@")
		actor.IncrementRequests(stats)
		profile, err := fetchUserInfo(ctx, a.client, username)
		if err != nil {
			actor.HandleURLError(err, username, &a.errors, stats)
			continue
		}
		if raw, err := json.Marshal(profile); err == nil {
			results = append(results, raw)
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	for _, q := range a.input.SearchQueries {
		actor.IncrementRequests(stats)
		videos, err := searchVideos(ctx, a.client, q, a.input.MaxResults)
		if err != nil {
			a.errors = append(a.errors, actor.NewError(err.Error(), actor.ErrNetwork))
			continue
		}
		for _, v := range videos {
			if raw, err := json.Marshal(v); err == nil {
				results = append(results, raw)
			}
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}
