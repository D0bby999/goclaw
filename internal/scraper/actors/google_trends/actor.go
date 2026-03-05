package google_trends

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// GoogleTrendsActor scrapes Google Trends data.
type GoogleTrendsActor struct {
	input  GoogleTrendsInput
	client *httpclient.Client
	cookie string
	errors []actor.Error
}

// NewActor constructs a GoogleTrendsActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*GoogleTrendsActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed GoogleTrendsInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.TimeRange == "" {
		typed.TimeRange = "today 12-m"
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 1500
	}
	return &GoogleTrendsActor{input: typed, client: client}, nil
}

// Initialize fetches a session cookie from trends.google.com.
func (a *GoogleTrendsActor) Initialize(ctx context.Context) error {
	cookie, err := initSession(ctx, a.client)
	if err != nil {
		return fmt.Errorf("init trends session: %w", err)
	}
	a.cookie = cookie
	return nil
}

// Cleanup is a no-op.
func (a *GoogleTrendsActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *GoogleTrendsActor) CollectedErrors() []actor.Error { return a.errors }

// Execute fetches interest-over-time, related queries, and/or trending searches.
func (a *GoogleTrendsActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	result := GoogleTrendsResult{
		Type:     "trends",
		Keywords: a.input.Keywords,
	}

	if len(a.input.Keywords) > 0 && (a.input.IncludeInterestOverTime || a.input.IncludeRelatedQueries) {
		actor.IncrementRequests(stats)
		widgets, err := fetchExplore(ctx, a.client, a.input.Keywords, a.input.Geo, a.input.TimeRange, a.cookie)
		if err != nil {
			actor.HandleURLError(err, trendsExploreURL, &a.errors, stats)
		} else {
			if a.input.IncludeInterestOverTime {
				if wi, ok := widgets["TIMESERIES"]; ok {
					if err := actor.Delay(ctx, delay); err != nil {
						return results, err
					}
					actor.IncrementRequests(stats)
					points, err := fetchInterestOverTime(ctx, a.client, wi, a.cookie)
					if err == nil {
						result.InterestOverTime = points
					}
				}
			}

			if a.input.IncludeRelatedQueries {
				if wi, ok := widgets["RELATED_QUERIES"]; ok {
					if err := actor.Delay(ctx, delay); err != nil {
						return results, err
					}
					actor.IncrementRequests(stats)
					related, err := fetchRelatedQueries(ctx, a.client, wi, a.cookie)
					if err == nil {
						result.RelatedQueries = related
					}
				}
			}
		}
	}

	if a.input.IncludeTrendingSearches {
		geo := a.input.TrendingSearchesGeo
		if geo == "" {
			geo = a.input.Geo
		}
		if geo == "" {
			geo = "US"
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
		actor.IncrementRequests(stats)
		trending, err := fetchDailyTrending(ctx, a.client, geo, a.cookie)
		if err == nil {
			result.TrendingSearches = trending
		}
	}

	b, _ := json.Marshal(result)
	results = append(results, b)

	return results, nil
}
