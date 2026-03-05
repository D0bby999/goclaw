package website

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// WebsiteActor implements actor.Actor for website crawling.
type WebsiteActor struct {
	input  WebsiteInput
	client *httpclient.Client
}

// NewActor constructs a WebsiteActor by decoding the generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*WebsiteActor, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	var wi WebsiteInput
	if err := json.Unmarshal(raw, &wi); err != nil {
		return nil, fmt.Errorf("unmarshal WebsiteInput: %w", err)
	}

	if len(wi.StartURLs) == 0 {
		return nil, fmt.Errorf("start_urls must not be empty")
	}

	return &WebsiteActor{input: wi, client: client}, nil
}

// Initialize satisfies actor.Actor; no setup needed.
func (a *WebsiteActor) Initialize(_ context.Context) error { return nil }

// Execute runs the crawl and returns results as raw JSON messages.
func (a *WebsiteActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	orch, err := NewOrchestrator(a.input, a.client)
	if err != nil {
		return nil, fmt.Errorf("create orchestrator: %w", err)
	}

	pages, err := orch.Crawl(ctx, stats)
	if err != nil {
		return nil, fmt.Errorf("crawl: %w", err)
	}

	results := make([]json.RawMessage, 0, len(pages))
	for _, p := range pages {
		b, merr := json.Marshal(p)
		if merr != nil {
			continue
		}
		results = append(results, json.RawMessage(b))
	}

	return results, nil
}

// Cleanup satisfies actor.Actor; nothing to release.
func (a *WebsiteActor) Cleanup() {}
