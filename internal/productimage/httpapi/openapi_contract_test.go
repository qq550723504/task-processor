package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

func TestAssetOpenAPIContractLoads(t *testing.T) {
	doc := loadAssetOpenAPI(t)

	for _, path := range []string{
		"/api/v1/images/process",
		"/api/v1/images/tasks/{task_id}",
		"/api/v1/listing-kits/tasks/{task_id}/preview",
	} {
		if doc.Paths.Find(path) == nil {
			t.Fatalf("contract missing path %q", path)
		}
	}
}

func TestAssetOpenAPIContractValidatesExplicitTargetLifecycle(t *testing.T) {
	doc := loadAssetOpenAPI(t)
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}

	validProcess := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewBufferString(`{"image_urls":["https://example.test/source.jpg"],"target_platform":"shein"}`))
	validProcess.Header.Set("Content-Type", "application/json")
	validateOpenAPIRequest(t, router, validProcess, false)

	missingTarget := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewBufferString(`{"image_urls":["https://example.test/source.jpg"]}`))
	missingTarget.Header.Set("Content-Type", "application/json")
	validateOpenAPIRequest(t, router, missingTarget, true)

	validateOpenAPIResponse(t, router, validProcess, http.StatusBadRequest, `{"error":"invalid_request","message":"target_platform is required"}`)

	asyncResult := httptest.NewRequest(http.MethodGet, "/api/v1/images/tasks/image-task-1", nil)
	validateOpenAPIResponse(t, router, asyncResult, http.StatusOK, `{"task_id":"image-task-1","status":"completed","target_platform":"shein","created_at":"2026-08-12T09:00:00Z","result":{"main_image":{"url":"https://example.test/main.jpg","type":"main_image"}}}`)
}

func loadAssetOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	contractPath := filepath.Join("..", "..", "..", "docs", "api", "listingkit-asset.openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(contractPath)
	if err != nil {
		t.Fatalf("load asset OpenAPI contract: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("validate asset OpenAPI contract: %v", err)
	}
	return doc
}

func validateOpenAPIRequest(t *testing.T, router routers.Router, request *http.Request, wantError bool) {
	t.Helper()
	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find request route: %v", err)
	}
	err = openapi3filter.ValidateRequest(context.Background(), &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
	})
	if wantError && err == nil {
		t.Fatal("request validation succeeded, want missing target_platform error")
	}
	if !wantError && err != nil {
		t.Fatalf("request validation: %v", err)
	}
}

func validateOpenAPIResponse(t *testing.T, router routers.Router, request *http.Request, status int, body string) {
	t.Helper()
	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find response route: %v", err)
	}
	if err := openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: request, PathParams: pathParams, Route: route},
		Status:                 status,
		Header:                 http.Header{"Content-Type": []string{"application/json"}},
		Body:                   io.NopCloser(bytes.NewBufferString(body)),
	}); err != nil {
		t.Fatalf("response validation: %v", err)
	}
}
