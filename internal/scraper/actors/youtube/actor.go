package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// YouTubeActor scrapes YouTube videos, channels, and search results.
type YouTubeActor struct {
	input  YouTubeInput
	client *httpclient.Client
	errors []actor.Error
}

// NewActor constructs a YouTubeActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*YouTubeActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed YouTubeInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxResults <= 0 {
		typed.MaxResults = 20
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 1000
	}
	return &YouTubeActor{input: typed, client: client}, nil
}

// Initialize is a no-op for YouTube.
func (a *YouTubeActor) Initialize(_ context.Context) error { return nil }

// Cleanup is a no-op for YouTube.
func (a *YouTubeActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *YouTubeActor) CollectedErrors() []actor.Error { return a.errors }

// Execute iterates start URLs, scrapes channel/video data, then searches if keywords set.
func (a *YouTubeActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for _, rawURL := range a.input.StartURLs {
		actor.IncrementRequests(stats)
		items, err := a.dispatch(ctx, rawURL)
		if err != nil {
			actor.HandleURLError(err, rawURL, &a.errors, stats)
			continue
		}
		results = append(results, items...)

		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	if a.input.SearchKeywords != "" {
		actor.IncrementRequests(stats)
		videos, err := searchVideos(ctx, a.client, a.input.SearchKeywords, a.input.MaxResults)
		if err == nil {
			for _, v := range videos {
				b, _ := json.Marshal(v)
				results = append(results, b)
			}
		}
	}

	return results, nil
}

func (a *YouTubeActor) dispatch(ctx context.Context, rawURL string) ([]json.RawMessage, error) {
	urlType, id := ResolveURL(rawURL)
	switch urlType {
	case "channel":
		return a.scrapeChannel(ctx, id)
	case "video", "short":
		return a.scrapeVideo(ctx, id)
	default:
		return nil, fmt.Errorf("unsupported URL type: %s (%s)", urlType, rawURL)
	}
}

func (a *YouTubeActor) scrapeChannel(ctx context.Context, channelID string) ([]json.RawMessage, error) {
	ch, videos, err := browseChannel(ctx, a.client, channelID)
	if err != nil {
		return nil, err
	}

	var results []json.RawMessage
	b, _ := json.Marshal(ch)
	results = append(results, b)

	limit := a.input.MaxResults
	for i, v := range videos {
		if i >= limit {
			break
		}
		vb, _ := json.Marshal(v)
		results = append(results, vb)
	}
	return results, nil
}

func (a *YouTubeActor) scrapeVideo(ctx context.Context, videoID string) ([]json.RawMessage, error) {
	video, err := getVideoInfo(ctx, a.client, videoID)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(video)
	return []json.RawMessage{b}, nil
}
