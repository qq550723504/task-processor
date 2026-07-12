package translate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	sheinclient "task-processor/internal/shein/client"

	"github.com/imroc/req/v3"
)

func TestHandleLoadsProductNameLengthConfigOnce(t *testing.T) {
	t.Parallel()

	lengthRequests := 0
	languageRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case sheinclient.GetProductNameLengthConfigEndpoint():
			lengthRequests++
			var body struct {
				CategoryID int `json:"category_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
			}
			if body.CategoryID != 1772 {
				t.Errorf("category_id = %d, want 1772", body.CategoryID)
			}
			_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":[{"language":"en","max_length":12}]}`))
		case sheinclient.GetLanguageListEndpoint():
			languageRequests++
			_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"language_abbr":"en","input_mode":1},{"language_abbr":"es","input_mode":0},{"language_abbr":"fr","input_mode":1}]}}`))
		case sheinclient.GetTranslateTextEndpoint():
			_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[{"translated_text":"titre","code":0}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "XX"})
	ctx.AmazonProduct = &model.Product{Title: "short title", Description: "description"}
	ctx.ProductData = &sheinproduct.Product{CategoryID: 1772}
	baseClient := sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C())
	ctx.ProductAPI = sheinproduct.NewClient(baseClient)
	ctx.TranslateAPI = sheintranslate.NewClient(baseClient)
	handler := NewTranslateHandler(nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("first Handle() error = %v", err)
	}
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("second Handle() error = %v", err)
	}
	if lengthRequests != 1 || languageRequests != 1 {
		t.Fatalf("requests = length %d, language %d; want 1 each", lengthRequests, languageRequests)
	}
	if maxLength, ok := ctx.ProductNameLengthLimits.Max("en"); !ok || maxLength != 12 {
		t.Fatalf("english max = %d, %v; want 12, true", maxLength, ok)
	}
	if got := ctx.TargetLanguages; !reflect.DeepEqual(got, []string{"en", "fr"}) {
		t.Fatalf("TargetLanguages = %#v, want [en fr]", got)
	}
	if got := ctx.ProductData.MultiLanguageNameList; len(got) != 2 || got[0].Language != "en" || got[1].Language != "fr" {
		t.Fatalf("product name languages = %#v, want en, fr", got)
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
	if requests != 2 {
		t.Fatalf("config requests = %d, want 2", requests)
	}
	if ctx.ProductNameLengthLimits == nil {
		t.Fatal("fallback limits are nil, want initialized empty limits")
	}
}

func TestLoadTargetLanguagesFallsBackOnEmptyResponse(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"0","msg":"OK","info":{"data":[]}}`))
	}))
	defer server.Close()

	ctx := shein.NewTaskContext(context.Background(), &model.Task{Region: "JP"})
	ctx.ProductAPI = sheinproduct.NewClient(sheinclient.NewBaseAPIClient(server.URL, 1, 2, req.C()))
	handler := NewTranslateHandler(nil)
	handler.loadTargetLanguages(ctx)
	handler.loadTargetLanguages(ctx)

	if !reflect.DeepEqual(ctx.TargetLanguages, []string{"ja", "en"}) {
		t.Fatalf("TargetLanguages = %#v, want [ja en]", ctx.TargetLanguages)
	}
	if requests != 1 {
		t.Fatalf("language requests = %d, want 1", requests)
	}
}
