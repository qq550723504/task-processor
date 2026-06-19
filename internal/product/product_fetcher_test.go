package product

import (
	"context"
	"errors"
	"os"
	"strings"
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

func TestProductFetcherUsesSourcingDefaultZipcodePolicy(t *testing.T) {
	source, err := os.ReadFile("product_fetcher.go")
	if err != nil {
		t.Fatalf("ReadFile(product_fetcher.go) error = %v", err)
	}
	content := string(source)
	if !strings.Contains(content, "ZipcodePolicy:  sourcing.AmazonDefaultZipcodePolicy{}") {
		t.Fatal("ProductFetcher should delegate Amazon default zipcode policy to internal/product/sourcing")
	}
	if strings.Contains(content, "type productAmazonZipcodePolicy struct{}") {
		t.Fatal("product_fetcher.go should not own Amazon default zipcode policy")
	}
}

func TestProductFetcherUsesSourcingDefaultDomainResolver(t *testing.T) {
	source, err := os.ReadFile("product_fetcher.go")
	if err != nil {
		t.Fatalf("ReadFile(product_fetcher.go) error = %v", err)
	}
	content := string(source)
	if !strings.Contains(content, "DomainResolver: sourcing.AmazonDefaultDomainResolver{}") {
		t.Fatal("ProductFetcher should delegate Amazon domain and URL planning to internal/product/sourcing")
	}
	if strings.Contains(content, "DomainResolver: NewDomainResolver()") {
		t.Fatal("product_fetcher.go should not construct the product package Amazon domain resolver")
	}
}

func TestProductDomainResolverCompatibilityLayerIsRetired(t *testing.T) {
	if _, err := os.Stat("domain_resolver.go"); err == nil {
		t.Fatal("domain_resolver.go still exists; Amazon source URL and zipcode rules belong in internal/product/sourcing")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat domain_resolver.go: %v", err)
	}
}

func TestProductRepositoryServiceCompatibilityLayerIsRetired(t *testing.T) {
	for _, file := range []string{"repository.go", "service.go"} {
		if _, err := os.Stat(file); err == nil {
			t.Fatalf("%s still exists; use ProductFetcher and product/sourcing instead of the unwired repository-style product crawler service", file)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", file, err)
		}
	}
}

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

func TestProductFetcherFetchVariantsPreservesRequestedASINWhenCrawlerRedirects(t *testing.T) {
	rawClient := &recordingProductFetcherRawJSONClient{}
	source := &selectiveProductFetcherCrawlSource{
		products: map[string]*model.Product{
			"https://www.amazon.com/dp/B-requested?th=1&psc=1&language=en_US": {
				Asin:       "B-redirected",
				ParentAsin: "PARENT-1",
				ShipsFrom:  "Amazon.com",
			},
		},
	}
	fetcher := NewProductFetcher(rawClient, &config.AmazonConfig{Enabled: true}, source)

	variants, err := fetcher.FetchVariants(context.Background(), &FetchRequest{
		TenantID:  1,
		Platform:  "amazon",
		Region:    "us",
		ProductID: "B-parent",
		Creator:   "tester",
	}, []string{"B-requested"})
	if err != nil {
		t.Fatalf("FetchVariants() error = %v", err)
	}
	if len(variants) != 1 {
		t.Fatalf("len(variants) = %d, want 1", len(variants))
	}
	if variants[0] == nil {
		t.Fatal("variants[0] = nil, want product")
	}
	if variants[0].Asin != "B-requested" {
		t.Fatalf("variants[0].Asin = %q, want requested ASIN preserved", variants[0].Asin)
	}
	if len(rawClient.created) != 1 || rawClient.created[0] != "B-requested" {
		t.Fatalf("created product IDs = %v, want [B-requested]", rawClient.created)
	}
}
