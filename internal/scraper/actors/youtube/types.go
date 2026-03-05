package youtube

// YouTubeInput defines scraping parameters for the YouTube actor.
type YouTubeInput struct {
	StartURLs      []string `json:"start_urls"`
	SearchKeywords string   `json:"search_keywords,omitempty"`
	MaxResults     int      `json:"max_results"`
	MaxComments    int      `json:"max_comments"`
	RequestDelayMs int      `json:"request_delay_ms"`
}

// YouTubeVideo represents a YouTube video.
type YouTubeVideo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Description  string `json:"description"`
	Date         string `json:"date"`
	Duration     string `json:"duration"`
	IsShort      bool   `json:"is_short"`
	ViewCount    int    `json:"view_count"`
	Likes        int    `json:"likes"`
	CommentsCount int   `json:"comments_count"`
	ChannelName  string `json:"channel_name"`
	ChannelURL   string `json:"channel_url"`
	ChannelID    string `json:"channel_id"`
}

// YouTubeChannel represents a YouTube channel.
type YouTubeChannel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Description     string `json:"description"`
	ThumbnailURL    string `json:"thumbnail_url"`
	SubscriberCount int    `json:"subscriber_count"`
	VideoCount      int    `json:"video_count"`
}

// YouTubeComment represents a YouTube comment.
type YouTubeComment struct {
	Author      string `json:"author"`
	Text        string `json:"text"`
	Likes       int    `json:"likes"`
	PublishedAt string `json:"published_at"`
}
