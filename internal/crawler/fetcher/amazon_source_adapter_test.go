package fetcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/marketplace/sourceproduct"
	"task-processor/internal/model"
)

type recordingAmazonCrawlSource struct {
	calls       int
	lastContext context.Context
	lastURL     string
	lastZipcode string
	product     *model.Product
	err         error
	cancel      context.CancelFunc
}

func (s *recordingAmazonCrawlSource) Process(url, zipcode string) (*model.Product, error) {
	return s.ProcessWithContext(context.Background(), url, zipcode)
}

func (s *recordingAmazonCrawlSource) ProcessWithContext(ctx context.Context, url, zipcode string) (*model.Product, error) {
	s.calls++
	s.lastContext = ctx
	s.lastURL = url
	s.lastZipcode = zipcode
	if s.cancel != nil {
		s.cancel()
	}
	return s.product, s.err
}

func (*recordingAmazonCrawlSource) Shutdown() {}

type amazonAdapterCacheClient struct {
	raw *sourceproduct.RawJsonResp
}

func (c amazonAdapterCacheClient) GetRawJsonData(*sourceproduct.RawJsonReq) (*sourceproduct.RawJsonResp, error) {
	if c.raw == nil {
		return nil, errors.New("cache miss")
	}
	return c.raw, nil
}

func (amazonAdapterCacheClient) CreateRawJsonData(*sourceproduct.RawJsonCreateReq) (int64, error) {
	return 0, nil
}

func TestAmazonSourceFetcherAdapterMapsRequestAndPreservesContext(t *testing.T) {
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "B001"}}
	adapter := newAmazonSourceFetcher(source, map[string]string{"uk": "W1A 1AA"})
	ctx := context.WithValue(context.Background(), struct{}{}, "sentinel")

	got, err := adapter.Fetch(ctx, sourceproduct.SourceFetchRequest{
		Region:    " UK ",
		ProductID: " B001 ",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got == nil || got.Asin != "B001" {
		t.Fatalf("Fetch() = %+v, want crawler product", got)
	}
	if source.lastContext != ctx {
		t.Fatal("crawl source received a different context")
	}
	if source.lastURL != "https://www.amazon.co.uk/dp/B001?th=1&psc=1&language=en_GB" {
		t.Fatalf("crawl source URL = %q, want normalized UK Amazon URL", source.lastURL)
	}
	if source.lastZipcode != "W1A 1AA" {
		t.Fatalf("crawl source zipcode = %q, want configured UK zipcode", source.lastZipcode)
	}
}

func TestAmazonSourceFetcherAdapterPreservesExplicitZipcode(t *testing.T) {
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "B002"}}
	adapter := newAmazonSourceFetcher(source, map[string]string{"uk": "W1A 1AA"})

	_, err := adapter.Fetch(context.Background(), sourceproduct.SourceFetchRequest{
		Region:    "uk",
		ProductID: "B002",
		Zipcode:   " EC1A 1BB ",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if source.lastZipcode != "EC1A 1BB" {
		t.Fatalf("crawl source zipcode = %q, want explicit zipcode", source.lastZipcode)
	}
}

func TestAmazonSourceFetcherAdapterDefensivelyCopiesConfiguredZipcodes(t *testing.T) {
	zipcodes := map[string]string{"uk": "W1A 1AA"}
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "B003"}}
	adapter := newAmazonSourceFetcher(source, zipcodes)
	zipcodes["uk"] = "MUTATED"

	_, err := adapter.Fetch(context.Background(), sourceproduct.SourceFetchRequest{
		Region:    "uk",
		ProductID: "B003",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if source.lastZipcode != "W1A 1AA" {
		t.Fatalf("crawl source zipcode = %q, want construction snapshot", source.lastZipcode)
	}
}

func TestAmazonSourceFetcherAdapterTreatsTypedNilCrawlSourceAsUnconfigured(t *testing.T) {
	var source *recordingAmazonCrawlSource
	adapter := newAmazonSourceFetcher(source, nil)

	if adapter.Configured() {
		t.Fatal("Configured() = true for typed-nil crawl source")
	}
	product, err := adapter.Fetch(context.Background(), sourceproduct.SourceFetchRequest{Region: "us", ProductID: "B004"})
	if err == nil {
		t.Fatal("Fetch() error = nil, want unconfigured error")
	}
	if product != nil {
		t.Fatalf("Fetch() = %+v, want nil product", product)
	}
}

func TestAmazonSourceFetcherAdapterPrioritizesPostCallContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sourceErr := errors.New("crawl source failed")
	source := &recordingAmazonCrawlSource{cancel: cancel, err: sourceErr}
	adapter := newAmazonSourceFetcher(source, nil)

	_, err := adapter.Fetch(ctx, sourceproduct.SourceFetchRequest{Region: "us", ProductID: "B005"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context cancellation over crawl source error", err)
	}
}

func TestAmazonSourceFetcherAdapterDoesNotDispatchCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "unused"}}
	adapter := newAmazonSourceFetcher(source, nil)

	_, err := adapter.Fetch(ctx, sourceproduct.SourceFetchRequest{Region: "us", ProductID: "B006"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context cancellation", err)
	}
	if source.calls != 0 {
		t.Fatalf("crawl source calls = %d, want 0 for already canceled context", source.calls)
	}
}

func TestFetcherFactoryMapsAmazonConfigZipcodesIntoLocalAdapter(t *testing.T) {
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "B006"}}
	fetcher, err := NewFetcherFactory().CreateFetcher(
		LocalFetcher,
		amazonAdapterCacheClient{},
		&config.AmazonConfig{Enabled: true, Zipcodes: map[string]string{"uk": "W1A 1AA"}},
		source,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateFetcher() error = %v", err)
	}

	_, err = fetcher.FetchProduct(context.Background(), &sourceproduct.FetchRequest{Region: "uk", ProductID: "B006"})
	if err != nil {
		t.Fatalf("FetchProduct() error = %v", err)
	}
	if source.lastZipcode != "W1A 1AA" {
		t.Fatalf("crawl source zipcode = %q, want configured UK zipcode", source.lastZipcode)
	}
}

func TestFetcherFactoryMapsAmazonFreshnessIntoLocalCachePolicy(t *testing.T) {
	twoDaysAgo := time.Now().Add(-48 * time.Hour).UnixMilli()
	cacheClient := amazonAdapterCacheClient{raw: &sourceproduct.RawJsonResp{
		RawJSONData: `{"asin":"B-cache","ships_from":""}`,
		CreateTime:  twoDaysAgo,
		UpdateTime:  twoDaysAgo,
	}}
	source := &recordingAmazonCrawlSource{product: &model.Product{Asin: "B-fresh"}}
	fetcher, err := NewFetcherFactory().CreateFetcher(
		LocalFetcher,
		cacheClient,
		&config.AmazonConfig{Enabled: true, DataFreshnessDays: 1},
		source,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateFetcher() error = %v", err)
	}

	product, err := fetcher.FetchProduct(context.Background(), &sourceproduct.FetchRequest{Region: "us", ProductID: "B007"})
	if err != nil {
		t.Fatalf("FetchProduct() error = %v", err)
	}
	if product == nil || product.Asin != "B-fresh" {
		t.Fatalf("FetchProduct() = %+v, want source refresh under one-day freshness policy", product)
	}
}
