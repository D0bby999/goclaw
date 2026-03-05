package facebook

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

// FacebookActor scrapes Facebook pages via headless browser with JS-level fetch interception.
type FacebookActor struct {
	input   FacebookInput
	client  *httpclient.Client
	errors  []actor.Error
	browser *rod.Browser
}

// NewActor constructs a FacebookActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*FacebookActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed FacebookInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxResults <= 0 {
		typed.MaxResults = 20
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 2000
	}
	return &FacebookActor{input: typed, client: client}, nil
}

// Initialize launches headless Chrome and injects Facebook cookies.
func (a *FacebookActor) Initialize(ctx context.Context) error {
	cookies := strings.TrimSpace(a.input.Cookies)
	if cookies == "" {
		return fmt.Errorf("facebook requires cookies (c_user + xs)")
	}

	chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	l := launcher.New().Bin(chromePath).Headless(true).
		Set("disable-gpu").
		Set("no-first-run").
		Set("no-default-browser-check")
	controlURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch chrome: %w", err)
	}

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return fmt.Errorf("connect chrome: %w", err)
	}
	a.browser = b

	// Random viewport.
	widths := []int{1280, 1366, 1440, 1536, 1920}
	heights := []int{720, 768, 900, 1024, 1080}
	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	w, h := widths[rand.IntN(len(widths))], heights[rand.IntN(len(heights))]
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h})

	// Parse and inject cookies.
	expire := float64(time.Now().Add(365 * 24 * time.Hour).Unix())
	cookieParams := parseCookieString(cookies, expire)
	if len(cookieParams) == 0 {
		return fmt.Errorf("no valid cookies parsed from input")
	}
	if err := b.SetCookies(cookieParams); err != nil {
		return fmt.Errorf("set cookies: %w", err)
	}
	_ = page.Close()
	return nil
}

// Cleanup closes the browser.
func (a *FacebookActor) Cleanup() {
	if a.browser != nil {
		_ = a.browser.Close()
	}
}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *FacebookActor) CollectedErrors() []actor.Error { return a.errors }

