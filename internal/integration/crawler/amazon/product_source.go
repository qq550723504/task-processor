package amazon

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/product/sourcing"
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
		variantReq := sourcing.VariantSourceRequest(sourcing.SourceRequest{
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
	normalized := sourcing.NormalizeSourceRequest(sourcing.SourceRequest{
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

const (
	AmazonSourcePlatform = "amazon"

	amazonSourceReferenceType = "amazon_product"
	amazonImageRolePrimary    = "primary"
	amazonImageRoleGallery    = "gallery"
)

// AmazonSourceEnvelopeInput is the product-sourcing view of one Amazon crawler
// result. It keeps crawler execution details out of product sourcing while still
// preserving source request context for identity and traceability.
type AmazonSourceEnvelopeInput struct {
	Request     sourcing.SourceRequest
	Product     *model.Product
	RawSnapshot string
	SourceRunID string
	RequestID   string
}

// AmazonSourceEnvelope maps one Amazon crawler product into the neutral product
// sourcing envelope. The mapper intentionally produces platform-neutral product
// and asset candidates only; target marketplace publish payloads belong in
// marketplace packages.
func AmazonSourceEnvelope(input AmazonSourceEnvelopeInput) sourcing.SourceEnvelope {
	req := sourcing.NormalizeSourceRequest(input.Request)
	product := input.Product

	identity := amazonSourceIdentity(req, product)
	envelope := sourcing.SourceEnvelope{
		Identity:     identity,
		RawReference: amazonRawReference(product, input.RawSnapshot),
		Trace: sourcing.SourceTrace{
			SourceRunID: strings.TrimSpace(input.SourceRunID),
			RequestID:   strings.TrimSpace(input.RequestID),
		},
	}
	if product == nil {
		envelope.Warnings = append(envelope.Warnings, sourcing.SourceWarning{
			Code:    "missing_product",
			Message: "Amazon source product is missing",
		})
		return envelope.Normalize()
	}

	envelope.ProductCandidate = amazonProductCandidate(product)
	envelope.AssetCandidates = amazonAssetCandidates(product)
	envelope.SupplierOrCostFacts = amazonSupplierOrCostFacts(product)
	envelope.Warnings = amazonSourceWarnings(identity, product, envelope)
	return envelope.Normalize()
}

func amazonSourceIdentity(req sourcing.SourceRequest, product *model.Product) sourcing.SourceIdentity {
	id := sourcing.SourceIdentity{
		SourceType:     sourcing.SourceTypeCrawler,
		SourcePlatform: AmazonSourcePlatform,
		Region:         req.Region,
		Platform:       AmazonSourcePlatform,
		StoreID:        req.StoreID,
	}
	if product != nil {
		id.SourceID = strings.TrimSpace(product.Asin)
		id.SourceURL = strings.TrimSpace(product.URL)
		id.ProductID = strings.TrimSpace(product.Asin)
	}
	if id.SourceID == "" {
		id.SourceID = req.ProductID
		id.ProductID = req.ProductID
	}
	return sourcing.NormalizeSourceIdentity(id)
}

func amazonRawReference(product *model.Product, snapshot string) sourcing.RawSourceReference {
	ref := sourcing.RawSourceReference{
		ReferenceType: amazonSourceReferenceType,
		SnapshotID:    strings.TrimSpace(snapshot),
	}
	if product == nil {
		return ref
	}
	ref.ReferenceID = strings.TrimSpace(product.Asin)
	ref.URL = strings.TrimSpace(product.URL)
	return ref
}

func amazonProductCandidate(product *model.Product) sourcing.ProductCandidate {
	attributes := map[string]string{}
	addStringAttribute(attributes, "asin", product.Asin)
	addStringAttribute(attributes, "parent_asin", product.ParentAsin)
	addStringAttribute(attributes, "availability", product.Availability)
	addStringAttribute(attributes, "bs_category", product.BsCategory)
	addStringAttribute(attributes, "root_bs_category", product.RootBsCategory)
	addStringAttribute(attributes, "product_dimensions", product.ProductDimensions)
	addStringAttribute(attributes, "item_weight", product.ItemWeight)
	addStringAttribute(attributes, "model_number", product.ModelNumber)
	addStringAttribute(attributes, "department", product.Department)
	addStringAttribute(attributes, "manufacturer", product.Manufacturer)
	addStringAttribute(attributes, "country_of_origin", product.CountryOfOrigin)
	addStringAttribute(attributes, "ships_from", product.ShipsFrom)
	addStringAttribute(attributes, "domain", product.Domain)
	if len(product.Categories) > 0 {
		addStringAttribute(attributes, "categories", strings.Join(trimNonEmptyStrings(product.Categories), ">"))
	}
	if len(product.Features) > 0 {
		addStringAttribute(attributes, "features", strings.Join(trimNonEmptyStrings(product.Features), "\n"))
	}

	return sourcing.ProductCandidate{
		Title:       strings.TrimSpace(product.Title),
		Description: amazonProductDescription(product),
		Brand:       strings.TrimSpace(product.Brand),
		Attributes:  attributes,
		Variants:    amazonVariantCandidates(product.Variations),
	}
}

func amazonProductDescription(product *model.Product) string {
	if description := strings.TrimSpace(product.Description); description != "" {
		return description
	}
	parts := make([]string, 0, len(product.ProductDescription))
	for _, description := range product.ProductDescription {
		if text := strings.TrimSpace(description.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func amazonVariantCandidates(variations []model.Variation) []sourcing.ProductVariantCandidate {
	if len(variations) == 0 {
		return nil
	}
	candidates := make([]sourcing.ProductVariantCandidate, 0, len(variations))
	for _, variation := range variations {
		candidate := sourcing.ProductVariantCandidate{
			SourceID:   strings.TrimSpace(variation.Asin),
			Title:      strings.TrimSpace(variation.Name),
			Attributes: stringifyAttributes(variation.Attributes),
		}
		if candidate.SourceID == "" && candidate.Title == "" && len(candidate.Attributes) == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func amazonAssetCandidates(product *model.Product) []sourcing.AssetCandidate {
	seen := map[string]struct{}{}
	assets := make([]sourcing.AssetCandidate, 0, len(product.Images)+1)
	appendAsset := func(url, role string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		assets = append(assets, sourcing.AssetCandidate{
			SourceID:  url,
			URL:       url,
			MediaType: "image",
			Role:      role,
		})
	}
	appendAsset(product.ImageURL, amazonImageRolePrimary)
	for _, image := range product.Images {
		appendAsset(image, amazonImageRoleGallery)
	}
	return assets
}

func amazonSupplierOrCostFacts(product *model.Product) sourcing.SupplierOrCostFacts {
	facts := map[string]string{}
	addStringAttribute(facts, "buybox_seller", product.BuyboxSeller)
	if product.NumberOfSellers > 0 {
		facts["number_of_sellers"] = strconv.Itoa(product.NumberOfSellers)
	}
	return sourcing.SupplierOrCostFacts{
		SupplierID:   strings.TrimSpace(product.SellerID),
		SupplierName: strings.TrimSpace(product.SellerName),
		Currency:     strings.TrimSpace(product.Currency),
		Cost:         formatOptionalPrice(product.FinalPrice),
		Price:        formatOptionalPrice(product.FinalPrice),
		Facts:        facts,
	}
}

func amazonSourceWarnings(identity sourcing.SourceIdentity, product *model.Product, envelope sourcing.SourceEnvelope) []sourcing.SourceWarning {
	warnings := []sourcing.SourceWarning{}
	validation := identity.Validation()
	if validation.MissingSourceID {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_source_id", Field: "asin", Message: "Amazon source product is missing ASIN"})
	}
	if strings.TrimSpace(product.Title) == "" {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_title", Field: "title", Message: "Amazon source product is missing title"})
	}
	if len(envelope.AssetCandidates) == 0 {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_assets", Field: "images", Message: "Amazon source product has no image assets"})
	}
	return warnings
}

func addStringAttribute(attributes map[string]string, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		attributes[key] = value
	}
}

func trimNonEmptyStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func stringifyAttributes(attributes map[string]any) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range attributes {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		out[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return out
}

func formatOptionalPrice(price float64) string {
	if price <= 0 {
		return ""
	}
	return strconv.FormatFloat(price, 'f', -1, 64)
}

// AmazonCrawlerSource is the minimal source capability needed to fetch one
// Amazon product.
type AmazonCrawlerSource interface {
	ProcessWithContext(ctx context.Context, url string, zipcode string) (*model.Product, error)
}

// AmazonBatchCrawlerSource is the optional batch capability for Amazon source fetches.
type AmazonBatchCrawlerSource interface {
	ProcessBatchWithContext(ctx context.Context, requests []model.ProductRequest) []model.ProductResult
}

// AmazonSourceFetcher plans and executes Amazon source product fetches.
type AmazonSourceFetcher struct {
	Planner AmazonCrawlRequestPlanner
	Source  AmazonCrawlerSource
}

// Configured reports whether the fetcher has an executable source.
func (f AmazonSourceFetcher) Configured() bool {
	return f.Source != nil
}

// Fetch plans a crawler request and delegates execution to the source adapter.
func (f AmazonSourceFetcher) Fetch(ctx context.Context, input AmazonCrawlRequestInput) (*model.Product, error) {
	if f.Source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	req, err := f.Planner.BuildRequest(input)
	if err != nil {
		return nil, err
	}
	return f.Source.ProcessWithContext(ctx, req.URL, req.Zipcode)
}

// FetchBatch plans crawler requests and delegates batch execution to the source
// when available.
func (f AmazonSourceFetcher) FetchBatch(ctx context.Context, input AmazonCrawlRequestInput, productIDs []string) ([]model.ProductResult, error) {
	requests, err := f.Planner.BuildBatchRequests(input, productIDs)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return []model.ProductResult{}, nil
	}
	if f.Source == nil {
		return nil, fmt.Errorf("amazon crawler source is not configured")
	}
	batchSource, ok := f.Source.(AmazonBatchCrawlerSource)
	if !ok || batchSource == nil {
		return f.fetchBatchSequentially(ctx, requests), nil
	}
	return batchSource.ProcessBatchWithContext(ctx, requests), nil
}

func (f AmazonSourceFetcher) fetchBatchSequentially(ctx context.Context, requests []model.ProductRequest) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))
	for i, req := range requests {
		product, err := f.Source.ProcessWithContext(ctx, req.URL, req.Zipcode)
		results[i] = model.ProductResult{Product: product, Error: err}
	}
	return results
}

// CrawlerPlatformForSource maps product source platforms onto the crawler
// platform that can fetch their source product data.
func CrawlerPlatformForSource(platform string) string {
	trimmed := strings.TrimSpace(platform)
	switch strings.ToLower(trimmed) {
	case "shein", "temu":
		return "amazon"
	default:
		return trimmed
	}
}

// SupportsCrawlerSource reports whether the platform has a crawler-backed
// product source path.
func SupportsCrawlerSource(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "amazon", "shein", "temu", "1688":
		return true
	default:
		return false
	}
}

// SourceProductResult is a normalized product result with its source identity.
type SourceProductResult struct {
	Identity sourcing.SourceIdentity
	Product  *model.Product
	Error    error
}

// NormalizeAmazonBatchResults aligns raw Amazon crawler batch results with the
// requested source identities. Missing source results are represented as empty
// product results so callers can keep request/result accounting stable.
func NormalizeAmazonBatchResults(input AmazonCrawlRequestInput, productIDs []string, results []model.ProductResult) []SourceProductResult {
	input = normalizeAmazonCrawlRequestInput(input)
	normalized := make([]SourceProductResult, 0, len(productIDs))
	for index, productID := range productIDs {
		req := sourcing.VariantSourceRequest(sourcing.SourceRequest{
			Platform:  "amazon",
			Region:    input.Region,
			ProductID: input.ProductID,
			Zipcode:   input.Zipcode,
		}, productID)
		item := SourceProductResult{Identity: req.Identity()}
		if index < len(results) {
			item.Product = results[index].Product
			item.Error = results[index].Error
		}
		normalized = append(normalized, item)
	}
	return normalized
}
