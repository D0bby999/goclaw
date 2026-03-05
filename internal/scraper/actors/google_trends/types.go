package google_trends

// GoogleTrendsInput defines scraping parameters for the Google Trends actor.
type GoogleTrendsInput struct {
	Keywords                []string `json:"keywords"`
	Geo                     string   `json:"geo"`
	TimeRange               string   `json:"time_range"`
	IncludeInterestOverTime bool     `json:"include_interest_over_time"`
	IncludeRelatedQueries   bool     `json:"include_related_queries"`
	IncludeTrendingSearches bool     `json:"include_trending_searches"`
	TrendingSearchesGeo     string   `json:"trending_searches_geo"`
	RequestDelayMs          int      `json:"request_delay_ms"`
}

// GoogleTrendsResult holds results for a trends query.
type GoogleTrendsResult struct {
	Type             string          `json:"type"`
	Keywords         []string        `json:"keywords"`
	InterestOverTime []InterestPoint `json:"interest_over_time,omitempty"`
	RelatedQueries   []RelatedItem   `json:"related_queries,omitempty"`
	TrendingSearches []TrendingItem  `json:"trending_searches,omitempty"`
}

// InterestPoint is a single data point in interest-over-time.
type InterestPoint struct {
	Time  string `json:"time"`
	Value int    `json:"value"`
}

// RelatedItem is a related or rising query.
type RelatedItem struct {
	Query string `json:"query"`
	Value int    `json:"value"`
	Type  string `json:"type"` // rising, top
}

// TrendingItem is a trending search entry.
type TrendingItem struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Source        string `json:"source"`
	URL           string `json:"url"`
	TrafficVolume string `json:"traffic_volume"`
}
