package twitter

// TwitterInput defines the scraping parameters for the Twitter actor.
type TwitterInput struct {
	Handles          []string `json:"handles"`
	SearchQueries    []string `json:"search_queries"`
	TweetURLs        []string `json:"tweet_urls"`
	MaxResults       int      `json:"max_results"`
	MaxTweetsPerUser int      `json:"max_tweets_per_user"`
	SortBy           string   `json:"sort_by"`      // "top" or "latest" (default: "latest")
	TimeFilter       string   `json:"time_filter"`   // "day", "week", "month", "year", "all" (default: "all")
	RequestDelayMs   int      `json:"request_delay_ms"`
}

// TwitterProfile represents a Twitter/X user profile.
type TwitterProfile struct {
	ID              string `json:"id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Bio             string `json:"bio"`
	URL             string `json:"url"`
	Followers       int    `json:"followers"`
	Following       int    `json:"following"`
	TweetCount      int    `json:"tweet_count"`
	Verified        bool   `json:"verified"`
	IsBlueVerified  bool   `json:"is_blue_verified"`
	ProfileImageURL string `json:"profile_image_url"`
	BannerURL       string `json:"banner_url"`
	JoinedAt        string `json:"joined_at"`
}

// TwitterTweet represents a single tweet.
type TwitterTweet struct {
	ID             string   `json:"id"`
	Text           string   `json:"text"`
	AuthorID       string   `json:"author_id"`
	AuthorUsername string   `json:"author_username"`
	URL            string   `json:"url"`
	CreatedAt      string   `json:"created_at"`
	Likes          int      `json:"likes"`
	Retweets       int      `json:"retweets"`
	Replies        int      `json:"replies"`
	Quotes         int      `json:"quotes"`
	Views          int      `json:"views"`
	IsRetweet      bool     `json:"is_retweet"`
	IsReply        bool     `json:"is_reply"`
	Media          []string `json:"media,omitempty"`
}
