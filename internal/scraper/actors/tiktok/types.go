package tiktok

// TikTokInput defines the scraping parameters for the TikTok actor.
type TikTokInput struct {
	VideoURLs      []string `json:"video_urls"`
	Usernames      []string `json:"usernames"`
	SearchQueries  []string `json:"search_queries"`
	MaxResults     int      `json:"max_results"`
	RequestDelayMs int      `json:"request_delay_ms"`
}

// TikTokVideo represents a single TikTok video.
type TikTokVideo struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	AuthorURL   string `json:"author_url"`
	Likes       int    `json:"likes"`
	Comments    int    `json:"comments"`
	Shares      int    `json:"shares"`
	Views       int    `json:"views"`
	Duration    int    `json:"duration"`
	CreatedAt   string `json:"created_at"`
	MusicTitle  string `json:"music_title"`
	MusicAuthor string `json:"music_author"`
	CoverURL     string `json:"cover_url"`
	VideoURL     string `json:"video_url,omitempty"`
	VideoHDURL   string `json:"video_hd_url,omitempty"`
	IsAd         bool   `json:"is_ad"`
}

// TikTokProfile represents a TikTok user profile.
type TikTokProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	URL         string `json:"url"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	Likes       int    `json:"likes"`
	VideoCount  int    `json:"video_count"`
	AvatarURL   string `json:"avatar_url"`
	Verified    bool   `json:"verified"`
}
