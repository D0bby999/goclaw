package instagram

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

// InstagramActor scrapes Instagram profiles and posts via headless browser
// with GraphQL response interception (same approach as TS Puppeteer version).
type InstagramActor struct {
	input   InstagramInput
	client  *httpclient.Client // kept for interface compat, browser used instead
	errors  []actor.Error
	browser *rod.Browser
}

// NewActor constructs an InstagramActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*InstagramActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed InstagramInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxResults <= 0 {
		typed.MaxResults = 20
	}
	if typed.MaxPostsPerUser <= 0 {
		typed.MaxPostsPerUser = 12
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 2000
	}
	return &InstagramActor{input: typed, client: client}, nil
}

// Initialize launches a headless Chrome browser and injects the session cookie.
func (a *InstagramActor) Initialize(ctx context.Context) error {
	sessionID := strings.TrimSpace(a.input.Cookies)
	if sessionID == "" {
		return fmt.Errorf("instagram requires cookies (sessionid). Configure cookies in Settings > Scrapers")
	}
	sessionID = strings.TrimPrefix(sessionID, "sessionid=")

	// Prefer system Chrome (matches TS Puppeteer behaviour and avoids
	// fingerprint differences with Rod's bundled Chromium).
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

	// Set random viewport to avoid bot detection.
	widths := []int{1280, 1366, 1440, 1536, 1920}
	heights := []int{720, 768, 900, 1024, 1080}
	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	w, h := widths[rand.IntN(len(widths))], heights[rand.IntN(len(heights))]
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h})

	// Inject sessionid cookie before any navigation.
	expire := float64(time.Now().Add(365 * 24 * time.Hour).Unix())
	err = b.SetCookies([]*proto.NetworkCookieParam{{
		Name:     "sessionid",
		Value:    sessionID,
		Domain:   ".instagram.com",
		Path:     "/",
		Secure:   true,
		HTTPOnly: true,
		Expires:  proto.TimeSinceEpoch(expire),
	}})
	if err != nil {
		return fmt.Errorf("set cookie: %w", err)
	}
	_ = page.Close()
	return nil
}

// Cleanup closes the browser.
func (a *InstagramActor) Cleanup() {
	if a.browser != nil {
		_ = a.browser.Close()
	}
}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *InstagramActor) CollectedErrors() []actor.Error { return a.errors }

