package amazon

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/model"
	"task-processor/internal/product/sourcing"
)

type stubAmazonDomainResolver struct {
	domain string
}

func (r stubAmazonDomainResolver) GetAmazonDomainByRegion(string) string {
	return r.domain
}

func (r stubAmazonDomainResolver) BuildAmazonProductURL(region, asin string) string {
	return "https://example." + region + "/dp/" + asin
}

type stubAmazonZipcodePolicy struct {
	useDefault bool
	defaultZip string
}

func (p stubAmazonZipcodePolicy) ShouldUseDefaultZipcode(string) bool {
	return p.useDefault
}

func (p stubAmazonZipcodePolicy) DefaultZipcode(string) string {
	return p.defaultZip
}

func TestAmazonCrawlRequestPlannerBuildRequestUsesExplicitZipcode(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
		ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
	}

	got, err := planner.BuildRequest(AmazonCrawlRequestInput{
		Region:    " UK ",
		ProductID: " B001 ",
		Zipcode:   " EC1A 1BB ",
	})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got.URL != "https://example.uk/dp/B001" {
		t.Fatalf("URL = %q, want generated URL", got.URL)
	}
	if got.Zipcode != "EC1A 1BB" {
		t.Fatalf("Zipcode = %q, want explicit zipcode", got.Zipcode)
	}
}

func TestAmazonCrawlRequestPlannerBuildBatchRequestsUsesConfiguredDefaultZipcode(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
		ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:       map[string]string{"uk": "W1A 1AA"},
	}

	got, err := planner.BuildBatchRequests(AmazonCrawlRequestInput{Region: " UK "}, []string{" B001 ", "B002"})
	if err != nil {
		t.Fatalf("BuildBatchRequests() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Zipcode != "W1A 1AA" || got[1].Zipcode != "W1A 1AA" {
		t.Fatalf("zipcodes = %q/%q, want configured default", got[0].Zipcode, got[1].Zipcode)
	}
	if got[0].URL != "https://example.uk/dp/B001" {
		t.Fatalf("got[0].URL = %q, want trimmed first product URL", got[0].URL)
	}
	if got[1].URL != "https://example.uk/dp/B002" {
		t.Fatalf("got[1].URL = %q, want URL for second product", got[1].URL)
	}
}

func TestAmazonCrawlRequestPlannerBuildRequestRejectsUnsupportedRegion(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{DomainResolver: stubAmazonDomainResolver{}}

	_, err := planner.BuildRequest(AmazonCrawlRequestInput{Region: "unknown", ProductID: "B001"})
	if err == nil {
		t.Fatal("BuildRequest() error = nil, want unsupported region error")
	}
}

func TestAmazonCrawlRequestPlannerResolveZipcodeUsesConfiguredDefault(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		ZipcodePolicy: stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:      map[string]string{"uk": "W1A 1AA"},
	}

	got := planner.ResolveZipcode("UK", "")
	if got != "W1A 1AA" {
		t.Fatalf("ResolveZipcode() = %q, want configured default", got)
	}
}

func TestAmazonCrawlRequestPlannerResolveZipcodePreservesExplicit(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		ZipcodePolicy: stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:      map[string]string{"uk": "W1A 1AA"},
	}

	got := planner.ResolveZipcode("UK", " EC1A 1BB ")
	if got != "EC1A 1BB" {
		t.Fatalf("ResolveZipcode() = %q, want explicit zipcode", got)
	}
}

func TestAmazonDefaultZipcodePolicyKeepsSourceDefaults(t *testing.T) {
	policy := AmazonDefaultZipcodePolicy{}

	if policy.ShouldUseDefaultZipcode("us") {
		t.Fatal("ShouldUseDefaultZipcode(us) = true, want false")
	}
	if !policy.ShouldUseDefaultZipcode(" UK ") {
		t.Fatal("ShouldUseDefaultZipcode(UK) = false, want true")
	}
	if got := policy.DefaultZipcode("UK"); got != "SW1A 1AA" {
		t.Fatalf("DefaultZipcode(UK) = %q, want SW1A 1AA", got)
	}
	if got := policy.DefaultZipcode("unknown"); got != "94107" {
		t.Fatalf("DefaultZipcode(unknown) = %q, want fallback 94107", got)
	}
}

