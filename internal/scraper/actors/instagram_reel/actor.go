package instagram_reel

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

// ReelActor scrapes Instagram Reels via headless browser + private API.
type ReelActor struct {
	input   ReelInput
	client  *httpclient.Client
	errors  []actor.Error
	browser *rod.Browser
}

// NewActor constructs a ReelActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*ReelActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed ReelInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxResults <= 0 {
		typed.MaxResults = 20
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 1500
	}
	return &ReelActor{input: typed, client: client}, nil
}

// Initialize launches headless Chrome and injects the session cookie.
func (a *ReelActor) Initialize(ctx context.Context) error {
	sessionID := strings.TrimSpace(a.input.Cookies)
	if sessionID == "" {
		return fmt.Errorf("instagram_reel requires cookies (sessionid). Configure cookies in Settings > Scrapers")
	}
	sessionID = strings.TrimPrefix(sessionID, "sessionid=")

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

	// Set random viewport.
	widths := []int{1280, 1366, 1440, 1536, 1920}
	heights := []int{720, 768, 900, 1024, 1080}
	page, err := b.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	w, h := widths[rand.IntN(len(widths))], heights[rand.IntN(len(heights))]
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: w, Height: h})

	// Inject sessionid cookie.
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
func (a *ReelActor) Cleanup() {
	if a.browser != nil {
		_ = a.browser.Close()
	}
}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *ReelActor) CollectedErrors() []actor.Error { return a.errors }

// Execute scrapes each reel URL via browser-based API call.
func (a *ReelActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for i, reelURL := range a.input.ReelURLs {
		if i >= a.input.MaxResults {
			break
		}

		actor.IncrementRequests(stats)
		reel, err := a.scrapeReel(ctx, reelURL)
		if err != nil {
			actor.HandleURLError(err, reelURL, &a.errors, stats)
			continue
		}

		if reel.URL == "" {
			reel.URL = reelURL
		}

		b, _ := json.Marshal(reel)
		results = append(results, b)

		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}

// igMediaInfoScript calls the private API from within the page context,
// using the httpOnly session cookie automatically.
const igMediaInfoScript = `async () => {
	const r = await fetch('/api/v1/media/%s/info/', {
		headers: {'X-IG-App-ID': '936619743392459'},
		credentials: 'include',
	});
	const text = await r.text();
	return JSON.stringify({status: r.status, body: text});
}`

// scrapeReel navigates to the reel URL then fetches data via private API.
func (a *ReelActor) scrapeReel(ctx context.Context, reelURL string) (*ReelResult, error) {
	shortcode := extractShortcode(reelURL)
	if shortcode == "" {
		return nil, fmt.Errorf("could not extract shortcode from URL: %s", reelURL)
	}

	page, err := a.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}
	defer page.Close()
	page = page.Context(ctx)

	// Navigate to establish instagram.com origin for same-origin fetch.
	if err := page.Navigate(reelURL); err != nil {
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

	// Call private API.
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

	item := gjson.Get(body, "items.0")
	if !item.Exists() {
		return nil, fmt.Errorf("media info API: no items in response")
	}

	return parseReelFromAPI(item), nil
}

// parseReelFromAPI extracts ReelResult from the API response item.
func parseReelFromAPI(item gjson.Result) *ReelResult {
	shortcode := item.Get("code").String()
	owner := item.Get("user")

	r := &ReelResult{
		ID:           item.Get("pk").String(),
		URL:          "https://www.instagram.com/reel/" + shortcode + "/",
		Caption:      item.Get("caption.text").String(),
		Likes:        int(item.Get("like_count").Int()),
		Comments:     int(item.Get("comment_count").Int()),
		Views:        int(item.Get("play_count").Int()),
		Duration:     int(item.Get("video_duration").Float()),
		Author:       owner.Get("username").String(),
		AuthorURL:    "https://www.instagram.com/" + owner.Get("username").String() + "/",
		MusicTitle:   item.Get("clips_metadata.music_info.music_asset_info.title").String(),
		ThumbnailURL: item.Get("image_versions2.candidates.0.url").String(),
	}

	// Video URL: try video_versions first.
	if vv := item.Get("video_versions.0.url"); vv.Exists() {
		r.VideoURL = vv.String()
	}

	return r
}

// shortcodeToMediaID converts an Instagram shortcode to its numeric media ID.
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

// extractShortcode parses a shortcode from /reel/SHORTCODE/ or /p/SHORTCODE/.
func extractShortcode(rawURL string) string {
	rawURL = strings.TrimSuffix(rawURL, "/")
	for _, prefix := range []string{"/reel/", "/p/"} {
		if idx := strings.Index(rawURL, prefix); idx >= 0 {
			rest := rawURL[idx+len(prefix):]
			return strings.Split(rest, "/")[0]
		}
	}
	return ""
}
