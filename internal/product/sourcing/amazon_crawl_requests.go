package sourcing

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
)

// AmazonDomainResolver builds source URLs for Amazon crawler requests.
type AmazonDomainResolver interface {
	GetAmazonDomainByRegion(region string) string
	BuildAmazonProductURL(region, asin string) string
}

// AmazonDefaultDomainResolver preserves Amazon source domain and language URL rules.
type AmazonDefaultDomainResolver struct{}

var amazonDefaultDomains = map[string]string{
	"us": "amazon.com",
	"uk": "amazon.co.uk",
	"de": "amazon.de",
	"fr": "amazon.fr",
	"it": "amazon.it",
	"es": "amazon.es",
	"ca": "amazon.ca",
	"jp": "amazon.co.jp",
	"au": "amazon.com.au",
	"mx": "amazon.com.mx",
	"br": "amazon.com.br",
	"in": "amazon.in",
	"ae": "amazon.ae",
	"sa": "amazon.sa",
}

var amazonDefaultLanguages = map[string]string{
	"us": "en_US",
	"uk": "en_GB",
	"de": "de_DE",
	"fr": "fr_FR",
	"it": "it_IT",
	"es": "es_ES",
	"ca": "en_CA",
	"jp": "ja_JP",
	"au": "en_AU",
	"mx": "es_MX",
	"br": "pt_BR",
	"in": "en_IN",
	"ae": "en_AE",
	"sa": "en_AE",
}

// GetAmazonDomainByRegion returns the Amazon domain for a source region.
func (AmazonDefaultDomainResolver) GetAmazonDomainByRegion(region string) string {
	if domain := strings.TrimSpace(amazonDefaultDomains[normalizeAmazonRegion(region)]); domain != "" {
		return domain
	}
	return amazonDefaultDomains["us"]
}

// BuildAmazonProductURL builds the canonical Amazon product URL for crawler requests.
func (r AmazonDefaultDomainResolver) BuildAmazonProductURL(region, asin string) string {
	domain := r.GetAmazonDomainByRegion(region)
	language := r.languageByRegion(region)
	return "https://www." + domain + "/dp/" + asin + "?th=1&psc=1&language=" + language
}

func (AmazonDefaultDomainResolver) languageByRegion(region string) string {
	if language := strings.TrimSpace(amazonDefaultLanguages[normalizeAmazonRegion(region)]); language != "" {
		return language
	}
	return amazonDefaultLanguages["us"]
}

// AmazonZipcodePolicy owns source-specific default zipcode behavior.
type AmazonZipcodePolicy interface {
	ShouldUseDefaultZipcode(region string) bool
	DefaultZipcode(region string) string
}

// AmazonDefaultZipcodePolicy preserves source-level default zipcode behavior.
type AmazonDefaultZipcodePolicy struct{}

var amazonDefaultZipcodes = map[string]string{
	"us": "94107",
	"uk": "SW1A 1AA",
	"de": "10115",
	"fr": "75001",
	"jp": "153-0064",
	"ca": "M5H 2N2",
	"it": "00118",
	"es": "28001",
	"in": "110001",
	"mx": "11000",
	"br": "01310-100",
	"au": "2000",
	"ae": "00000",
	"sa": "11564",
}

// ShouldUseDefaultZipcode reports whether a region should receive a source default.
func (AmazonDefaultZipcodePolicy) ShouldUseDefaultZipcode(region string) bool {
	region = normalizeAmazonRegion(region)
	return region != "" && region != "us"
}

// DefaultZipcode returns the source default zipcode for a region.
func (AmazonDefaultZipcodePolicy) DefaultZipcode(region string) string {
	if zipcode := strings.TrimSpace(amazonDefaultZipcodes[normalizeAmazonRegion(region)]); zipcode != "" {
		return zipcode
	}
	return amazonDefaultZipcodes["us"]
}

func normalizeAmazonRegion(region string) string {
	return strings.ToLower(strings.TrimSpace(region))
}

// AmazonCrawlRequestPlanner converts product fetch requests into raw Amazon
// crawler requests without depending on a concrete crawler implementation.
type AmazonCrawlRequestPlanner struct {
	DomainResolver AmazonDomainResolver
	ZipcodePolicy  AmazonZipcodePolicy
	Zipcodes       map[string]string
}

// AmazonCrawlRequestInput is the product-side source request data needed to
// build an Amazon crawler request.
type AmazonCrawlRequestInput struct {
	Region    string
	ProductID string
	Zipcode   string
}

// BuildRequest builds one Amazon crawler request from a product fetch request.
func (p AmazonCrawlRequestPlanner) BuildRequest(req AmazonCrawlRequestInput) (model.ProductRequest, error) {
	req = normalizeAmazonCrawlRequestInput(req)
	if err := p.validateRegion(req.Region); err != nil {
		return model.ProductRequest{}, err
	}
	return model.ProductRequest{
		URL:     p.DomainResolver.BuildAmazonProductURL(req.Region, req.ProductID),
		Zipcode: p.ResolveZipcode(req.Region, req.Zipcode),
	}, nil
}

// BuildBatchRequests builds Amazon crawler requests for multiple product IDs.
func (p AmazonCrawlRequestPlanner) BuildBatchRequests(req AmazonCrawlRequestInput, productIDs []string) ([]model.ProductRequest, error) {
	req = normalizeAmazonCrawlRequestInput(req)
	if err := p.validateRegion(req.Region); err != nil {
		return nil, err
	}
	zipcode := p.ResolveZipcode(req.Region, req.Zipcode)
	requests := make([]model.ProductRequest, 0, len(productIDs))
	for _, productID := range productIDs {
		variantReq := VariantSourceRequest(SourceRequest{
			Platform:  "amazon",
			Region:    req.Region,
			ProductID: req.ProductID,
			Zipcode:   req.Zipcode,
		}, productID)
		requests = append(requests, model.ProductRequest{
			URL:     p.DomainResolver.BuildAmazonProductURL(variantReq.Region, variantReq.ProductID),
			Zipcode: zipcode,
		})
	}
	return requests, nil
}

func (p AmazonCrawlRequestPlanner) validateRegion(region string) error {
	if p.DomainResolver == nil {
		return fmt.Errorf("amazon domain resolver is not configured")
	}
	domain := p.DomainResolver.GetAmazonDomainByRegion(region)
	if domain == "" {
		return fmt.Errorf("不支持的地区: %s", region)
	}
	return nil
}

// ResolveZipcode applies explicit zipcodes, configured defaults, and legacy
// source-specific defaults in one reusable place.
func (p AmazonCrawlRequestPlanner) ResolveZipcode(region, explicit string) string {
	zipcode := strings.TrimSpace(explicit)
	if zipcode != "" {
		return zipcode
	}
	if p.ZipcodePolicy == nil || !p.ZipcodePolicy.ShouldUseDefaultZipcode(region) {
		return ""
	}
	if p.Zipcodes != nil {
		if configured := strings.TrimSpace(p.Zipcodes[strings.ToLower(region)]); configured != "" {
			return configured
		}
	}
	return p.ZipcodePolicy.DefaultZipcode(strings.ToLower(region))
}

func normalizeAmazonCrawlRequestInput(req AmazonCrawlRequestInput) AmazonCrawlRequestInput {
	normalized := NormalizeSourceRequest(SourceRequest{
		Platform:  "amazon",
		Region:    req.Region,
		ProductID: req.ProductID,
		Zipcode:   req.Zipcode,
	})
	return AmazonCrawlRequestInput{
		Region:    normalized.Region,
		ProductID: normalized.ProductID,
		Zipcode:   normalized.Zipcode,
	}
}