// Execute iterates page URLs, navigates with browser, intercepts GraphQL responses.
func (a *FacebookActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for _, pageURL := range a.input.PageURLs {
		actor.IncrementRequests(stats)
		posts, err := a.scrapePage(ctx, pageURL, a.input.MaxResults)
		if err != nil {
			actor.HandleURLError(err, pageURL, &a.errors, stats)
		} else {
			for _, p := range posts {
				b, _ := json.Marshal(p)
				results = append(results, b)
			}
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}

// fetchInterceptScript is injected before navigation to monkey-patch window.fetch.
// It captures all /api/graphql response bodies and stores them in window.__fbGraphQLResponses.
const fetchInterceptScript = `
	window.__fbGraphQLResponses = [];

	// Patch fetch
	const origFetch = window.fetch;
	window.fetch = async function(...args) {
		const response = await origFetch.apply(this, args);
		try {
			const url = (typeof args[0] === 'string') ? args[0] : args[0]?.url || '';
			if (url.includes('/api/graphql')) {
				const clone = response.clone();
				clone.text().then(text => {
					window.__fbGraphQLResponses.push(text);
				}).catch(() => {});
			}
		} catch(e) {}
		return response;
	};

	// Patch XMLHttpRequest
	const origOpen = XMLHttpRequest.prototype.open;
	const origSend = XMLHttpRequest.prototype.send;
	XMLHttpRequest.prototype.open = function(method, url, ...rest) {
		this.__url = url;
		return origOpen.call(this, method, url, ...rest);
	};
	XMLHttpRequest.prototype.send = function(...args) {
		if (this.__url && this.__url.includes('/api/graphql')) {
			this.addEventListener('load', function() {
				try {
					window.__fbGraphQLResponses.push(this.responseText);
				} catch(e) {}
			});
		}
		return origSend.apply(this, args);
	};
`

// scrapePage navigates to a Facebook page, intercepts GraphQL via JS fetch monkey-patch.
func (a *FacebookActor) scrapePage(ctx context.Context, pageURL string, maxPosts int) ([]FacebookPost, error) {
	pageID := extractPageID(pageURL)
	collector := newFBCollector(pageURL, pageID)

	page, err := a.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()
	page = page.Context(ctx)

	// Inject fetch interceptor BEFORE navigation to capture initial feed GraphQL.
	// Use MustEvalOnNewDocument to ensure it runs on every new document (including navigations).
	page.MustEvalOnNewDocument(fetchInterceptScript)

	// Navigate to the page.
	if err := page.Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
	_ = page.WaitLoad()
	time.Sleep(5 * time.Second)

	// Check for login redirect.
	info, _ := page.Info()
	if info != nil {
		u := info.URL
		if strings.Contains(u, "/login") || strings.Contains(u, "/checkpoint") {
			return nil, fmt.Errorf("session invalid: redirected to %s", u)
		}
	}

	// Drain initial captured responses.
	collector.drainFromPage(page)

	// Scroll to load more posts.
	emptyScrolls := 0
	for collector.postCount() < maxPosts && emptyScrolls < 3 {
		select {
		case <-ctx.Done():
			goto done
		default:
		}
		before := collector.postCount()
		scrollH := 800 + rand.IntN(600)
		_, _ = page.Eval(fmt.Sprintf("window.scrollBy(0, %d)", scrollH))

		scrollDelay := time.Duration(a.input.RequestDelayMs)*time.Millisecond + time.Duration(rand.IntN(1000))*time.Millisecond
		select {
		case <-ctx.Done():
			goto done
		case <-time.After(scrollDelay):
		}

		// Drain newly captured responses.
		collector.drainFromPage(page)

		if collector.postCount() == before {
			emptyScrolls++
		} else {
			emptyScrolls = 0
		}
	}

done:
	return collector.posts(maxPosts), nil
}

// extractPageID extracts the page identifier from a Facebook URL.
func extractPageID(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	parts := strings.Split(rawURL, "facebook.com/")
	if len(parts) < 2 {
		return ""
	}
	seg := strings.Split(parts[1], "/")[0]
	seg = strings.Split(seg, "?")[0]
	return seg
}

// parseCookieString parses "key=value; key2=value2" into Rod cookie params.
func parseCookieString(raw string, expire float64) []*proto.NetworkCookieParam {
	var params []*proto.NetworkCookieParam
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		params = append(params, &proto.NetworkCookieParam{
			Name:     strings.TrimSpace(parts[0]),
			Value:    strings.TrimSpace(parts[1]),
			Domain:   ".facebook.com",
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
			Expires:  proto.TimeSinceEpoch(expire),
		})
	}
	return params
}

// fbCollector accumulates Facebook posts from intercepted GraphQL responses.
type fbCollector struct {
	mu       sync.Mutex
	postMap  map[string]FacebookPost
	inputURL string
	pageID   string
}

func newFBCollector(inputURL, pageID string) *fbCollector {
	return &fbCollector{
		postMap:  make(map[string]FacebookPost),
		inputURL: inputURL,
		pageID:   pageID,
	}
}

// drainFromPage reads captured GraphQL responses from window.__fbGraphQLResponses.
func (c *fbCollector) drainFromPage(page *rod.Page) {
	// Read and clear captured responses from the page.
	res, err := page.Eval(`() => {
		const data = window.__fbGraphQLResponses || [];
		window.__fbGraphQLResponses = [];
		return JSON.stringify(data);
	}`)
	if err != nil {
		return
	}

	var responses []string
	if err := json.Unmarshal([]byte(res.Value.String()), &responses); err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, text := range responses {
		// FB streams multiple JSON objects separated by newlines per response.
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "{") || !gjson.Valid(line) {
				continue
			}
			posts := parsePostsFromGraphQL(line, c.inputURL, c.pageID)
			for _, p := range posts {
				if p.ID != "" {
					c.postMap[p.ID] = p
				}
			}
		}
	}
}

func (c *fbCollector) postCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.postMap)
}

func (c *fbCollector) posts(limit int) []FacebookPost {
	c.mu.Lock()
	defer c.mu.Unlock()
	posts := make([]FacebookPost, 0, len(c.postMap))
	for _, p := range c.postMap {
		posts = append(posts, p)
	}
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts
}
