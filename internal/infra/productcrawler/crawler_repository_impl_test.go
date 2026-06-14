package productcrawler

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"task-processor/internal/product"

	"github.com/sirupsen/logrus"
)

func TestFetchVariantsBatchAlignsShortSourceResults(t *testing.T) {
	source := &stubAmazonCrawlerSource{
		results: []model.ProductResult{
			{Product: &model.Product{Asin: "B001", Title: "first variant"}},
		},
	}
	repo := NewCrawlerRepositoryImpl(source, &config.AmazonConfig{Enabled: true}, stubDomainResolver{}, testLogger())

	products, errs := repo.FetchVariantsBatch(context.Background(), &product.FetchRequest{
		Platform: "amazon",
		Region:   " US ",
		Zipcode:  " 10001 ",
	}, []string{" B001 ", "B002"})

	if len(errs) != 0 {
		t.Fatalf("expected no crawler errors for missing trailing source result, got %d: %v", len(errs), errs)
	}
	if len(products) != 1 {
		t.Fatalf("expected one returned product, got %d", len(products))
	}
	if products[0].Asin != "B001" {
		t.Fatalf("expected first product ASIN B001, got %q", products[0].Asin)
	}

	if len(source.requests) != 2 {
		t.Fatalf("expected two planned batch requests, got %d", len(source.requests))
	}
	if got, want := source.requests[0].URL, "https://example.us/dp/B001"; got != want {
		t.Fatalf("request[0] URL = %q, want %q", got, want)
	}
	if got, want := source.requests[1].URL, "https://example.us/dp/B002"; got != want {
		t.Fatalf("request[1] URL = %q, want %q", got, want)
	}
	if got, want := source.requests[0].Zipcode, "10001"; got != want {
		t.Fatalf("request[0] zipcode = %q, want %q", got, want)
	}
}

type stubAmazonCrawlerSource struct {
	requests []model.ProductRequest
	results  []model.ProductResult
}

func (s *stubAmazonCrawlerSource) ProcessWithContext(context.Context, string, string) (*model.Product, error) {
	return nil, fmt.Errorf("unexpected single product fetch")
}

func (s *stubAmazonCrawlerSource) ProcessBatchWithContext(_ context.Context, requests []model.ProductRequest) []model.ProductResult {
	s.requests = append([]model.ProductRequest(nil), requests...)
	return s.results
}

type stubDomainResolver struct{}

func (stubDomainResolver) GetAmazonDomainByRegion(region string) string {
	region = strings.ToLower(strings.TrimSpace(region))
	if region == "" {
		return ""
	}
	return "example." + region
}

func (stubDomainResolver) BuildAmazonProductURL(region, asin string) string {
	return "https://" + stubDomainResolver{}.GetAmazonDomainByRegion(region) + "/dp/" + strings.TrimSpace(asin)
}

func testLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logrus.NewEntry(logger)
}
