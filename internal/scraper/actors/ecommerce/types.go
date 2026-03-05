package ecommerce

// EcommerceInput defines scraping parameters for the Ecommerce actor.
type EcommerceInput struct {
	StartURLs      []string `json:"start_urls"`
	MaxResults     int      `json:"max_results"`
	RequestDelayMs int      `json:"request_delay_ms"`
}

// Product represents a scraped product listing.
type Product struct {
	Title        string  `json:"title"`
	URL          string  `json:"url"`
	ImageURL     string  `json:"image_url"`
	Description  string  `json:"description"`
	Price        string  `json:"price"`
	Currency     string  `json:"currency"`
	Brand        string  `json:"brand"`
	Rating       float64 `json:"rating"`
	ReviewCount  int     `json:"review_count"`
	Availability string  `json:"availability"`
	Platform     string  `json:"platform"`
}

// Platform identifies the ecommerce site.
type Platform string

const (
	PlatformAmazon  Platform = "amazon"
	PlatformEbay    Platform = "ebay"
	PlatformWalmart Platform = "walmart"
	PlatformGeneric Platform = "generic"
)
