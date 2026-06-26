package amazon

import (
	"context"
	"errors"
	"testing"

	amazonbrowser "task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/model"
)

func TestAmazonProcessorProcessWithContextReturnsInitError(t *testing.T) {
	ap := &AmazonProcessor{
		initErr: errors.New("初始化浏览器池失败: playwright missing"),
	}

	product, err := ap.ProcessWithContext(context.Background(), "https://www.amazon.com/dp/B001234567", "")
	if err == nil {
		t.Fatal("expected init error when browser pool is unavailable")
	}
	if err.Error() != "初始化浏览器池失败: playwright missing" {
		t.Fatalf("unexpected error: %v", err)
	}
	if product != nil {
		t.Fatalf("expected nil product, got %+v", product)
	}
}

func TestAmazonProcessorProcessBatchReturnsInitErrorForEveryRequest(t *testing.T) {
	ap := &AmazonProcessor{
		initErr: errors.New("初始化浏览器池失败: chromium launch failed"),
	}

	results := ap.ProcessBatch([]model.ProductRequest{
		{URL: "https://www.amazon.com/dp/B001234567"},
		{URL: "https://www.amazon.com/dp/B009876543"},
	})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, result := range results {
		if result.Error == nil {
			t.Fatalf("result %d expected init error", i)
		}
		if result.Error.Error() != "初始化浏览器池失败: chromium launch failed" {
			t.Fatalf("result %d unexpected error: %v", i, result.Error)
		}
		if result.Product != nil {
			t.Fatalf("result %d expected nil product, got %+v", i, result.Product)
		}
	}
}

func TestAmazonProcessorProcessWithContextRequiresPoolManager(t *testing.T) {
	ap := &AmazonProcessor{}

	product, err := ap.ProcessWithContext(context.Background(), "https://www.amazon.com/dp/B001234567", "")
	if err == nil {
		t.Fatal("expected unavailable error when pool manager is missing")
	}
	if err.Error() != "Amazon处理器不可用: 浏览器池管理器未初始化" {
		t.Fatalf("unexpected error: %v", err)
	}
	if product != nil {
		t.Fatalf("expected nil product, got %+v", product)
	}
}

func TestAmazonProcessorPoolStatsIncludesInitError(t *testing.T) {
	ap := &AmazonProcessor{
		initErr: errors.New("初始化浏览器池失败: chromium launch failed"),
	}

	stats := ap.PoolStats()
	if stats == nil {
		t.Fatal("expected pool stats")
	}
	if stats["browser_pool_init_error"] != "初始化浏览器池失败: chromium launch failed" {
		t.Fatalf("unexpected browser_pool_init_error: %#v", stats["browser_pool_init_error"])
	}
}

func TestAmazonProcessorEnsureReadyRetriesInitError(t *testing.T) {
	ap := &AmazonProcessor{
		initErr: errors.New("初始化浏览器池失败: browser target closed"),
	}
	retried := false
	ap.retryInit = func(ctx context.Context) error {
		retried = true
		ap.mu.Lock()
		defer ap.mu.Unlock()
		ap.initErr = nil
		ap.browserPool = &amazonbrowser.BrowserPool{}
		ap.poolManager = &amazonbrowser.PoolManager{}
		return nil
	}

	if err := ap.ensureReady(context.Background()); err != nil {
		t.Fatalf("expected retry to recover processor, got %v", err)
	}
	if !retried {
		t.Fatal("expected init retry to run")
	}
	if ap.initErr != nil {
		t.Fatalf("expected init error to be cleared, got %v", ap.initErr)
	}
}
