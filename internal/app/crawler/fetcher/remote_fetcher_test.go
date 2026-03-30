package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
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
