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

// AmazonZipcodePolicy owns source-specific default zipcode behavior.
type AmazonZipcodePolicy interface {
	ShouldUseDefaultZipcode(region string) bool
	DefaultZipcode(region string) string
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
	if err := p.validateRegion(req.Region); err != nil {
		return nil, err
	}
	zipcode := p.ResolveZipcode(req.Region, req.Zipcode)
	requests := make([]model.ProductRequest, 0, len(productIDs))
	for _, productID := range productIDs {
		requests = append(requests, model.ProductRequest{
			URL:     p.DomainResolver.BuildAmazonProductURL(req.Region, productID),
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
