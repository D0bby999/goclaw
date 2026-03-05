package google_search

// GoogleSearchInput defines scraping parameters for the Google Search actor.
type GoogleSearchInput struct {
	Queries          []string `json:"queries"`
	MaxPagesPerQuery int      `json:"max_pages_per_query"`
	CountryCode      string   `json:"country_code"`
	LanguageCode     string   `json:"language_code"`
	MobileResults    bool     `json:"mobile_results"`
	RequestDelayMs   int      `json:"request_delay_ms"`
	ProxyURL         string   `json:"proxy_url,omitempty"`
}

// GoogleSearchResult contains SERP results for a single query.
type GoogleSearchResult struct {
	Query          string          `json:"query"`
	SearchURL      string          `json:"search_url"`
	OrganicResults []OrganicResult `json:"organic_results"`
	PeopleAlsoAsk  []string        `json:"people_also_ask,omitempty"`
	RelatedQueries []string        `json:"related_queries,omitempty"`
	HasNextPage    bool            `json:"has_next_page"`
}

// OrganicResult represents a single organic search result.
type OrganicResult struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
}
