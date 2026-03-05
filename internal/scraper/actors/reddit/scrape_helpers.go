package reddit

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
)

var allowedHosts = map[string]bool{
	"reddit.com":     true,
	"www.reddit.com": true,
	"old.reddit.com": true,
}

func (a *RedditActor) scrapeSubreddit(ctx context.Context, sub string, stats *actor.RunStats, delay time.Duration) ([]RedditPost, error) {
	var all []RedditPost
	after := ""
	for len(all) < a.input.MaxPostsPerSource {
		remaining := a.input.MaxPostsPerSource - len(all)
		if remaining > 100 {
			remaining = 100
		}
		actor.IncrementRequests(stats)
		posts, next, err := fetchSubredditPosts(ctx, a.client, sub, a.input.SortBy, a.input.TimeFilter, after, remaining)
		if err != nil {
			actor.IncrementFailed(stats)
			return all, err
		}
		all = append(all, posts...)
		if next == "" || len(posts) == 0 {
			break
		}
		after = next
		_ = actor.Delay(ctx, delay)
	}
	return all, nil
}

func (a *RedditActor) scrapePostURL(ctx context.Context, postURL string, stats *actor.RunStats, delay time.Duration) (RedditPost, error) {
	u, err := url.Parse(postURL)
	if err != nil {
		return RedditPost{}, fmt.Errorf("parse URL: %w", err)
	}
	permalink := u.Path

	actor.IncrementRequests(stats)
	resp, err := getJSON(ctx, a.client, postURL+".json?raw_json=1")
	if err != nil {
		actor.IncrementFailed(stats)
		return RedditPost{}, err
	}

	// Post URL response is a JSON array: [post_listing, comment_listing].
	// Try array format first, fall back to single listing.
	posts, _ := parsePostDetailResponse(resp.Body)
	if len(posts) == 0 {
		posts, _ = parsePostListing(resp.Body)
	}
	if len(posts) == 0 {
		return RedditPost{}, fmt.Errorf("no post found at %s", postURL)
	}
	post := posts[0]

	if a.input.IncludeComments && a.input.MaxCommentsPerPost > 0 {
		// Post detail response already has comments in index 1, parse from there
		// to avoid an extra request.
		comments := parseCommentsFromDetailResponse(resp.Body, a.input.MaxCommentsPerPost)
		if len(comments) == 0 {
			// Fallback: fetch comments separately.
			_ = actor.Delay(ctx, delay)
			actor.IncrementRequests(stats)
			comments, err = fetchPostComments(ctx, a.client, permalink, a.input.MaxCommentsPerPost)
			if err != nil {
				actor.IncrementFailed(stats)
			}
		}
		post.Comments = comments
	}
	return post, nil
}

func (a *RedditActor) scrapeUser(ctx context.Context, username string, stats *actor.RunStats, delay time.Duration) ([]RedditPost, error) {
	var all []RedditPost
	after := ""
	for len(all) < a.input.MaxPostsPerSource {
		remaining := a.input.MaxPostsPerSource - len(all)
		if remaining > 100 {
			remaining = 100
		}
		actor.IncrementRequests(stats)
		posts, next, err := fetchUserPosts(ctx, a.client, username, a.input.SortBy, a.input.TimeFilter, after, remaining)
		if err != nil {
			actor.IncrementFailed(stats)
			return all, err
		}
		all = append(all, posts...)
		if next == "" || len(posts) == 0 {
			break
		}
		after = next
		_ = actor.Delay(ctx, delay)
	}
	return all, nil
}

func (a *RedditActor) scrapeSearch(ctx context.Context, query string, stats *actor.RunStats, delay time.Duration) ([]RedditPost, error) {
	var all []RedditPost
	after := ""
	for len(all) < a.input.MaxPostsPerSource {
		remaining := a.input.MaxPostsPerSource - len(all)
		if remaining > 100 {
			remaining = 100
		}
		actor.IncrementRequests(stats)
		posts, next, err := searchPosts(ctx, a.client, query, a.input.SortBy, a.input.TimeFilter, after, remaining)
		if err != nil {
			actor.IncrementFailed(stats)
			return all, err
		}
		all = append(all, posts...)
		if next == "" || len(posts) == 0 {
			break
		}
		after = next
		_ = actor.Delay(ctx, delay)
	}
	return all, nil
}

func (a *RedditActor) validateRedditURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := strings.ToLower(u.Hostname())
	if !allowedHosts[host] {
		return fmt.Errorf("SSRF: host %q not allowed (must be reddit.com)", host)
	}
	return nil
}
