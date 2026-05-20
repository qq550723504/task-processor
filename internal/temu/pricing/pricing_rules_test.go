package pricing

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/core/logger"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/model"
	"task-processor/internal/product"
)

type cancelingFetcher struct {
	calls  int
	cancel context.CancelFunc
}

func (f *cancelingFetcher) FetchProduct(ctx context.Context, req *product.FetchRequest) (*model.Product, error) {
	f.calls++
	if f.calls == 1 && f.cancel != nil {
		f.cancel()
	}
	return nil, errors.New("fetch failed")
}

func (f *cancelingFetcher) FetchVariants(ctx context.Context, req *product.FetchRequest, variantASINs []string) ([]*model.Product, error) {
	return nil, nil
}

func (f *cancelingFetcher) CacheProduct(req *product.FetchRequest, product *model.Product) error {
	return nil
}

func (f *cancelingFetcher) CacheVariants(req *product.FetchRequest, variants []*model.Product) error {
	return nil
}

func (f *cancelingFetcher) GetStats() map[string]any {
	return nil
}

var _ appfetcher.ProductFetcher = (*cancelingFetcher)(nil)

func TestGetAmazonProductWithCacheStopsRetrySleepWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fetcher := &cancelingFetcher{cancel: cancel}
	service := &PricingDecisionService{
		config: &ServiceConfig{
			MaxRetries:   3,
			CacheTimeout: time.Minute,
		},
		productFetcher: fetcher,
		logger:         logger.GetGlobalLogger("pricing_rules_test"),
	}

	start := time.Now()
	_, err := service.getAmazonProductWithCache(ctx, "B001", "us", 1, 1)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected one fetch attempt before cancellation, got %d", fetcher.calls)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected cancellation to stop retry wait quickly, elapsed=%v", elapsed)
	}
}