// Execute scrapes profiles and posts by navigating pages and intercepting GraphQL.
func (a *InstagramActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for _, profileURL := range a.input.ProfileURLs {
		username := extractUsername(profileURL)
		if username == "" {
			continue
		}
		actor.IncrementRequests(stats)
		profile, posts, err := a.scrapeProfile(ctx, profileURL, a.input.MaxPostsPerUser)
		if err != nil {
			actor.HandleURLError(err, profileURL, &a.errors, stats)
		} else {
			if profile != nil {
				b, _ := json.Marshal(profile)
				results = append(results, b)
			}
			for _, p := range posts {
				pb, _ := json.Marshal(p)
				results = append(results, pb)
			}
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	for _, postURL := range a.input.PostURLs {
		shortcode := extractShortcode(postURL)
		if shortcode == "" {
			continue
		}
		actor.IncrementRequests(stats)
		post, err := a.scrapePost(ctx, postURL)
		if err != nil {
			actor.HandleURLError(err, postURL, &a.errors, stats)
		} else if post != nil {
			b, _ := json.Marshal(post)
			results = append(results, b)
		}
		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}

// scrapeProfile navigates to a profile URL, intercepts GraphQL responses,
// and scrolls to load posts. Returns profile + posts.
func (a *InstagramActor) scrapeProfile(ctx context.Context, profileURL string, maxPosts int) (*InstagramProfile, []InstagramPost, error) {
	collector := newGraphQLCollector()

	page, err := a.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, nil, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()

	// Set per-page timeout from context.
	page = page.Context(ctx)

	// Enable network events and set up GraphQL response listener.
	_ = proto.NetworkEnable{}.Call(page)
	collector.listenPage(page)

	// Navigate to profile.
	if err := page.Navigate(profileURL); err != nil {
		return nil, nil, fmt.Errorf("navigate: %w", err)
	}
	// Wait for DOM load then give time for GraphQL XHRs to fire.
	_ = page.WaitLoad()
	time.Sleep(5 * time.Second)

	// Check for login redirect (session invalid).
	info, _ := page.Info()
	if info != nil {
		u := info.URL
		if strings.Contains(u, "/accounts/login") || strings.Contains(u, "/challenge/") {
			return nil, nil, fmt.Errorf("session invalid: redirected to %s", u)
		}
	}

	// Scroll to load more posts.
	emptyScrolls := 0
	for collector.postCount() < maxPosts && emptyScrolls < 3 {
		select {
		case <-ctx.Done():
			goto done
		default:
		}
		before := collector.postCount()
		scrollH := 600 + rand.IntN(800)
		_, _ = page.Eval(fmt.Sprintf("window.scrollBy(0, %d)", scrollH))

		scrollDelay := time.Duration(a.input.RequestDelayMs)*time.Millisecond + time.Duration(rand.IntN(1000))*time.Millisecond
		select {
		case <-ctx.Done():
			goto done
		case <-time.After(scrollDelay):
		}

		if collector.postCount() == before {
			emptyScrolls++
		} else {
			emptyScrolls = 0
		}
	}

done:
	profile := collector.profile()
	posts := collector.posts(maxPosts)
	return profile, posts, nil
}

// scrapePost navigates to a post URL then calls the Instagram media info API
// from within the page context (authenticated via httpOnly session cookie).
func (a *InstagramActor) scrapePost(ctx context.Context, postURL string) (*InstagramPost, error) {
	shortcode := extractShortcode(postURL)
	if shortcode == "" {
		return nil, fmt.Errorf("could not extract shortcode from URL: %s", postURL)
	}

	page, err := a.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()

	page = page.Context(ctx)

	// Navigate to the post URL first to establish instagram.com origin.
	// This allows same-origin fetch with the httpOnly session cookie.
	if err := page.Navigate(postURL); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
	_ = page.WaitLoad()
	time.Sleep(3 * time.Second)

	// Check for login redirect.
	info, _ := page.Info()
	if info != nil {
		u := info.URL
		if strings.Contains(u, "/accounts/login") || strings.Contains(u, "/challenge/") {
			return nil, fmt.Errorf("session invalid: redirected to %s", u)
		}
	}

	return fetchPostFromAPI(page, shortcode)
}

// igMediaInfoScript is the JS template used to call /api/v1/media/{id}/info/
// from within the page context. Uses the httpOnly session cookie automatically.
const igMediaInfoScript = `async () => {
	const r = await fetch('/api/v1/media/%s/info/', {
		headers: {'X-IG-App-ID': '936619743392459'},
		credentials: 'include',
	});
	const text = await r.text();
	return JSON.stringify({status: r.status, body: text});
}`

// fetchPostFromAPI calls the Instagram private API to retrieve post data for
// the given shortcode. The page must already be navigated to instagram.com so
// the httpOnly session cookie is sent automatically with the same-origin fetch.
func fetchPostFromAPI(page *rod.Page, shortcode string) (*InstagramPost, error) {
	mediaID := shortcodeToMediaID(shortcode)
	script := fmt.Sprintf(igMediaInfoScript, mediaID)

	res, err := page.Eval("() => (" + script + ")()")
	if err != nil {
		return nil, fmt.Errorf("fetch media info: %w", err)
	}

	result := res.Value.String()
	status := gjson.Get(result, "status").Int()
	body := gjson.Get(result, "body").String()

	if status != 200 {
		msg := gjson.Get(body, "message").String()
		if msg == "" {
			msg = "non-200 response"
		}
		return nil, fmt.Errorf("media info API: %s (status %d)", msg, status)
	}

	// Parse the first item from the items array.
	item := gjson.Get(body, "items.0")
	if !item.Exists() {
		return nil, fmt.Errorf("media info API: no items in response")
	}

	post := parsePostNode(item)
	if post.ID == "" && post.Shortcode == "" {
		return nil, fmt.Errorf("media info API: could not parse post node")
	}
	return &post, nil
}

// shortcodeToMediaID converts an Instagram shortcode to its numeric media ID
// using the Instagram base64url alphabet (A-Z a-z 0-9 - _).
func shortcodeToMediaID(shortcode string) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var id uint64
	for _, c := range shortcode {
		idx := strings.IndexRune(alpha, c)
		if idx < 0 {
			continue
		}
		id = id*64 + uint64(idx)
	}
	return fmt.Sprintf("%d", id)
}

// graphQLCollector accumulates profile and post data from intercepted GraphQL responses.
type graphQLCollector struct {
	mu       sync.Mutex
	prof     *InstagramProfile
	postMap  map[string]InstagramPost // dedup by ID
}

func newGraphQLCollector() *graphQLCollector {
	return &graphQLCollector{postMap: make(map[string]InstagramPost)}
}

// listenPage sets up a Rod event listener that intercepts GraphQL responses.
func (c *graphQLCollector) listenPage(page *rod.Page) {
	go page.EachEvent(
		func(e *proto.NetworkResponseReceived) {
			url := e.Response.URL
			if !strings.Contains(url, "/graphql") {
				return
			}
			// Fetch response body.
			body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
			if err != nil || body == nil {
				return
			}
			text := body.Body
			if !strings.HasPrefix(strings.TrimSpace(text), "{") {
				return
			}
			if !gjson.Valid(text) {
				return
			}
			c.ingest(text)
		},
	)()
}

// ingest parses a GraphQL JSON response and extracts profile/post data.
func (c *graphQLCollector) ingest(body string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if p := parseProfileFromGraphQL(body); p != nil && c.prof == nil {
		c.prof = p
	}
	for _, post := range parsePostsFromGraphQL(body) {
		key := post.ID
		if key == "" {
			key = post.Shortcode
		}
		if key == "" {
			continue
		}
		c.postMap[key] = post
	}
}

func (c *graphQLCollector) profile() *InstagramProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prof
}

func (c *graphQLCollector) postCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.postMap)
}

func (c *graphQLCollector) posts(limit int) []InstagramPost {
	c.mu.Lock()
	defer c.mu.Unlock()
	posts := make([]InstagramPost, 0, len(c.postMap))
	for _, p := range c.postMap {
		posts = append(posts, p)
	}
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts
}

// extractUsername parses a username from instagram.com/username/.
func extractUsername(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	parts := strings.Split(rawURL, "instagram.com/")
	if len(parts) < 2 {
		return ""
	}
	seg := strings.Split(parts[1], "/")[0]
	if seg == "p" || seg == "reel" || seg == "" {
		return ""
	}
	return seg
}

// extractShortcode parses a shortcode from instagram.com/p/SHORTCODE/ or /reel/SHORTCODE/.
func extractShortcode(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	for _, prefix := range []string{"/p/", "/reel/"} {
		if idx := strings.Index(rawURL, prefix); idx >= 0 {
			rest := rawURL[idx+len(prefix):]
			return strings.Split(rest, "/")[0]
		}
	}
	return ""
}
