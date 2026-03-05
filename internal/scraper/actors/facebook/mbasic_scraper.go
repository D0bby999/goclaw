package facebook

// FacebookInput defines scraping parameters for the Facebook actor.
type FacebookInput struct {
	PageURLs       []string `json:"page_urls"`
	MaxResults     int      `json:"max_results"`
	RequestDelayMs int      `json:"request_delay_ms"`
	Cookies        string   `json:"cookies,omitempty"`
}

// FacebookPost represents a single Facebook post.
type FacebookPost struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	AuthorName string   `json:"author_name"`
	AuthorURL  string   `json:"author_url"`
	URL        string   `json:"url"`
	Timestamp  string   `json:"timestamp"`
	Likes      int      `json:"likes"`
	Comments   int      `json:"comments"`
	Shares     int      `json:"shares"`
	ImageURLs  []string `json:"image_urls,omitempty"`
}

// hasData returns true if the post has meaningful content.
func (p FacebookPost) hasData() bool {
	return p.ID != "" && (p.Text != "" || p.URL != "")
}
