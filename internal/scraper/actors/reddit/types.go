package reddit

// RedditInput defines the scraping parameters for the Reddit actor.
type RedditInput struct {
	Subreddits         []string `json:"subreddits"`
	Usernames          []string `json:"usernames"`
	PostURLs           []string `json:"post_urls"`
	SearchQueries      []string `json:"search_queries"`
	MaxResults         int      `json:"max_results"`
	MaxPostsPerSource  int      `json:"max_posts_per_source"`
	MaxCommentsPerPost int      `json:"max_comments_per_post"`
	IncludeComments    bool     `json:"include_comments"`
	SortBy             string   `json:"sort_by"`     // hot, new, top, rising
	TimeFilter         string   `json:"time_filter"` // hour, day, week, month, year, all
	RequestDelayMs     int      `json:"request_delay_ms"`
}

// RedditPost represents a single Reddit post with optional comments.
type RedditPost struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	Subreddit  string `json:"subreddit"`
	URL        string `json:"url"`
	Permalink  string `json:"permalink"`
	Selftext   string `json:"selftext"`
	Score      int    `json:"score"`
	NumComments int   `json:"num_comments"`
	Upvotes    int    `json:"upvotes"`
	Downvotes  int    `json:"downvotes"`
	CreatedUTC float64 `json:"created_utc"`
	IsNSFW     bool   `json:"is_nsfw"`
	IsSelf     bool   `json:"is_self"`
	Thumbnail  string `json:"thumbnail"`
	Comments   []RedditComment `json:"comments,omitempty"`
}

// RedditComment represents a single Reddit comment.
type RedditComment struct {
	ID         string  `json:"id"`
	Author     string  `json:"author"`
	Body       string  `json:"body"`
	Permalink  string  `json:"permalink"`
	Score      int     `json:"score"`
	CreatedUTC float64 `json:"created_utc"`
}
