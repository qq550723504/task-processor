package sourcing

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/model"
)

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
