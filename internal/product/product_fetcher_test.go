package product

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/model"
)

type stubProductFetcherRawJSONClient struct{}

func (stubProductFetcherRawJSONClient) GetRawJsonData(*RawJsonReq) (*RawJsonResp, error) {
	return nil, errors.New("cache miss")
}

func (stubProductFetcherRawJSONClient) CreateRawJsonData(*RawJsonCreateReq) (int64, error) {
	return 0, nil
}

type stubProductFetcherCrawlSource struct {
	lastURL     string
	lastZipcode string
}

func (s *stubProductFetcherCrawlSource) Process(url, zipcode string) (*model.Product, error) {
	return s.ProcessWithContext(context.Background(), url, zipcode)
}

func (s *stubProductFetcherCrawlSource) ProcessWithContext(_ context.Context, url, zipcode string) (*model.Product, error) {
	s.lastURL = url
	s.lastZipcode = zipcode
	return &model.Product{Asin: "B001"}, nil
}

func (s *stubProductFetcherCrawlSource) Shutdown() {}

func TestProductFetcherUsesExplicitZipcodeForCrawlerRequest(t *testing.T) {
	source := &stubProductFetcherCrawlSource{}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, &config.AmazonConfig{Enabled: true}, source)

	product, err := fetcher.FetchProduct(context.Background(), &FetchRequest{
		Region:    "uk",
		ProductID: "B001",
		Zipcode:   "EC1A 1BB",
	})
	if err != nil {
		t.Fatalf("FetchProduct() error = %v", err)
	}
	if product == nil || product.Asin != "B001" {
		t.Fatalf("FetchProduct() = %+v, want crawler product", product)
	}
	if source.lastZipcode != "EC1A 1BB" {
		t.Fatalf("zipcode = %q, want explicit zipcode", source.lastZipcode)
	}
	if source.lastURL != "https://www.amazon.co.uk/dp/B001?th=1&psc=1&language=en_GB" {
		t.Fatalf("url = %q, want UK Amazon URL", source.lastURL)
	}
}

func TestProductFetcherUsesConfiguredDefaultZipcodeForCrawlerRequest(t *testing.T) {
	source := &stubProductFetcherCrawlSource{}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, &config.AmazonConfig{
		Enabled:  true,
		Zipcodes: map[string]string{"uk": "W1A 1AA"},
	}, source)

	_, err := fetcher.FetchProduct(context.Background(), &FetchRequest{
		Region:    "UK",
		ProductID: "B002",
	})
	if err != nil {
		t.Fatalf("FetchProduct() error = %v", err)
	}
	if source.lastZipcode != "W1A 1AA" {
		t.Fatalf("zipcode = %q, want configured default zipcode", source.lastZipcode)
	}
}

func TestProductFetcherFetchVariantsPreservesExplicitZipcode(t *testing.T) {
	source := &stubProductFetcherCrawlSource{}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, &config.AmazonConfig{Enabled: true}, source)

	_, err := fetcher.FetchVariants(context.Background(), &FetchRequest{
		Region:    "uk",
		ProductID: "B-parent",
		Zipcode:   "EC1A 1BB",
	}, []string{"B-variant"})
	if err != nil {
		t.Fatalf("FetchVariants() error = %v", err)
	}
	if source.lastZipcode != "EC1A 1BB" {
		t.Fatalf("variant zipcode = %q, want inherited explicit zipcode", source.lastZipcode)
	}
}
