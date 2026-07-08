package productdata

import (
	"context"
	"errors"
	"strings"
	"testing"

	appProduct "task-processor/internal/crawler/fetcher"
	"task-processor/internal/model"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"
)

type stubProductFetcher struct {
	err error
}

var _ appProduct.ProductFetcher = (*stubProductFetcher)(nil)

func (s *stubProductFetcher) FetchProduct(context.Context, *product.FetchRequest) (*model.Product, error) {
	return nil, s.err
}

func (s *stubProductFetcher) FetchVariants(context.Context, *product.FetchRequest, []string) ([]*model.Product, error) {
	return nil, s.err
}

func (s *stubProductFetcher) CacheProduct(*product.FetchRequest, *model.Product) error {
	return nil
}

func (s *stubProductFetcher) CacheVariants(*product.FetchRequest, []*model.Product) error {
	return nil
}

func (s *stubProductFetcher) GetStats() map[string]any {
	return nil
}

func TestFetchAndCacheProductHandlerTreatsPlaywrightDriver404AsRetryable(t *testing.T) {
	driverErr := errors.New("初始化浏览器池失败: 初始化playwright失败: 自动安装 Playwright 驱动失败: error: got non 200 status code: 404 (404 Not Found) from https://playwright.azureedge.net/builds/driver/playwright-1.57.0-linux.zip")

	handler := NewFetchAndCacheProductHandler(&stubProductFetcher{err: driverErr})
	err := handler.Handle(shein.NewTaskContext(context.Background(), &model.Task{
		TenantID:  1,
		Platform:  "shein",
		Region:    "us",
		ProductID: "B001",
	}))

	if err == nil {
		t.Fatal("Handle() error = nil, want retryable infrastructure error")
	}
	if !shein.IsRetryableError(err) {
		t.Fatalf("Handle() retryable = false, want true; err=%v", err)
	}
	if strings.Contains(err.Error(), "Amazon product not found") {
		t.Fatalf("Handle() err = %v, want infrastructure fetch failure instead of product-not-found", err)
	}
}
