package social

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// tiktokCookie is the JSON structure for cookies stored in metadata or cookie store.
type tiktokCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// tiktokBrowserClient implements PlatformClient using Rod browser automation.
type tiktokBrowserClient struct {
	browserMgr *browser.Manager
	cookies    CookieSource
	metadata   json.RawMessage
	logger     *slog.Logger
}

func newTikTokBrowserClient(browserMgr *browser.Manager, cookies CookieSource, metadata json.RawMessage) *tiktokBrowserClient {
	return &tiktokBrowserClient{
		browserMgr: browserMgr,
		cookies:    cookies,
		metadata:   metadata,
		logger:     slog.Default().With("component", "tiktok-browser"),
	}
}

func (c *tiktokBrowserClient) Platform() string { return "tiktok" }

func (c *tiktokBrowserClient) RefreshToken(_ context.Context, _ string) (*TokenResult, error) {
	return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryClient, Message: "token refresh not supported in browser mode"}
}

func (c *tiktokBrowserClient) GetProfile(_ context.Context) (*ProfileResult, error) {
	return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryClient, Message: "get profile not supported in browser mode"}
}

func (c *tiktokBrowserClient) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if err := c.browserMgr.Start(ctx); err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "browser start: " + err.Error()}
	}

	tab, err := c.browserMgr.OpenTab(ctx, "about:blank")
	if err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "open tab: " + err.Error()}
	}
	defer func() {
		if closeErr := c.browserMgr.CloseTab(ctx, tab.TargetID); closeErr != nil {
			c.logger.Warn("failed to close tiktok tab", "error", closeErr)
		}
	}()

	page, err := c.browserMgr.GetPage(tab.TargetID)
	if err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "get page: " + err.Error()}
	}

	applyStealthSetup(page)

	if err := c.loadCookies(ctx, page); err != nil {
		c.logger.Warn("cookie loading failed (continuing unauthenticated)", "error", err)
	}

	humanDelay(500, 1200)

	if err := page.Navigate("https://www.tiktok.com/creator#upload"); err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "navigate: " + err.Error()}
	}
	if err := page.WaitLoad(); err != nil {
		c.logger.Warn("page WaitLoad error (continuing)", "error", err)
	}

	if info, infoErr := page.Info(); infoErr == nil && isLoginPage(info.URL) {
		c.captureErrorScreenshot(ctx, tab.TargetID)
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryAuth, Message: "redirected to login — cookies may be expired"}
	}

	result, uploadErr := uploadTikTokVideo(ctx, page, req, c.logger)
	if uploadErr != nil {
		c.captureErrorScreenshot(ctx, tab.TargetID)
		return nil, uploadErr
	}
	return result, nil
}

// applyStealthSetup injects anti-detection overrides and sets a realistic viewport.
func applyStealthSetup(page *rod.Page) {
	const stealthJS = `
		Object.defineProperty(navigator, 'webdriver', {get: () => false});
		Object.defineProperty(navigator, 'plugins', {get: () => [1,2,3,4,5]});
		Object.defineProperty(navigator, 'languages', {get: () => ['en-US','en']});
		try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_Array; } catch(_) {}
		try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_Promise; } catch(_) {}
		try { delete window.cdc_adoQpoasnfa76pfcZLmcfl_Symbol; } catch(_) {}
	`
	_, _ = page.Eval(stealthJS)
	_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             1920,
		Height:            1080,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})
}

// loadCookies injects TikTok session cookies via CDP Network.SetCookies.
func (c *tiktokBrowserClient) loadCookies(ctx context.Context, page *rod.Page) error {
	cookieJSON := c.extractCookiesFromMetadata()
	if cookieJSON == "" && c.cookies != nil {
		raw, err := c.cookies.GetDefault(ctx, "tiktok")
		if err != nil {
			return fmt.Errorf("cookie store: %w", err)
		}
		cookieJSON = raw
	}
	if cookieJSON == "" {
		return nil
	}

	var cookies []tiktokCookie
	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return fmt.Errorf("parse cookies: %w", err)
	}

	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	for _, ck := range cookies {
		domain := ck.Domain
		if domain == "" {
			domain = ".tiktok.com"
		}
		path := ck.Path
		if path == "" {
			path = "/"
		}
		params = append(params, &proto.NetworkCookieParam{
			Name:   ck.Name,
			Value:  ck.Value,
			Domain: domain,
			Path:   path,
		})
	}

	return proto.NetworkSetCookies{Cookies: params}.Call(page)
}

func (c *tiktokBrowserClient) extractCookiesFromMetadata() string {
	if len(c.metadata) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(c.metadata, &m); err != nil {
		return ""
	}
	raw, ok := m["cookies"]
	if !ok {
		return ""
	}
	// Stored as JSON string or raw JSON array
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func (c *tiktokBrowserClient) captureErrorScreenshot(ctx context.Context, targetID string) {
	data, err := c.browserMgr.Screenshot(ctx, targetID, false)
	if err != nil {
		c.logger.Warn("screenshot failed", "error", err)
		return
	}
	f, err := os.CreateTemp("", "tiktok-err-*.png")
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
	c.logger.Info("tiktok error screenshot saved", "path", f.Name())
}

func isLoginPage(url string) bool {
	return strings.Contains(url, "/login") ||
		strings.Contains(url, "/signup") ||
		strings.Contains(url, "accounts.tiktok")
}

func humanDelay(minMs, maxMs int) {
	d := minMs + rand.Intn(maxMs-minMs+1)
	time.Sleep(time.Duration(d) * time.Millisecond)
}
