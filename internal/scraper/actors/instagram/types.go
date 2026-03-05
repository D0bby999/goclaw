package instagram

// InstagramInput defines scraping parameters for the Instagram actor.
type InstagramInput struct {
	ProfileURLs     []string `json:"profile_urls"`
	PostURLs        []string `json:"post_urls"`
	MaxResults      int      `json:"max_results"`
	MaxPostsPerUser int      `json:"max_posts_per_user"`
	RequestDelayMs  int      `json:"request_delay_ms"`
	Cookies         string   `json:"cookies,omitempty"`
}

// InstagramPost represents a single Instagram post.
type InstagramPost struct {
	ID            string `json:"id"`
	Shortcode     string `json:"shortcode"`
	URL           string `json:"url"`
	Caption       string `json:"caption"`
	MediaURL      string `json:"media_url"`
	MediaType     string `json:"media_type"`
	Likes         int    `json:"likes"`
	Comments      int    `json:"comments"`
	Timestamp     string `json:"timestamp"`
	OwnerUsername string `json:"owner_username"`
	OwnerID       string `json:"owner_id"`
	IsVideo       bool   `json:"is_video"`
	VideoURL      string `json:"video_url,omitempty"`
}

// InstagramProfile represents an Instagram user profile.
type InstagramProfile struct {
	UserID        string `json:"user_id"`
	Username      string `json:"username"`
	FullName      string `json:"full_name"`
	Bio           string `json:"bio"`
	ProfilePicURL string `json:"profile_pic_url"`
	ExternalURL   string `json:"external_url"`
	Followers     int    `json:"followers"`
	Following     int    `json:"following"`
	PostCount     int    `json:"post_count"`
	IsPrivate     bool   `json:"is_private"`
	IsVerified    bool   `json:"is_verified"`
}
