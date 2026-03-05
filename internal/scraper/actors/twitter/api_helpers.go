package twitter

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"time"

	"github.com/tidwall/gjson"
)

// tweetIDFromHTML extracts tweet IDs from x.com/twitter.com status URLs in HTML.
var tweetIDFromHTML = regexp.MustCompile(`(?:x\.com|twitter\.com)/[^/]+/status/(\d+)`)

const fxTwitterAPI = "https://api.fxtwitter.com"

// fetchProfile fetches a user profile via the FxTwitter public API.
func (a *TwitterActor) fetchProfile(ctx context.Context, screenName string) (TwitterProfile, error) {
	target := fxTwitterAPI + "/" + screenName
	resp, err := a.client.Get(ctx, target)
	if err != nil {
		return TwitterProfile{}, fmt.Errorf("profile %s: %w", screenName, err)
	}
	if resp.StatusCode != 200 {
		return TwitterProfile{}, fmt.Errorf("profile %s: status %d", screenName, resp.StatusCode)
	}
	b := resp.Body
	if gjson.Get(b, "code").Int() != 200 {
		return TwitterProfile{}, fmt.Errorf("profile %s: %s", screenName, gjson.Get(b, "message").String())
	}
	u := gjson.Get(b, "user")
	return TwitterProfile{
		ID:              u.Get("id").String(),
		Username:        u.Get("screen_name").String(),
		DisplayName:     u.Get("name").String(),
		Bio:             u.Get("description").String(),
		URL:             u.Get("url").String(),
		Followers:       int(u.Get("followers").Int()),
		Following:       int(u.Get("following").Int()),
		TweetCount:      int(u.Get("tweets").Int()),
		Verified:        u.Get("verified").Bool(),
		IsBlueVerified:  u.Get("verified_type").String() == "blue",
		ProfileImageURL: u.Get("avatar_url").String(),
		BannerURL:       u.Get("banner_url").String(),
		JoinedAt:        u.Get("joined").String(),
	}, nil
}

// fetchTweet fetches a single tweet by ID via the FxTwitter public API.
func (a *TwitterActor) fetchTweet(ctx context.Context, id string) (TwitterTweet, error) {
	target := fxTwitterAPI + "/i/status/" + id
	resp, err := a.client.Get(ctx, target)
	if err != nil {
		return TwitterTweet{}, fmt.Errorf("tweet %s: %w", id, err)
	}
	if resp.StatusCode != 200 {
		return TwitterTweet{}, fmt.Errorf("tweet %s: status %d", id, resp.StatusCode)
	}
	b := resp.Body
	if gjson.Get(b, "code").Int() != 200 {
		return TwitterTweet{}, fmt.Errorf("tweet %s: %s", id, gjson.Get(b, "message").String())
	}
	return parseFxTweet(gjson.Get(b, "tweet")), nil
}

// braveTimeFilter maps time_filter values to Brave Search tf= parameter.
var braveTimeFilter = map[string]string{
	"day":   "pd",
	"week":  "pw",
	"month": "pm",
	"year":  "py",
}

// searchTweets searches for tweets via Brave Search with site:x.com,
// then fetches each discovered tweet via the FxTwitter API.
func (a *TwitterActor) searchTweets(ctx context.Context, query string, maxResults int) ([]TwitterTweet, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	searchQuery := query + " site:x.com"
	searchURL := "https://search.brave.com/search?q=" + url.QueryEscape(searchQuery)
	if tf, ok := braveTimeFilter[a.input.TimeFilter]; ok {
		searchURL += "&tf=" + tf
	}

	resp, err := a.client.Get(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search tweets: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search tweets: status %d", resp.StatusCode)
	}

	// Extract unique tweet IDs from Brave SERP
	seen := make(map[string]bool)
	var tweetIDs []string
	for _, match := range tweetIDFromHTML.FindAllStringSubmatch(resp.Body, -1) {
		id := match[1]
		if !seen[id] {
			seen[id] = true
			tweetIDs = append(tweetIDs, id)
		}
		if len(tweetIDs) >= maxResults {
			break
		}
	}

	// Fetch each tweet via FxTwitter
	var tweets []TwitterTweet
	for _, id := range tweetIDs {
		tweet, err := a.fetchTweet(ctx, id)
		if err != nil {
			continue
		}
		tweets = append(tweets, tweet)
	}

	// Sort results
	sortTweets(tweets, a.input.SortBy)
	return tweets, nil
}

// sortTweets sorts tweets by "top" (engagement) or "latest" (date, default).
func sortTweets(tweets []TwitterTweet, sortBy string) {
	if sortBy == "top" {
		sort.Slice(tweets, func(i, j int) bool {
			return tweetEngagement(tweets[i]) > tweetEngagement(tweets[j])
		})
	} else {
		// Default: latest first
		sort.Slice(tweets, func(i, j int) bool {
			ti, _ := time.Parse(time.RubyDate, tweets[i].CreatedAt)
			tj, _ := time.Parse(time.RubyDate, tweets[j].CreatedAt)
			return ti.After(tj)
		})
	}
}

func tweetEngagement(t TwitterTweet) int {
	return t.Likes + t.Retweets*2 + t.Replies + t.Quotes + t.Views/100
}

// fetchUserTweets discovers and fetches recent tweets from a user via Brave Search.
func (a *TwitterActor) fetchUserTweets(ctx context.Context, username string, maxTweets int) ([]TwitterTweet, error) {
	query := "from:" + username
	return a.searchTweets(ctx, query, maxTweets)
}

// parseFxTweet converts an FxTwitter tweet JSON object into a TwitterTweet.
func parseFxTweet(t gjson.Result) TwitterTweet {
	authorUsername := t.Get("author.screen_name").String()
	tweetID := t.Get("id").String()
	tweetURL := t.Get("url").String()
	if tweetURL == "" && authorUsername != "" && tweetID != "" {
		tweetURL = fmt.Sprintf("https://x.com/%s/status/%s", authorUsername, tweetID)
	}
	var media []string
	t.Get("media.all").ForEach(func(_, v gjson.Result) bool {
		if u := v.Get("url").String(); u != "" {
			media = append(media, u)
		}
		return true
	})
	return TwitterTweet{
		ID:             tweetID,
		Text:           t.Get("text").String(),
		AuthorID:       t.Get("author.id").String(),
		AuthorUsername: authorUsername,
		URL:            tweetURL,
		CreatedAt:      t.Get("created_at").String(),
		Likes:          int(t.Get("likes").Int()),
		Retweets:       int(t.Get("retweets").Int()),
		Replies:        int(t.Get("replies").Int()),
		Quotes:         int(t.Get("quotes").Int()),
		Views:          int(t.Get("views").Int()),
		IsRetweet:      t.Get("is_retweet").Bool(),
		IsReply:        t.Get("replying_to").Exists(),
		Media:          media,
	}
}

