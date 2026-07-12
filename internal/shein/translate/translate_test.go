package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestHandleLoadsProductNameLengthConfigOnce(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body struct {
			CategoryID int `json:"category_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.CategoryID != 1772 {
			t.Errorf("category_id = %d, want 1772", body.CategoryID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":[{"language":"en","max_length":12}]}`))
	}))
	defer server.Close()

	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "XX"})
	ctx.AmazonProduct = &model.Product{Title: "short title", Description: "description"}
	ctx.ProductData = &sheinproduct.Product{CategoryID: 1772}
	ctx.ProductAPI = sheinproduct.NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	handler := NewTranslateHandler(nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("second Handle() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("config requests = %d, want 1", requests)
	}
	if maxLength, ok := ctx.ProductNameLengthLimits.Max("en"); !ok || maxLength != 12 {
		t.Fatalf("english max = %d, %v; want 12, true", maxLength, ok)
	}
}

func TestHandleFallsBackAfterProductNameLengthConfigFailure(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"1","msg":"unavailable"}`))
	}))
	defer server.Close()

	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "XX"})
	ctx.AmazonProduct = &model.Product{Title: "short title", Description: "description"}
	ctx.ProductData = &sheinproduct.Product{CategoryID: 1772}
	ctx.ProductAPI = sheinproduct.NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	handler := NewTranslateHandler(nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("second Handle() error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("config requests = %d, want 1", requests)
	}
	if ctx.ProductNameLengthLimits == nil {
		t.Fatal("fallback limits are nil, want initialized empty limits")
	}
}
