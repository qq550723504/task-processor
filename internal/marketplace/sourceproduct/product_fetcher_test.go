package sourceproduct

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
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

type stubProductFetcherSource struct {
	configured  bool
	calls       int
	lastContext context.Context
	lastRequest SourceFetchRequest
	product     *model.Product
	err         error
}

func (s *stubProductFetcherSource) Configured() bool {
	if s == nil {
		panic("Configured called on typed-nil source fetcher")
	}
	return s.configured
}

func (s *stubProductFetcherSource) Fetch(ctx context.Context, req SourceFetchRequest) (*model.Product, error) {
	if s == nil {
		panic("Fetch called on typed-nil source fetcher")
	}
	s.lastContext = ctx
	s.lastRequest = req
	s.calls++
	return s.product, s.err
}

type selectiveProductFetcherSource struct {
	products map[string]*model.Product
}

func (*selectiveProductFetcherSource) Configured() bool { return true }

func TestProductFetcherUsesDiscardLoggerWhenNoneIsInjected(t *testing.T) {
	fetcher := NewProductFetcher(nil, ProductFetcherOptions{}, nil)
	if fetcher.logger == nil || fetcher.logger.Logger.Out != io.Discard {
		t.Fatalf("default logger output = %v, want io.Discard", fetcher.logger)
	}
}

func TestProductFetcherKeepsExplicitLoggerInjection(t *testing.T) {
	var output bytes.Buffer
	log := logrus.New()
	log.SetOutput(&output)
	fetcher := NewProductFetcherWithLogger(nil, ProductFetcherOptions{}, nil, logrus.NewEntry(log))

	if err := fetcher.CacheProduct(nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "product is nil, skipping cache") {
		t.Fatalf("injected logger output = %q, want product warning", output.String())
	}
}

func (s *selectiveProductFetcherSource) Fetch(_ context.Context, req SourceFetchRequest) (*model.Product, error) {
	if product, ok := s.products[req.ProductID]; ok {
		return product, nil
	}
	return nil, errors.New("product not found")
}

func TestProductDomainResolverCompatibilityLayerIsRetired(t *testing.T) {
	if _, err := os.Stat("domain_resolver.go"); err == nil {
		t.Fatal("domain_resolver.go still exists; Amazon source URL and zipcode rules belong in internal/integration/crawler/amazon")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat domain_resolver.go: %v", err)
	}
}

func TestProductRepositoryServiceCompatibilityLayerIsRetired(t *testing.T) {
	for _, file := range []string{"repository.go", "service.go"} {
		if _, err := os.Stat(file); err == nil {
			t.Fatalf("%s still exists; use ProductFetcher and the Amazon adapter instead of the unwired repository-style product crawler service", file)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", file, err)
		}
	}
}

func TestProductFetcherMapsNeutralSourceRequestAndPreservesContext(t *testing.T) {
	source := &stubProductFetcherSource{configured: true, product: &model.Product{Asin: "B001"}}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, source)
	ctx := context.WithValue(context.Background(), struct{}{}, "sentinel")

	product, err := fetcher.FetchProduct(ctx, &FetchRequest{
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
	if source.lastContext != ctx {
		t.Fatal("SourceFetcher received a different context")
	}
	if source.lastRequest != (SourceFetchRequest{Region: "uk", ProductID: "B001", Zipcode: "EC1A 1BB"}) {
		t.Fatalf("source request = %+v, want complete neutral request", source.lastRequest)
	}
}

func TestProductFetcherFetchVariantsPreservesExplicitZipcode(t *testing.T) {
	source := &stubProductFetcherSource{configured: true, product: &model.Product{Asin: "B-variant"}}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, source)

	_, err := fetcher.FetchVariants(context.Background(), &FetchRequest{
		Region:    "uk",
		ProductID: "B-parent",
		Zipcode:   "EC1A 1BB",
	}, []string{"B-variant"})
	if err != nil {
		t.Fatalf("FetchVariants() error = %v", err)
	}
	if source.lastRequest.Zipcode != "EC1A 1BB" {
		t.Fatalf("variant zipcode = %q, want inherited explicit zipcode", source.lastRequest.Zipcode)
	}
}

func TestProductFetcherReturnsErrorWhenCrawlerUnavailableAfterCacheMiss(t *testing.T) {
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, nil)

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

func TestProductFetcherTreatsTypedNilSourceFetcherAsUnavailable(t *testing.T) {
	var source *stubProductFetcherSource
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, source)

	product, err := fetcher.FetchProduct(context.Background(), &FetchRequest{Region: "us", ProductID: "B003"})
	if err == nil {
		t.Fatal("FetchProduct() error = nil, want crawler unavailable error")
	}
	if product != nil {
		t.Fatalf("FetchProduct() product = %+v, want nil", product)
	}
}

func TestProductFetcherReturnsContextErrorAfterSourceCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wantSourceErr := errors.New("source failed")
	source := &cancelingProductFetcherSource{cancel: cancel, err: wantSourceErr}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, source)

	_, err := fetcher.FetchProduct(ctx, &FetchRequest{Region: "us", ProductID: "B004", Zipcode: "10001"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchProduct() error = %v, want context cancellation over source error", err)
	}
}

func TestProductFetcherDoesNotDispatchCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := &stubProductFetcherSource{configured: true, product: &model.Product{Asin: "unused"}}
	fetcher := NewProductFetcher(stubProductFetcherRawJSONClient{}, ProductFetcherOptions{Enabled: true}, source)

	_, err := fetcher.FetchProduct(ctx, &FetchRequest{Region: "us", ProductID: "B005", Zipcode: "10001"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchProduct() error = %v, want context cancellation", err)
	}
	if source.calls != 0 {
		t.Fatalf("SourceFetcher calls = %d, want 0 for already canceled context", source.calls)
	}
}

type cancelingProductFetcherSource struct {
	cancel context.CancelFunc
	err    error
}

func (*cancelingProductFetcherSource) Configured() bool { return true }

func (s *cancelingProductFetcherSource) Fetch(context.Context, SourceFetchRequest) (*model.Product, error) {
	s.cancel()
	return nil, s.err
}

func TestProductFetcherFetchVariantsCachesEachSuccessfulVariantImmediately(t *testing.T) {
	rawClient := &recordingProductFetcherRawJSONClient{}
	source := &selectiveProductFetcherSource{
		products: map[string]*model.Product{
			"B-success-1": {Asin: "B-success-1", ShipsFrom: "Amazon.com"},
			"B-success-2": {Asin: "B-success-2", ShipsFrom: "Amazon.com"},
		},
	}
	fetcher := NewProductFetcher(rawClient, ProductFetcherOptions{Enabled: true}, source)

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
	source := &selectiveProductFetcherSource{
		products: map[string]*model.Product{
			"B-requested": {
				Asin:       "B-redirected",
				ParentAsin: "PARENT-1",
				ShipsFrom:  "Amazon.com",
			},
		},
	}
	fetcher := NewProductFetcher(rawClient, ProductFetcherOptions{Enabled: true}, source)

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
