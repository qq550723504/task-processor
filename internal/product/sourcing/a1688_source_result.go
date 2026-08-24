package sourcing

import (
	"net/url"
	"regexp"
	"strings"
)

var alibaba1688OfferIDPattern = regexp.MustCompile(`(?i)(?:/offer/|offer[/=])(\d+)`)

// Alibaba1688CrawlRequestInput is the source-side request context for a 1688 product URL.
type Alibaba1688CrawlRequestInput struct {
	URL       string `json:"url"`
	AccountID int64  `json:"account_id,omitempty"`
	// StoreID is reserved for neutral source identity only. It is not accepted
	// as the 1688 login-account identifier at the HTTP/application boundary.
	StoreID int64 `json:"store_id,omitempty"`
}

// Alibaba1688SourceProductResult is a normalized 1688 crawler result with its
// source identity.
type Alibaba1688SourceProductResult struct {
	Identity SourceIdentity
	Product  *Alibaba1688ProductSnapshot
	Error    error
}

// Alibaba1688CrawlResultInput is the neutral result shape passed from a 1688
// integration adapter into product sourcing.
type Alibaba1688CrawlResultInput struct {
	Product *Alibaba1688ProductSnapshot
	Error   error
}

// Alibaba1688SourceRequest builds the stable source request identity for a
// 1688 product URL. Standard offer URLs use the numeric offer ID; non-standard
// URLs fall back to the cleaned URL so the source remains traceable.
func Alibaba1688SourceRequest(input Alibaba1688CrawlRequestInput) SourceRequest {
	cleanURL := NormalizeAlibaba1688URL(input.URL)
	productID := ExtractAlibaba1688ProductID(cleanURL)
	if productID == "" {
		productID = cleanURL
	}
	return SourceRequest{
		Platform:  "1688",
		Region:    "cn",
		ProductID: productID,
		StoreID:   input.StoreID,
	}
}

// NormalizeAlibaba1688SourceResult attaches a stable source identity to one
// raw 1688 crawler result.
func NormalizeAlibaba1688SourceResult(input Alibaba1688CrawlRequestInput, product *Alibaba1688ProductSnapshot, err error) Alibaba1688SourceProductResult {
	return Alibaba1688SourceProductResult{
		Identity: Alibaba1688SourceRequest(input).Identity(),
		Product:  product,
		Error:    err,
	}
}

// NormalizeAlibaba1688BatchResults aligns 1688 batch results with the
// requested source identities. Missing trailing results become empty source
// results, preserving request/result accounting without guessing failures.
func NormalizeAlibaba1688BatchResults(requests []Alibaba1688CrawlRequestInput, results []Alibaba1688CrawlResultInput) []Alibaba1688SourceProductResult {
	normalized := make([]Alibaba1688SourceProductResult, 0, len(requests))
	for index, req := range requests {
		item := NormalizeAlibaba1688SourceResult(req, nil, nil)
		if index < len(results) {
			item.Product = results[index].Product
			item.Error = results[index].Error
		}
		normalized = append(normalized, item)
	}
	return normalized
}

// NormalizeAlibaba1688URL trims and lightly normalizes a 1688 source URL for
// identity fallback. It intentionally does not validate crawler reachability.
func NormalizeAlibaba1688URL(rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(cleanURL), "http://") && !strings.HasPrefix(strings.ToLower(cleanURL), "https://") {
		cleanURL = "https://" + cleanURL
	}
	parsed, err := url.Parse(cleanURL)
	if err != nil {
		return cleanURL
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String()
}

// ExtractAlibaba1688ProductID extracts a 1688 offer ID from common detail URLs.
func ExtractAlibaba1688ProductID(rawURL string) string {
	if matches := alibaba1688OfferIDPattern.FindStringSubmatch(strings.TrimSpace(rawURL)); len(matches) > 1 {
		return matches[1]
	}
	return ""
}
