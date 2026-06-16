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

type recordingProductFetcherRawJSONClient struct {
	created []string
}

func (s *recordingProductFetcherRawJSONClient) GetRawJsonData(*RawJsonReq) (*RawJsonResp, error) {
	return nil, errors.New("cache miss")
}

func (s *recordingProductFetcherRawJSONClient) CreateRawJsonData(req *RawJsonCreateReq) (int64, error) {
	s.created = append(s.created, req.ProductID)
	return int64(len(s.created)), nil
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

type selectiveProductFetcherCrawlSource struct {
	products map[string]*model.Product
}

func (s *selectiveProductFetcherCrawlSource) Process(url, zipcode string) (*model.Product, error) {
	return s.ProcessWithContext(context.Background(), url, zipcode)
}

func (s *selectiveProductFetcherCrawlSource) ProcessWithContext(_ context.Context, url, zipcode string) (*model.Product, error) {
	if product, ok := s.products[url]; ok {
		return product, nil
	}
	return nil, errors.New("product not found")
}

func (s *selectiveProductFetcherCrawlSource) Shutdown() {}

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

func TestProductFetcherReturnsErrorWhenCrawlerUnavailableAfterCacheMiss(t *testing.T) {
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, &config.AmazonConfig{Enabled: true}, nil)

	product, err := fetcher.FetchProduct(context.Background(), &FetchRequest{
		Region:    "us",
		ProductID: "B003",
	})
	if err == nil {
		t.Fatal("FetchProduct() error = nil, want crawler unavailable error")
	}
	if product != nil {
		t.Fatalf("FetchProduct() product = %+v, want nil", product)
	}
}

func TestProductFetcherFetchVariantsCachesEachSuccessfulVariantImmediately(t *testing.T) {
	rawClient := &recordingProductFetcherRawJSONClient{}
	source := &selectiveProductFetcherCrawlSource{
		products: map[string]*model.Product{
			"https://www.amazon.com/dp/B-success-1?th=1&psc=1&language=en_US": {Asin: "B-success-1", ShipsFrom: "Amazon.com"},
			"https://www.amazon.com/dp/B-success-2?th=1&psc=1&language=en_US": {Asin: "B-success-2", ShipsFrom: "Amazon.com"},
		},
	}
	fetcher := NewProductFetcher(rawClient, &config.AmazonConfig{Enabled: true}, source)

	variants, err := fetcher.FetchVariants(context.Background(), &FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B-parent",
		Creator:   "tester",
	}, []string{"B-success-1", "B-miss", "B-success-2"})
	if err != nil {
		t.Fatalf("FetchVariants() error = %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("len(variants) = %d, want 2 successful variants", len(variants))
	}
	if len(rawClient.created) != 2 {
		t.Fatalf("CreateRawJsonData() calls = %d, want 2", len(rawClient.created))
	}
	if rawClient.created[0] != "B-success-1" || rawClient.created[1] != "B-success-2" {
		t.Fatalf("created product IDs = %v, want [B-success-1 B-success-2]", rawClient.created)
	}
}