func TestAmazonDefaultDomainResolverKeepsSourceURLRules(t *testing.T) {
	resolver := AmazonDefaultDomainResolver{}

	if got := resolver.GetAmazonDomainByRegion(" UK "); got != "amazon.co.uk" {
		t.Fatalf("GetAmazonDomainByRegion(UK) = %q, want amazon.co.uk", got)
	}
	if got := resolver.GetAmazonDomainByRegion("unknown"); got != "amazon.com" {
		t.Fatalf("GetAmazonDomainByRegion(unknown) = %q, want amazon.com", got)
	}
	if got := resolver.BuildAmazonProductURL("UK", "B001"); got != "https://www.amazon.co.uk/dp/B001?th=1&psc=1&language=en_GB" {
		t.Fatalf("BuildAmazonProductURL(UK, B001) = %q, want UK URL with language", got)
	}
	if got := resolver.BuildAmazonProductURL("unknown", "B002"); got != "https://www.amazon.com/dp/B002?th=1&psc=1&language=en_US" {
		t.Fatalf("BuildAmazonProductURL(unknown, B002) = %q, want US fallback URL", got)
	}
}

func TestAmazonSourceEnvelopeMapsProductFacts(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: sourcing.SourceRequest{Region: " UK ", ProductID: "fallback", StoreID: 7},
		Product: &model.Product{
			Asin:        " B001 ",
			ParentAsin:  " PARENT ",
			URL:         " https://www.amazon.co.uk/dp/B001 ",
			Title:       " Test Shirt ",
			Brand:       " Test Brand ",
			Description: " Test description ",
			Currency:    "GBP",
			FinalPrice:  12.34,
			SellerID:    " seller-1 ",
			SellerName:  " Seller One ",
			ImageURL:    " https://img.example/primary.jpg ",
			Images:      []string{"https://img.example/primary.jpg", " https://img.example/side.jpg "},
			Features:    []string{" Soft ", " Washable "},
			Categories:  []string{" Clothing ", " Shirts "},
			Variations: []model.Variation{{
				Name:       "Blue / M",
				Asin:       "B001-BLUE-M",
				Attributes: map[string]any{"Color": "Blue", "Size": "M"},
			}},
		},
		RawSnapshot: "raw-1",
		SourceRunID: "run-1",
		RequestID:   "request-1",
	})

	if envelope.Identity.SourceType != sourcing.SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want crawler", envelope.Identity.SourceType)
	}
	if envelope.Identity.SourcePlatform != AmazonSourcePlatform {
		t.Fatalf("SourcePlatform = %q, want amazon", envelope.Identity.SourcePlatform)
	}
	if envelope.Identity.SourceID != "B001" {
		t.Fatalf("SourceID = %q, want B001", envelope.Identity.SourceID)
	}
	if got := envelope.Identity.Key(); got != "amazon:uk:B001:7" {
		t.Fatalf("Key() = %q, want legacy key with store", got)
	}
	if got := envelope.Identity.SourceKey(); got != "crawler:amazon:B001" {
		t.Fatalf("SourceKey() = %q, want source key", got)
	}
	if envelope.RawReference.ReferenceType != amazonSourceReferenceType || envelope.RawReference.ReferenceID != "B001" {
		t.Fatalf("RawReference = %+v, want Amazon product reference", envelope.RawReference)
	}
	if envelope.ProductCandidate.Title != "Test Shirt" {
		t.Fatalf("Title = %q, want Test Shirt", envelope.ProductCandidate.Title)
	}
	if envelope.ProductCandidate.Attributes["categories"] != "Clothing>Shirts" {
		t.Fatalf("categories = %q, want normalized category path", envelope.ProductCandidate.Attributes["categories"])
	}
	if len(envelope.ProductCandidate.Variants) != 1 {
		t.Fatalf("variants = %d, want 1", len(envelope.ProductCandidate.Variants))
	}
	if envelope.ProductCandidate.Variants[0].Attributes["Color"] != "Blue" {
		t.Fatalf("variant color = %q, want Blue", envelope.ProductCandidate.Variants[0].Attributes["Color"])
	}
	if len(envelope.AssetCandidates) != 2 {
		t.Fatalf("assets = %d, want deduped primary + gallery", len(envelope.AssetCandidates))
	}
	if envelope.AssetCandidates[0].Role != amazonImageRolePrimary {
		t.Fatalf("first asset role = %q, want primary", envelope.AssetCandidates[0].Role)
	}
	if envelope.SupplierOrCostFacts.SupplierID != "seller-1" || envelope.SupplierOrCostFacts.Price != "12.34" {
		t.Fatalf("sourcing.SupplierOrCostFacts = %+v, want seller and price", envelope.SupplierOrCostFacts)
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("Warnings = %+v, want none", envelope.Warnings)
	}
}

