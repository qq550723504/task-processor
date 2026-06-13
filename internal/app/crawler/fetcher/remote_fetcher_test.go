package fetcher

import (
	"context"
	"encoding/json"
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

func TestRemoteAPIProductFetcherDoesNotSendDefaultUSZipcode(t *testing.T) {
	var payload remoteFetchRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
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

	_, err = fetcher.FetchProduct(context.Background(), &domainProduct.FetchRequest{
		Platform:  "shein",
		Region:    "us",
		ProductID: "B001",
	})
	if err != nil {
		t.Fatalf("fetch product: %v", err)
	}
	if payload.Zipcode != "" {
		t.Fatalf("expected no default us zipcode in payload, got %q", payload.Zipcode)
	}
}

func TestRemoteAPIProductFetcherPreservesExplicitZipcode(t *testing.T) {
	var payload remoteFetchRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
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

	_, err = fetcher.FetchProduct(context.Background(), &domainProduct.FetchRequest{
		Platform:  "shein",
		Region:    "us",
		ProductID: "B001",
		Zipcode:   "10001",
	})
	if err != nil {
		t.Fatalf("fetch product: %v", err)
	}
	if payload.Zipcode != "10001" {
		t.Fatalf("expected explicit zipcode to be preserved, got %q", payload.Zipcode)
	}
}

func TestRemoteAPIProductFetcherUsesConfiguredDefaultZipcode(t *testing.T) {
	var payload remoteFetchRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"url":"https://www.amazon.co.uk/dp/B001","product":{"asin":"B001","title":"Demo"}}}`))
	}))
	defer server.Close()

	fetcher, err := NewRemoteAPIProductFetcher(nil, &config.AmazonConfig{
		Enabled: true,
		Zipcodes: map[string]string{
			"uk": "W1A 1AA",
		},
		RemoteAPI: config.RemoteAPIConfig{
			Enabled: true,
			BaseURL: server.URL,
			Timeout: 10,
		},
	})
	if err != nil {
		t.Fatalf("create fetcher: %v", err)
	}

	_, err = fetcher.FetchProduct(context.Background(), &domainProduct.FetchRequest{
		Platform:  "shein",
		Region:    "UK",
		ProductID: "B001",
	})
	if err != nil {
		t.Fatalf("fetch product: %v", err)
	}
	if payload.Zipcode != "W1A 1AA" {
		t.Fatalf("expected configured zipcode, got %q", payload.Zipcode)
	}
}

func TestRemoteAPIProductFetcherFetchVariantsPreservesExplicitZipcode(t *testing.T) {
	var payloads []remoteFetchRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload remoteFetchRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		payloads = append(payloads, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"url":"https://www.amazon.com/dp/BVAR","product":{"asin":"BVAR","title":"Variant"}}}`))
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

	_, err = fetcher.FetchVariants(context.Background(), &domainProduct.FetchRequest{
		Platform:  "shein",
		Region:    "us",
		ProductID: "B-parent",
		Zipcode:   "10001",
	}, []string{"B-variant"})
	if err != nil {
		t.Fatalf("fetch variants: %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(payloads))
	}
	if payloads[0].Zipcode != "10001" {
		t.Fatalf("variant zipcode = %q, want inherited explicit zipcode", payloads[0].Zipcode)
	}
	if payloads[0].ASIN != "B-variant" {
		t.Fatalf("variant ASIN = %q, want B-variant", payloads[0].ASIN)
	}
}
