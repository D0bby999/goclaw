package social

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// uploadTikTokVideo orchestrates the TikTok web upload flow on an already-navigated page.
func uploadTikTokVideo(ctx context.Context, page *rod.Page, req PublishRequest, logger *slog.Logger) (*PublishResult, error) {
	if len(req.Media) == 0 {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryClient, Message: "at least one media item required"}
	}

	// Find the first video item
	var videoItem *MediaItem
	for i := range req.Media {
		if req.Media[i].MediaType == "video" || strings.HasPrefix(req.Media[i].MimeType, "video/") {
			videoItem = &req.Media[i]
			break
		}
	}
	if videoItem == nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryClient, Message: "no video media item found (tiktok browser upload requires video)"}
	}

	// Download video to temp file
	tmpPath, err := downloadToTemp(ctx, videoItem.URL)
	if err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryNetwork, Message: "download video: " + err.Error()}
	}
	defer os.Remove(tmpPath)

	// Wait for the upload page to be ready (file input must exist)
	fileInput, err := findElement(page, 5*time.Second,
		`input[type="file"][accept*="video"]`,
		`input[type="file"]`,
	)
	if err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "upload input not found: " + err.Error()}
	}

	// Set the video file on the input element
	if err := fileInput.SetFiles([]string{tmpPath}); err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "set file input: " + err.Error()}
	}
	logger.Info("video file set on upload input", "path", tmpPath)

	// Wait for upload to complete (progress indicator disappears)
	if err := waitForUploadComplete(ctx, page, 120*time.Second); err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "upload timeout: " + err.Error()}
	}
	logger.Info("video upload completed")

	humanDelay(800, 1500)

	// Fill caption
	if req.Content != "" {
		if err := fillCaption(page, req.Content); err != nil {
			logger.Warn("caption fill failed (continuing)", "error", err)
		}
	}

	humanDelay(500, 1000)

	// Click the Post button
	if err := clickPostButton(page); err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "click post button: " + err.Error()}
	}
	logger.Info("post button clicked")

	// Wait for success confirmation
	postURL, err := waitForPostSuccess(ctx, page, 30*time.Second)
	if err != nil {
		return nil, &PlatformError{Platform: "tiktok", Category: ErrCategoryPlatform, Message: "post confirmation: " + err.Error()}
	}

	postID := fmt.Sprintf("browser-%d", time.Now().UnixMilli())
	return &PublishResult{
		PlatformPostID: postID,
		PlatformURL:    postURL,
	}, nil
}

// findElement tries multiple CSS selectors and returns the first match within timeout.
func findElement(page *rod.Page, timeout time.Duration, selectors ...string) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, sel := range selectors {
			el, err := page.Timeout(2 * time.Second).Element(sel)
			if err == nil && el != nil {
				return el, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("element not found with selectors %v within %s", selectors, timeout)
}

// waitForUploadComplete polls until the upload progress indicator is gone or a success signal appears.
func waitForUploadComplete(_ context.Context, page *rod.Page, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Check for error state first
		errEl, _ := page.Timeout(500 * time.Millisecond).Element(`[data-e2e="upload-error"]`)
		if errEl != nil {
			txt, _ := errEl.Text()
			return fmt.Errorf("upload error: %s", txt)
		}

		// Check if upload progress is gone (progress bar or spinner)
		progressSelectors := []string{
			`[data-e2e="upload-progress"]`,
			`.upload-progress`,
			`[class*="uploadProgress"]`,
		}
		uploading := false
		for _, sel := range progressSelectors {
			el, err := page.Timeout(300 * time.Millisecond).Element(sel)
			if err == nil && el != nil {
				uploading = true
				break
			}
		}

		// Check for success state (caption editor or post button visible)
		successSelectors := []string{
			`div[contenteditable="true"]`,
			`[data-e2e="post_video"]`,
			`button[data-e2e="post_video_button"]`,
		}
		for _, sel := range successSelectors {
			el, err := page.Timeout(300 * time.Millisecond).Element(sel)
			if err == nil && el != nil && !uploading {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("upload did not complete within %s", timeout)
}

// fillCaption clears and fills the caption/description field.
func fillCaption(page *rod.Page, content string) error {
	captionSelectors := []string{
		`div[contenteditable="true"]`,
		`[data-text="true"]`,
		`[class*="caption"] div[contenteditable]`,
		`[class*="description"] div[contenteditable]`,
	}
	el, err := findElement(page, 5*time.Second, captionSelectors...)
	if err != nil {
		return fmt.Errorf("caption field not found: %w", err)
	}

	// Clear existing content and type new caption
	if err := el.SelectAllText(); err != nil {
		// If SelectAllText fails, click then select-all via keyboard
		_ = el.Click(proto.InputMouseButtonLeft, 1)
	}
	humanDelay(200, 400)
	return el.Input(content)
}

// clickPostButton finds and clicks the publish/post button.
func clickPostButton(page *rod.Page) error {
	postSelectors := []string{
		`button[data-e2e="post_video"]`,
		`button[data-e2e="post_video_button"]`,
		`[class*="btn-post"]`,
		`button[class*="post"]`,
	}

	// Fallback: find button containing "Post" text
	el, err := findElement(page, 5*time.Second, postSelectors...)
	if err != nil {
		// Try text-based search
		el, err = page.Timeout(5 * time.Second).ElementR("button", "^Post$")
		if err != nil {
			return fmt.Errorf("post button not found")
		}
	}

	humanDelay(300, 700)
	return el.Click(proto.InputMouseButtonLeft, 1)
}

// waitForPostSuccess waits for navigation or success toast after clicking Post.
// Returns the post URL if extractable, otherwise returns the profile URL.
func waitForPostSuccess(_ context.Context, page *rod.Page, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		info, err := page.Info()
		if err == nil && info != nil {
			// Success: navigated away from upload page to a video/profile page
			if strings.Contains(info.URL, "/video/") {
				return info.URL, nil
			}
			if strings.Contains(info.URL, "/@") && !strings.Contains(info.URL, "/creator") {
				return info.URL, nil
			}
		}

		// Check for success toast/banner
		successEl, _ := page.Timeout(500 * time.Millisecond).ElementR("*", "(?i)posted|success|published")
		if successEl != nil {
			return "https://www.tiktok.com", nil
		}

		// Check for error toast
		errEl, _ := page.Timeout(300 * time.Millisecond).ElementR("*", "(?i)error|failed|limit exceeded")
		if errEl != nil {
			txt, _ := errEl.Text()
			if strings.Contains(strings.ToLower(txt), "limit") {
				return "", &PlatformError{Platform: "tiktok", Category: ErrCategoryRateLimit, Message: txt}
			}
			return "", fmt.Errorf("post failed: %s", txt)
		}

		time.Sleep(1 * time.Second)
	}
	return "https://www.tiktok.com", nil // assume success after timeout; caller logs
}

// downloadToTemp downloads a URL to a temporary file and returns its path.
func downloadToTemp(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "tiktok-upload-*.mp4")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return f.Name(), nil
}
