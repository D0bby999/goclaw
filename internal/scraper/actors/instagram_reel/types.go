package instagram_reel

// ReelInput defines scraping parameters for the Instagram Reel actor.
type ReelInput struct {
	ReelURLs       []string `json:"reel_urls"`
	MaxResults     int      `json:"max_results"`
	RequestDelayMs int      `json:"request_delay_ms"`
	Cookies        string   `json:"cookies,omitempty"`
}

// ReelResult represents a scraped Instagram Reel.
type ReelResult struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	VideoURL     string `json:"video_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Caption      string `json:"caption"`
	Likes        int    `json:"likes"`
	Comments     int    `json:"comments"`
	Views        int    `json:"views"`
	Shares       int    `json:"shares"`
	Duration     int    `json:"duration"`
	Author       string `json:"author"`
	AuthorURL    string `json:"author_url"`
	MusicTitle   string `json:"music_title"`
}
