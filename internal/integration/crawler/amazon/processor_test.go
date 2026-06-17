package amazon

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/model"
)

type stubAmazonCrawlerSource struct {
	lastURL     string
	lastZipcode string
	lastBatch   []model.ProductRequest
	product     *model.Product
	err         error
	results     []model.ProductResult
}

type shutdownAmazonCrawlerSource struct {
	stubAmazonCrawlerSource
	shutdownCalled bool
}

func (s *stubAmazonCrawlerSource) ProcessWithContext(_ context.Context, url string, zipcode string) (*model.Product, error) {
	s.lastURL = url
	s.lastZipcode = zipcode
	return s.product, s.err
}

func (s *stubAmazonCrawlerSource) ProcessBatchWithContext(_ context.Context, requests []model.ProductRequest) []model.ProductResult {
	s.lastBatch = requests
	return s.results
}

func (s *shutdownAmazonCrawlerSource) Shutdown() {
	s.shutdownCalled = true
}

func TestProcessorProcessDelegatesToSource(t *testing.T) {
	want := &model.Product{Asin: "B001"}
	source := &stubAmazonCrawlerSource{product: want}
	processor := NewProcessor(source)

	got, err := processor.Process(context.Background(), model.ProductRequest{
		URL:     "https://www.amazon.com/dp/B001",
		Zipcode: "94107",
	})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if got != want {
		t.Fatal("Process() did not return source product")
	}
	if source.lastURL != "https://www.amazon.com/dp/B001" || source.lastZipcode != "94107" {
		t.Fatalf("source args = %q/%q, want request URL/zipcode", source.lastURL, source.lastZipcode)
	}
}

func TestProcessorProcessReturnsSourceError(t *testing.T) {
	wantErr := errors.New("source failed")
	processor := NewProcessor(&stubAmazonCrawlerSource{err: wantErr})

	_, err := processor.Process(context.Background(), model.ProductRequest{URL: "https://example.test"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Process() error = %v, want %v", err, wantErr)
	}
}

func TestProcessorProcessBatchDelegatesToSource(t *testing.T) {
	requests := []model.ProductRequest{{URL: "https://example.test/1"}, {URL: "https://example.test/2"}}
	results := []model.ProductResult{{Product: &model.Product{Asin: "B001"}}}
	source := &stubAmazonCrawlerSource{results: results}
	processor := NewProcessor(source)

	got := processor.ProcessBatch(context.Background(), requests)
	if len(source.lastBatch) != len(requests) {
		t.Fatalf("len(lastBatch) = %d, want %d", len(source.lastBatch), len(requests))
	}
	if len(got) != len(results) || got[0].Product.Asin != "B001" {
		t.Fatalf("ProcessBatch() = %+v, want source results", got)
	}
}

func TestProcessorProcessBatchWithoutSourceReturnsErrors(t *testing.T) {
	var processor *Processor

	got := processor.ProcessBatch(context.Background(), []model.ProductRequest{{}, {}})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Error == nil || got[1].Error == nil {
		t.Fatalf("got = %+v, want errors for each request", got)
	}
}

func TestLegacyCrawlSourceProcessDelegatesToSource(t *testing.T) {
	want := &model.Product{Asin: "B002"}
	source := &stubAmazonCrawlerSource{product: want}
	crawlSource := &LegacyCrawlSource{source: source}

	got, err := crawlSource.Process("https://www.amazon.com/dp/B002", "10001")
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if got != want {
		t.Fatal("Process() did not return source product")
	}
	if source.lastURL != "https://www.amazon.com/dp/B002" || source.lastZipcode != "10001" {
		t.Fatalf("source args = %q/%q, want request URL/zipcode", source.lastURL, source.lastZipcode)
	}
}

func TestLegacyCrawlSourceProcessBatchWithoutSourceReturnsErrors(t *testing.T) {
	var crawlSource *LegacyCrawlSource

	got := crawlSource.ProcessBatchWithContext(context.Background(), []model.ProductRequest{{}, {}})
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Error == nil || got[1].Error == nil {
		t.Fatalf("got = %+v, want errors for each request", got)
	}
}

func TestLegacyCrawlSourceShutdownDelegatesWhenAvailable(t *testing.T) {
	source := &shutdownAmazonCrawlerSource{}
	crawlSource := &LegacyCrawlSource{source: source}

	crawlSource.Shutdown()

	if !source.shutdownCalled {
		t.Fatal("Shutdown() did not delegate to source")
	}
}
