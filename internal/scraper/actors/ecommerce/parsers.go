package ecommerce

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/actor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/extractor"
	"github.com/nextlevelbuilder/goclaw/internal/scraper/httpclient"
)

// ParseProductPage extracts product data from HTML using platform-specific selectors.
func ParseProductPage(html, rawURL string, platform Platform) *Product {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	switch platform {
	case PlatformAmazon:
		return parseAmazon(doc, rawURL)
	case PlatformEbay:
		return parseEbay(doc, rawURL)
	case PlatformWalmart:
		return parseWalmart(doc, rawURL)
	default:
		return parseGeneric(html, rawURL)
	}
}

func parseAmazon(doc *goquery.Document, rawURL string) *Product {
	p := &Product{URL: rawURL, Platform: string(PlatformAmazon)}
	p.Title = strings.TrimSpace(doc.Find("#productTitle").Text())
	whole := strings.TrimSpace(doc.Find(".a-price-whole").First().Text())
	frac := strings.TrimSpace(doc.Find(".a-price-fraction").First().Text())
	if whole != "" {
		p.Price = whole + frac
		p.Currency = "USD"
	}
	p.Rating = parseRating(doc.Find("#acrPopover span").First().Text())
	p.ReviewCount = parseCount(doc.Find("#acrCustomerReviewText").First().Text())
	p.ImageURL, _ = doc.Find("#imgTagWrapperId img").First().Attr("src")
	p.Brand = strings.TrimSpace(doc.Find("#bylineInfo").Text())
	return p
}

func parseEbay(doc *goquery.Document, rawURL string) *Product {
	p := &Product{URL: rawURL, Platform: string(PlatformEbay)}
	p.Title = strings.TrimSpace(doc.Find("h1.x-item-title__mainTitle").Text())
	p.Price = strings.TrimSpace(doc.Find(".x-price-primary span").First().Text())
	p.ImageURL, _ = doc.Find("div.ux-image-carousel img").First().Attr("src")
	return p
}

func parseWalmart(doc *goquery.Document, rawURL string) *Product {
	p := &Product{URL: rawURL, Platform: string(PlatformWalmart)}
	p.Title = strings.TrimSpace(doc.Find("h1[itemprop=name]").Text())
	p.Price, _ = doc.Find("span[itemprop=price]").First().Attr("content")
	p.Currency, _ = doc.Find("span[itemprop=priceCurrency]").First().Attr("content")
	p.ImageURL, _ = doc.Find("img.db").First().Attr("src")
	return p
}

func parseGeneric(html, rawURL string) *Product {
	p := &Product{URL: rawURL, Platform: string(PlatformGeneric)}

	// Try JSON-LD structured data first
	items := extractor.ExtractJSONLD(html)
	for _, item := range items {
		typeVal, _ := item["@type"].(string)
		if !strings.EqualFold(typeVal, "Product") {
			continue
		}
		if name, ok := item["name"].(string); ok {
			p.Title = name
		}
		if desc, ok := item["description"].(string); ok {
			p.Description = desc
		}
		if brand, ok := item["brand"].(map[string]any); ok {
			if name, ok := brand["name"].(string); ok {
				p.Brand = name
			}
		}
		if offers, ok := item["offers"].(map[string]any); ok {
			if price, ok := offers["price"].(string); ok {
				p.Price = price
			}
			if currency, ok := offers["priceCurrency"].(string); ok {
				p.Currency = currency
			}
			if avail, ok := offers["availability"].(string); ok {
				p.Availability = avail
			}
		}
		if image, ok := item["image"].(string); ok {
			p.ImageURL = image
		}
		return p
	}

	// Fallback: OpenGraph
	og := extractor.ExtractOpenGraph(html)
	if og != nil {
		p.Title = og.Title
		p.Description = og.Description
		p.ImageURL = og.Image
	}
	return p
}

func parseRating(s string) float64 {
	s = strings.TrimSpace(strings.Split(s, " out")[0])
	s = strings.ReplaceAll(s, ",", ".")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseCount(s string) int {
	s = strings.ReplaceAll(s, ",", "")
	var n int
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}

// EcommerceActor scrapes product pages from Amazon, eBay, Walmart, and generic sites.
type EcommerceActor struct {
	input  EcommerceInput
	client *httpclient.Client
	errors []actor.Error
}

// NewActor constructs an EcommerceActor from a generic input map.
func NewActor(input map[string]any, client *httpclient.Client) (*EcommerceActor, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	var typed EcommerceInput
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, fmt.Errorf("unmarshal input: %w", err)
	}
	if typed.MaxResults <= 0 {
		typed.MaxResults = 20
	}
	if typed.RequestDelayMs <= 0 {
		typed.RequestDelayMs = 2000
	}
	return &EcommerceActor{input: typed, client: client}, nil
}

// Initialize is a no-op.
func (a *EcommerceActor) Initialize(_ context.Context) error { return nil }

// Cleanup is a no-op.
func (a *EcommerceActor) Cleanup() {}

// CollectedErrors returns per-URL errors accumulated during execution.
func (a *EcommerceActor) CollectedErrors() []actor.Error { return a.errors }

// Execute fetches each product URL, detects platform, parses the page.
func (a *EcommerceActor) Execute(ctx context.Context, stats *actor.RunStats) ([]json.RawMessage, error) {
	delay := time.Duration(a.input.RequestDelayMs) * time.Millisecond
	var results []json.RawMessage

	for i, rawURL := range a.input.StartURLs {
		if i >= a.input.MaxResults {
			break
		}
		platform := DetectPlatform(rawURL)

		actor.IncrementRequests(stats)
		resp, err := a.client.Get(ctx, rawURL)
		if err != nil {
			actor.HandleURLError(err, rawURL, &a.errors, stats)
			continue
		}

		product := ParseProductPage(resp.Body, rawURL, platform)
		if product == nil {
			continue
		}

		b, _ := json.Marshal(product)
		results = append(results, b)

		if err := actor.Delay(ctx, delay); err != nil {
			return results, err
		}
	}

	return results, nil
}