func TestAmazonSourceEnvelopeFallsBackToRequestIdentityAndWarnings(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: sourcing.SourceRequest{Region: "us", ProductID: "B-FALLBACK"},
		Product: &model.Product{},
	})

	if envelope.Identity.SourceID != "B-FALLBACK" {
		t.Fatalf("SourceID = %q, want request fallback", envelope.Identity.SourceID)
	}
	if len(envelope.Warnings) != 2 {
		t.Fatalf("Warnings = %+v, want missing title and assets", envelope.Warnings)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	if !codes["missing_title"] || !codes["missing_assets"] {
		t.Fatalf("warning codes = %+v, want missing_title and missing_assets", codes)
	}
}

func TestAmazonSourceEnvelopeHandlesMissingProduct(t *testing.T) {
	envelope := AmazonSourceEnvelope(AmazonSourceEnvelopeInput{
		Request: sourcing.SourceRequest{Region: "us", ProductID: "B001"},
	})

	if envelope.Identity.SourceID != "B001" {
		t.Fatalf("SourceID = %q, want request identity", envelope.Identity.SourceID)
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Code != "missing_product" {
		t.Fatalf("Warnings = %+v, want missing_product", envelope.Warnings)
	}
}

type stubAmazonSourceFetcherSource struct {
	lastURL     string
	lastZipcode string
	lastBatch   []model.ProductRequest
	product     *model.Product
	err         error
	results     []model.ProductResult
}

func (s *stubAmazonSourceFetcherSource) ProcessWithContext(_ context.Context, url string, zipcode string) (*model.Product, error) {
	s.lastURL = url
	s.lastZipcode = zipcode
	return s.product, s.err
}

func (s *stubAmazonSourceFetcherSource) ProcessBatchWithContext(_ context.Context, requests []model.ProductRequest) []model.ProductResult {
	s.lastBatch = requests
	return s.results
}

func TestAmazonSourceFetcherPlansAndExecutesRequest(t *testing.T) {
	source := &stubAmazonSourceFetcherSource{product: &model.Product{Asin: "B001"}}
	fetcher := AmazonSourceFetcher{
		Planner: AmazonCrawlRequestPlanner{
			DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
			ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		},
		Source: source,
	}

	got, err := fetcher.Fetch(context.Background(), AmazonCrawlRequestInput{
		Region:    "uk",
		ProductID: "B001",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got == nil || got.Asin != "B001" {
		t.Fatalf("Fetch() = %+v, want source product", got)
	}
	if source.lastURL != "https://example.uk/dp/B001" {
		t.Fatalf("source URL = %q, want planned URL", source.lastURL)
	}
	if source.lastZipcode != "SW1A 1AA" {
		t.Fatalf("source zipcode = %q, want planned default zipcode", source.lastZipcode)
	}
}

func TestAmazonSourceFetcherReturnsSourceError(t *testing.T) {
	wantErr := errors.New("source failed")
	fetcher := AmazonSourceFetcher{
		Planner: AmazonCrawlRequestPlanner{
			DomainResolver: stubAmazonDomainResolver{domain: "amazon.com"},
		},
		Source: &stubAmazonSourceFetcherSource{err: wantErr},
	}

	_, err := fetcher.Fetch(context.Background(), AmazonCrawlRequestInput{Region: "us", ProductID: "B001"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Fetch() error = %v, want %v", err, wantErr)
	}
}

func TestAmazonSourceFetcherFetchBatchRequiresSource(t *testing.T) {
	fetcher := AmazonSourceFetcher{
		Planner: AmazonCrawlRequestPlanner{
			DomainResolver: stubAmazonDomainResolver{domain: "amazon.com"},
		},
	}

	got, err := fetcher.FetchBatch(context.Background(), AmazonCrawlRequestInput{Region: "us"}, []string{"B001"})
	if err == nil {
		t.Fatal("FetchBatch() error = nil, want configuration error")
	}
	if got != nil {
		t.Fatalf("FetchBatch() results = %+v, want nil", got)
	}
	if err.Error() != "amazon crawler source is not configured" {
		t.Fatalf("FetchBatch() error = %q, want configuration error", err.Error())
	}
}

func TestAmazonSourceFetcherFetchBatchAllowsEmptyBatchWithoutSource(t *testing.T) {
	fetcher := AmazonSourceFetcher{
		Planner: AmazonCrawlRequestPlanner{
			DomainResolver: stubAmazonDomainResolver{domain: "amazon.com"},
		},
	}

	got, err := fetcher.FetchBatch(context.Background(), AmazonCrawlRequestInput{Region: "us"}, nil)
	if err != nil {
		t.Fatalf("FetchBatch(empty) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("FetchBatch(empty) results = %+v, want empty", got)
	}
}

func TestAmazonSourceFetcherFetchBatchUsesBatchSource(t *testing.T) {
	source := &stubAmazonSourceFetcherSource{
		results: []model.ProductResult{{Product: &model.Product{Asin: "B001"}}},
	}
	fetcher := AmazonSourceFetcher{
		Planner: AmazonCrawlRequestPlanner{
			DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
			ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		},
		Source: source,
	}

	got, err := fetcher.FetchBatch(context.Background(), AmazonCrawlRequestInput{Region: "uk"}, []string{"B001", "B002"})
	if err != nil {
		t.Fatalf("FetchBatch() error = %v", err)
	}
	if len(source.lastBatch) != 2 {
		t.Fatalf("len(lastBatch) = %d, want 2", len(source.lastBatch))
	}
	if len(got) != 1 || got[0].Product.Asin != "B001" {
		t.Fatalf("FetchBatch() = %+v, want source results", got)
	}
}

func TestCrawlerPlatformForSourceMapsMarketplaceAliasesToAmazon(t *testing.T) {
	tests := map[string]string{
		"shein": "amazon",
		"SHEIN": "amazon",
		"temu":  "amazon",
		"TEMU":  "amazon",
	}

	for platform, want := range tests {
		got := CrawlerPlatformForSource(platform)
		if got != want {
			t.Fatalf("CrawlerPlatformForSource(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestCrawlerPlatformForSourcePreservesNativePlatform(t *testing.T) {
	got := CrawlerPlatformForSource("Amazon")
	if got != "Amazon" {
		t.Fatalf("CrawlerPlatformForSource() = %q, want original platform", got)
	}
}

func TestSupportsCrawlerSource(t *testing.T) {
	supported := []string{"amazon", "shein", "temu", "1688", " SHEIN "}
	for _, platform := range supported {
		if !SupportsCrawlerSource(platform) {
			t.Fatalf("SupportsCrawlerSource(%q) = false, want true", platform)
		}
	}

	if SupportsCrawlerSource("walmart") {
		t.Fatal("SupportsCrawlerSource(walmart) = true, want false")
	}
}

func TestNormalizeAmazonBatchResultsAlignsResultsWithSourceIdentities(t *testing.T) {
	sourceErr := errors.New("source failed")
	got := NormalizeAmazonBatchResults(
		AmazonCrawlRequestInput{Region: " UK ", Zipcode: " W1A 1AA "},
		[]string{" B001 ", "B002", " B003 "},
		[]model.ProductResult{
			{Product: &model.Product{Asin: "B001"}},
			{Error: sourceErr},
		},
	)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].Identity.Key() != "amazon:uk:B001" || got[0].Product == nil || got[0].Product.Asin != "B001" {
		t.Fatalf("got[0] = %+v, want first product with normalized identity", got[0])
	}
	if got[1].Identity.Key() != "amazon:uk:B002" || !errors.Is(got[1].Error, sourceErr) {
		t.Fatalf("got[1] = %+v, want second error with normalized identity", got[1])
	}
	if got[2].Identity.Key() != "amazon:uk:B003" || got[2].Product != nil || got[2].Error != nil {
		t.Fatalf("got[2] = %+v, want missing source result placeholder", got[2])
	}
}

func TestNormalizeAmazonBatchResultsReturnsEmptyForNoProductIDs(t *testing.T) {
	got := NormalizeAmazonBatchResults(AmazonCrawlRequestInput{Region: "us"}, nil, []model.ProductResult{{Product: &model.Product{Asin: "unused"}}})
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}
