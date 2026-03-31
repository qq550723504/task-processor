package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"task-processor/internal/core/config"
	domainProduct "task-processor/internal/product"
)

func TestRemoteAPIProductFetcherFetchProduct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products/fetch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"url":"https://www.amazon.com/dp/B001","product":{"asin":"B001","title":"Demo"}}}`))
	}))
	defer server.Close()

	fetcher, err := NewRemoteAPIProductFetcher(nil, &config.AmazonConfig{
		Enabled: true,
		RemoteAPI: config.RemoteAPIConfig{
			Enabled: true,
			BaseURL: server.URL,
			Timeout: 10,
		},
	})
	if err != nil {
		t.Fatalf("create fetcher: %v", err)
	}

	product, err := fetcher.FetchProduct(context.Background(), &domainProduct.FetchRequest{
		Platform:  "temu",
		Region:    "us",
		ProductID: "B001",
	})
	if err != nil {
		t.Fatalf("fetch product: %v", err)
	}
	if product == nil || product.Asin != "B001" {
		t.Fatalf("unexpected product: %+v", product)
	}
}

func TestRemoteAPIProductFetcherFallsBackToAsyncPollingWhenBusy(t *testing.T) {
	var taskPolls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/products/fetch":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"success":false,"error":"system busy","code":"system_busy","data":{"retryable":true}}`))
		case r.URL.Path == "/api/v1/crawl":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":"task-123","url":"https://www.amazon.com/dp/B001"}}`))
		case r.URL.Path == "/api/v1/tasks/task-123":
			w.Header().Set("Content-Type", "application/json")
			if taskPolls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"success":true,"data":{"TaskID":"task-123","Status":"processing"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"TaskID":"task-123","Status":"success","ProductData":{"asin":"B001","title":"Demo"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetcher, err := NewRemoteAPIProductFetcher(nil, &config.AmazonConfig{
		Enabled: true,
		RemoteAPI: config.RemoteAPIConfig{
			Enabled: true,
			BaseURL: server.URL,
			Timeout: 10,
		},
	})
	if err != nil {
		t.Fatalf("create fetcher: %v", err)
	}

	product, err := fetcher.FetchProduct(context.Background(), &domainProduct.FetchRequest{
		Platform:  "shein",
		Region:    "us",
		ProductID: "B001",
	})
	if err != nil {
		t.Fatalf("fetch product with async fallback: %v", err)
	}
	if product == nil || product.Asin != "B001" {
		t.Fatalf("unexpected product: %+v", product)
	}
	if taskPolls.Load() < 2 {
		t.Fatalf("expected async polling to happen at least twice, got %d", taskPolls.Load())
	}
}
