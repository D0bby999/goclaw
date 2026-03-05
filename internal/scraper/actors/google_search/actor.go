package google_search

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

const googleSearchBase = "https://www.google.com/search"

// GoogleSearchActor scrapes Google SERP results using headless Chrome.
type GoogleSearchActor struct {
	input   GoogleSearchInput
	client  *httpclient.Client
	errors  []actor.Error
	browser *rod.Browser
}

// NewActor constructs a GoogleSearchActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*GoogleSearchActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed GoogleSearchInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxPagesPerQuery <= 0 {
		typed.MaxPagesPerQuery = 1
	}
	if typed.LanguageCode == "" {
		typed.LanguageCode = "en"
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 2000
	}
	return &GoogleSearchActor{input: typed, client: client}, nil
}

// Initialize launches headless Chrome with stealth settings.
func (a *GoogleSearchActor) Initialize(_ context.Context) error {
	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	l := launcher.New().Bin(chromePath).Headless(true).
		Set("disable-gpu").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-features", "AutomationControlled")

	if a.input.ProxyURL != "" {
		l = l.Proxy(a.input.ProxyURL)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch chrome: %w", err)
	}
	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return fmt.Errorf("connect chrome: %w", err)
	}
	a.browser = b
	return nil
}

// Cleanup closes the browser.
func (a *GoogleSearchActor) Cleanup() {
	if a.browser != nil {
		_ = a.browser.Close()
	}
}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *GoogleSearchActor) CollectedErrors() []actor.Error { return a.errors }

// Execute runs each query across configured pages and returns SERP results.
func (a *GoogleSearchActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for _, query := range a.input.Queries {
		for page := 0; page < a.input.MaxPagesPerQuery; page++ {
			start := page * 10
			searchURL := a.buildSearchURL(query, start)

			actor.IncrementRequests(stats)
			html, err := a.fetchSERP(ctx, searchURL)
			if err != nil {
				actor.HandleURLError(err, searchURL, &a.errors, stats)
				break
			}

			serpResult := ParseGoogleSERP(html, query)
			serpResult.SearchURL = searchURL

			b, _ := json.Marshal(serpResult)
			results = append(results, b)

			if !serpResult.HasNextPage {
				break
			}

			if err := actor.Delay(ctx, delay); err != nil {
				return results, err
			}
		}

		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}

func (a *GoogleSearchActor) buildSearchURL(query string, start int) string {
	params := url.Values{}
	params.Set("q", query)
	params.Set("hl", a.input.LanguageCode)
	if a.input.CountryCode != "" {
		params.Set("gl", strings.ToLower(a.input.CountryCode))
	}
	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}
	return googleSearchBase + "?" + params.Encode()
}

// fetchSERP navigates to a Google search URL with headless Chrome and returns rendered HTML.
func (a *GoogleSearchActor) fetchSERP(ctx context.Context, searchURL string) (string, error) {
	u, err := url.Parse(searchURL)
	if err != nil || !strings.HasSuffix(u.Hostname(), "google.com") {
		return "", fmt.Errorf("invalid search URL: %s", searchURL)
	}

	page, err := a.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "", fmt.Errorf("create page: %w", err)
	}
	defer page.Close()
	page = page.Context(ctx)

	// Random viewport to avoid fingerprinting.
	widths := []int{1280, 1366, 1440, 1536, 1920}
	heights := []int{720, 768, 900, 1024, 1080}
	w, h := widths[rand.IntN(len(widths))], heights[rand.IntN(len(heights))]
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h})

	// Stealth: hide navigator.webdriver and other automation signals.
	page.MustEvalOnNewDocument(`
		Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
		Object.defineProperty(navigator, 'languages', {get: () => ['en-US', 'en']});
		Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
		window.chrome = {runtime: {}};
	`)

	if err := page.Navigate(searchURL); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}
	_ = page.WaitLoad()

	// Wait for search results to render.
	time.Sleep(2 * time.Second)

	html, err := page.HTML()
	if err != nil {
		return "", fmt.Errorf("get html: %w", err)
	}
	return html, nil
}
