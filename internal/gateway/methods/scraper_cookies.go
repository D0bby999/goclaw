package methods

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/scraper"
	"github.com/nextlevelbuilder/goclaw/pkg/browser"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ScraperCookieMethods handles scraper.cookies.* RPC methods.
type ScraperCookieMethods struct {
	store      *scraper.ScraperCookieStore
	browserMgr *browser.Manager // may be nil
}

func NewScraperCookieMethods(store *scraper.ScraperCookieStore, browserMgr *browser.Manager) *ScraperCookieMethods {
	return &ScraperCookieMethods{store: store, browserMgr: browserMgr}
}

func (m *ScraperCookieMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodScraperCookiesList, m.handleList)
	router.Register(protocol.MethodScraperCookiesSet, m.handleSet)
	router.Register(protocol.MethodScraperCookiesDelete, m.handleDelete)
	router.Register(protocol.MethodScraperCookiesLogin, m.handleLogin)
}

func (m *ScraperCookieMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	entries, err := m.store.List(ctx)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	// Mask cookie values for list response.
	masked := make([]map[string]string, len(entries))
	for i, e := range entries {
		masked[i] = map[string]string{
			"label":      e.Label,
			"platform":   e.Platform,
			"cookies":    maskCookies(e.Cookies),
			"created_at": e.CreatedAt,
			"updated_at": e.UpdatedAt,
		}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"entries": masked}))
}

func (m *ScraperCookieMethods) handleSet(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Platform string `json:"platform"`
		Label    string `json:"label"`
		Cookies  string `json:"cookies"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Platform == "" || params.Label == "" || params.Cookies == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform, label, and cookies are required"))
		return
	}
	if !isValidScraperPlatform(params.Platform) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform must be facebook or instagram"))
		return
	}

	entry := scraper.ScraperCookieEntry{
		Label:    params.Label,
		Platform: params.Platform,
		Cookies:  params.Cookies,
	}
	if err := m.store.Set(ctx, entry); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *ScraperCookieMethods) handleDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Platform string `json:"platform"`
		Label    string `json:"label"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Platform == "" || params.Label == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform and label are required"))
		return
	}
	if err := m.store.Delete(ctx, params.Platform, params.Label); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *ScraperCookieMethods) handleLogin(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Platform string `json:"platform"`
		Label    string `json:"label"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Platform == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform is required"))
		return
	}
	if !isValidScraperPlatform(params.Platform) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform must be facebook or instagram"))
		return
	}
	if params.Label == "" {
		params.Label = params.Platform + "-default"
	}

	loginURL := platformLoginURL(params.Platform)
	cookies, err := browserLoginFlow(ctx, loginURL, params.Platform)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "login failed: "+err.Error()))
		return
	}

	entry := scraper.ScraperCookieEntry{
		Label:    params.Label,
		Platform: params.Platform,
		Cookies:  cookies,
	}
	if err := m.store.Set(ctx, entry); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok":       true,
		"label":    params.Label,
		"platform": params.Platform,
	}))
}

// browserLoginFlow opens a non-headless browser for the user to log in,
// polls until login completes (URL no longer contains /login), extracts cookies.
func browserLoginFlow(ctx context.Context, loginURL, platform string) (string, error) {
	chromePath := launcher.NewBrowser().MustGet()
	l := launcher.New().Bin(chromePath).Headless(false).
		Set("no-first-run").
		Set("no-default-browser-check")
	controlURL, err := l.Launch()
	if err != nil {
		return "", fmt.Errorf("launch browser: %w", err)
	}

	b := rod.New().ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		return "", fmt.Errorf("connect browser: %w", err)
	}
	defer b.Close()

	page, err := b.Page(proto.TargetCreateTarget{URL: loginURL})
	if err != nil {
		return "", fmt.Errorf("open login page: %w", err)
	}

	slog.Info("scraper.cookies.login: waiting for user login", "platform", platform, "url", loginURL)

	// Poll every 2s for up to 120s.
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("login timed out after 120s")
		case <-ticker.C:
			info, err := page.Info()
			if err != nil {
				continue
			}
			if !strings.Contains(info.URL, "/login") && !strings.Contains(info.URL, "/accounts/login") {
				// User has logged in — extract cookies.
				return extractPlatformCookies(b, platform)
			}
		}
	}
}

// extractPlatformCookies gets all cookies from the browser and filters for the platform.
func extractPlatformCookies(b *rod.Browser, platform string) (string, error) {
	cookies, err := b.GetCookies()
	if err != nil {
		return "", fmt.Errorf("get cookies: %w", err)
	}

	var required map[string]bool
	switch platform {
	case "facebook":
		required = map[string]bool{"c_user": false, "xs": false}
	case "instagram":
		required = map[string]bool{"sessionid": false}
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}

	var parts []string
	for _, c := range cookies {
		if _, ok := required[c.Name]; ok {
			parts = append(parts, c.Name+"="+c.Value)
			required[c.Name] = true
		}
	}

	// Check all required cookies were found.
	var missing []string
	for name, found := range required {
		if !found {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing required cookies: %s", strings.Join(missing, ", "))
	}

	return strings.Join(parts, "; "), nil
}

func platformLoginURL(platform string) string {
	switch platform {
	case "facebook":
		return "https://www.facebook.com/login"
	case "instagram":
		return "https://www.instagram.com/accounts/login/"
	default:
		return ""
	}
}

func isValidScraperPlatform(p string) bool {
	return p == "facebook" || p == "instagram"
}

func maskCookies(cookies string) string {
	if len(cookies) <= 8 {
		return "***"
	}
	return cookies[:4] + "..." + cookies[len(cookies)-4:]
}
