package reddit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
	"github.com/tidwall/gjson"
)

const redditBase = "https://www.reddit.com"

func buildURL(path string, params map[string]string) string {
	u := redditBase + path + ".json"
	q := url.Values{"raw_json": {"1"}}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	return u + "?" + q.Encode()
}

// getJSON fetches a Reddit JSON endpoint using old.reddit.com API headers.
// Reddit blocks stealth browser headers on .json endpoints (403). We use a
// simple bot-style User-Agent which Reddit's JSON API explicitly allows.
func getJSON(ctx context.Context, client *httpclient.Client, rawURL string) (*httpclient.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	// Reddit JSON API requires a descriptive User-Agent; browser-like UAs get blocked.
	req.Header.Set("User-Agent", "goclaw:scraper/1.0 (by /u/goclaw_bot)")
	req.Header.Set("Accept", "application/json")
	return client.Do(ctx, req)
}

// fetchSubredditPosts retrieves posts from a subreddit with cursor-based pagination.
// Returns posts, next "after" cursor, and any error.
func fetchSubredditPosts(ctx context.Context, client *httpclient.Client, subreddit, sort, timeFilter, after string, limit int) ([]RedditPost, string, error) {
	if sort == "" {
		sort = "hot"
	}
	path := fmt.Sprintf("/r/%s/%s", subreddit, sort)
	params := map[string]string{
		"limit": fmt.Sprintf("%d", limit),
		"after": after,
		"t":     timeFilter,
	}
	resp, err := getJSON(ctx, client, buildURL(path, params))
	if err != nil {
		return nil, "", fmt.Errorf("fetch subreddit %s: %w", subreddit, err)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("fetch subreddit %s: status %d", subreddit, resp.StatusCode)
	}
	posts, nextAfter := parsePostListing(resp.Body)
	return posts, nextAfter, nil
}

// fetchPostComments retrieves comments for a post by its permalink.
func fetchPostComments(ctx context.Context, client *httpclient.Client, permalink string, maxComments int) ([]RedditComment, error) {
	target := buildURL(permalink, map[string]string{"limit": fmt.Sprintf("%d", maxComments)})
	resp, err := client.Get(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("fetch comments: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch comments: status %d", resp.StatusCode)
	}
	// Response is an array: [post_listing, comment_listing]
	commentJSON := gjson.Get(resp.Body, "1.data.children")
	if !commentJSON.Exists() {
		return nil, nil
	}
	var comments []RedditComment
	commentJSON.ForEach(func(_, v gjson.Result) bool {
		if v.Get("kind").String() != "t1" {
			return true
		}
		d := v.Get("data")
		comments = append(comments, RedditComment{
			ID:         d.Get("id").String(),
			Author:     d.Get("author").String(),
			Body:       d.Get("body").String(),
			Permalink:  d.Get("permalink").String(),
			Score:      int(d.Get("score").Int()),
			CreatedUTC: d.Get("created_utc").Float(),
		})
		return len(comments) < maxComments
	})
	return comments, nil
}

// fetchUserPosts retrieves posts submitted by a Reddit user.
func fetchUserPosts(ctx context.Context, client *httpclient.Client, username, sort, timeFilter, after string, limit int) ([]RedditPost, string, error) {
	if sort == "" {
		sort = "new"
	}
	path := fmt.Sprintf("/user/%s/submitted", username)
	params := map[string]string{
		"sort":  sort,
		"t":     timeFilter,
		"limit": fmt.Sprintf("%d", limit),
		"after": after,
	}
	resp, err := getJSON(ctx, client, buildURL(path, params))
	if err != nil {
		return nil, "", fmt.Errorf("fetch user %s: %w", username, err)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("fetch user %s: status %d", username, resp.StatusCode)
	}
	posts, nextAfter := parsePostListing(resp.Body)
	return posts, nextAfter, nil
}

// searchPosts queries the Reddit search endpoint.
func searchPosts(ctx context.Context, client *httpclient.Client, query, sort, timeFilter, after string, limit int) ([]RedditPost, string, error) {
	params := map[string]string{
		"q":     query,
		"sort":  sort,
		"t":     timeFilter,
		"limit": fmt.Sprintf("%d", limit),
		"after": after,
		"type":  "link",
	}
	resp, err := getJSON(ctx, client, buildURL("/search", params))
	if err != nil {
		return nil, "", fmt.Errorf("search posts: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("search posts: status %d", resp.StatusCode)
	}
	posts, nextAfter := parsePostListing(resp.Body)
	return posts, nextAfter, nil
}

// parsePostDetailResponse extracts posts from a Reddit post detail response.
// Post detail responses are JSON arrays: [post_listing, comment_listing].
func parsePostDetailResponse(body string) ([]RedditPost, string) {
	children := gjson.Get(body, "0.data.children")
	if !children.Exists() {
		return nil, ""
	}
	nextAfter := gjson.Get(body, "0.data.after").String()
	return parseChildren(children, nextAfter)
}

// parseCommentsFromDetailResponse extracts comments from the second element
// of a post detail response array, avoiding an extra HTTP request.
func parseCommentsFromDetailResponse(body string, maxComments int) []RedditComment {
	commentJSON := gjson.Get(body, "1.data.children")
	if !commentJSON.Exists() {
		return nil
	}
	var comments []RedditComment
	commentJSON.ForEach(func(_, v gjson.Result) bool {
		if v.Get("kind").String() != "t1" {
			return true
		}
		d := v.Get("data")
		comments = append(comments, RedditComment{
			ID:         d.Get("id").String(),
			Author:     d.Get("author").String(),
			Body:       d.Get("body").String(),
			Permalink:  d.Get("permalink").String(),
			Score:      int(d.Get("score").Int()),
			CreatedUTC: d.Get("created_utc").Float(),
		})
		return len(comments) < maxComments
	})
	return comments
}

// parsePostListing extracts posts from a Reddit JSON listing response.
func parsePostListing(body string) ([]RedditPost, string) {
	children := gjson.Get(body, "data.children")
	nextAfter := gjson.Get(body, "data.after").String()
	return parseChildren(children, nextAfter)
}

// parseChildren extracts RedditPost items from a gjson children array.
func parseChildren(children gjson.Result, nextAfter string) ([]RedditPost, string) {
	var posts []RedditPost
	children.ForEach(func(_, v gjson.Result) bool {
		if v.Get("kind").String() != "t3" {
			return true
		}
		d := v.Get("data")
		posts = append(posts, RedditPost{
			ID:          d.Get("id").String(),
			Title:       d.Get("title").String(),
			Author:      d.Get("author").String(),
			Subreddit:   d.Get("subreddit").String(),
			URL:         d.Get("url").String(),
			Permalink:   d.Get("permalink").String(),
			Selftext:    d.Get("selftext").String(),
			Score:       int(d.Get("score").Int()),
			NumComments: int(d.Get("num_comments").Int()),
			Upvotes:     int(d.Get("ups").Int()),
			Downvotes:   int(d.Get("downs").Int()),
			CreatedUTC:  d.Get("created_utc").Float(),
			IsNSFW:      d.Get("over_18").Bool(),
			IsSelf:      d.Get("is_self").Bool(),
			Thumbnail:   d.Get("thumbnail").String(),
		})
		return true
	})
	return posts, nextAfter
}
