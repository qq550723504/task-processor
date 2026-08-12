package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
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

func TestAssetOpenAPIContractValidatesRealGinHandlerResponses(t *testing.T) {
	doc := loadAssetOpenAPI(t)
	contractRouter, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}
	repo := productimagestore.NewMemTaskRepository()
	service, err := productimage.NewService(&productimage.ServiceConfig{TaskRepo: repo})
	if err != nil {
		t.Fatalf("new image service: %v", err)
	}
	ginRouter := newTestRouter(service)

	post := httptest.NewRequest(http.MethodPost, "/images/process", bytes.NewBufferString(`{"image_urls":["source.jpg"],"target_platform":"shein"}`))
	post.Header.Set("Content-Type", "application/json")
	postResponse := httptest.NewRecorder()
	ginRouter.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusOK {
		t.Fatalf("real process response = %d, body=%s", postResponse.Code, postResponse.Body.String())
	}
	validateOpenAPIResponse(t, contractRouter, withContractPath(post, "/api/v1/images/process"), postResponse.Code, postResponse.Body.String())

	missingTarget := httptest.NewRequest(http.MethodPost, "/images/process", bytes.NewBufferString(`{"image_urls":["source.jpg"]}`))
	missingTarget.Header.Set("Content-Type", "application/json")
	missingTargetResponse := httptest.NewRecorder()
	ginRouter.ServeHTTP(missingTargetResponse, missingTarget)
	if missingTargetResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing target response = %d, body=%s", missingTargetResponse.Code, missingTargetResponse.Body.String())
	}
	validateOpenAPIResponse(t, contractRouter, withContractPath(missingTarget, "/api/v1/images/process"), missingTargetResponse.Code, missingTargetResponse.Body.String())

	for _, status := range []productimage.TaskStatus{productimage.TaskStatusPending, productimage.TaskStatusProcessing, productimage.TaskStatusCompleted, productimage.TaskStatusNeedsReview, productimage.TaskStatusRejected, productimage.TaskStatusFailed} {
		taskID := "real-" + string(status)
		if err := repo.CreateTask(context.Background(), &productimage.Task{ID: taskID, Request: &productimage.ImageProcessRequest{TargetPlatform: "shein", ImageURLs: []string{"source.jpg"}}, Status: status, Result: &productimage.ImageProcessResult{MainImage: &productimage.ImageAsset{URL: "https://example.test/main.jpg", Type: productimage.AssetTypeMainImage}}, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
			t.Fatalf("create %s task: %v", status, err)
		}
		get := httptest.NewRequest(http.MethodGet, "/images/tasks/"+taskID, nil)
		getResponse := httptest.NewRecorder()
		ginRouter.ServeHTTP(getResponse, get)
		if getResponse.Code != http.StatusOK {
			t.Fatalf("real %s task response = %d, body=%s", status, getResponse.Code, getResponse.Body.String())
		}
		validateOpenAPIResponse(t, contractRouter, withContractPath(get, "/api/v1/images/tasks/"+taskID), getResponse.Code, getResponse.Body.String())
	}
}

func TestAssetOpenAPIContractRejectsRequestWithoutImageSource(t *testing.T) {
	doc := loadAssetOpenAPI(t)
	contractRouter, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/images/process", bytes.NewBufferString(`{"target_platform":"shein"}`))
	request.Header.Set("Content-Type", "application/json")
	validateOpenAPIRequest(t, contractRouter, request, true)
}

func TestRealGinHandlerRejectsUnresolvableLegacyTaskTarget(t *testing.T) {
	doc := loadAssetOpenAPI(t)
	contractRouter, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("create OpenAPI router: %v", err)
	}
	repo := productimagestore.NewMemTaskRepository()
	service, err := productimage.NewService(&productimage.ServiceConfig{TaskRepo: repo})
	if err != nil {
		t.Fatalf("new image service: %v", err)
	}
	ginRouter := newTestRouter(service)
	for _, status := range []productimage.TaskStatus{productimage.TaskStatusPending, productimage.TaskStatusCompleted, productimage.TaskStatusFailed} {
		for _, marketplace := range []string{"", "unknown"} {
			taskID := "legacy-" + string(status) + "-" + marketplace
			if marketplace == "" {
				taskID += "blank"
			}
			if err := repo.CreateTask(context.Background(), &productimage.Task{ID: taskID, Request: &productimage.ImageProcessRequest{Marketplace: marketplace, ImageURLs: []string{"source.jpg"}}, Status: status, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
				t.Fatalf("create legacy %s task: %v", taskID, err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/images/tasks/"+taskID, nil)
			ginRouter.ServeHTTP(response, request)
			if response.Code != http.StatusConflict {
				t.Fatalf("legacy %s target status = %d, body=%s", taskID, response.Code, response.Body.String())
			}
			validateOpenAPIResponse(t, contractRouter, withContractPath(request, "/api/v1/images/tasks/"+taskID), response.Code, response.Body.String())
		}
	}
}

func TestRealGinHandlerNormalizesSupportedLegacyTaskTarget(t *testing.T) {
	repo := productimagestore.NewMemTaskRepository()
	service, err := productimage.NewService(&productimage.ServiceConfig{TaskRepo: repo})
	if err != nil {
		t.Fatalf("new image service: %v", err)
	}
	if err := repo.CreateTask(context.Background(), &productimage.Task{ID: "legacy-shein", Request: &productimage.ImageProcessRequest{Marketplace: " SHEIN ", ImageURLs: []string{"source.jpg"}}, Status: productimage.TaskStatusCompleted, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("create legacy task: %v", err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/images/tasks/legacy-shein", nil)
	newTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("legacy supported target status = %d, body=%s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"target_platform":"shein"`)) {
		t.Fatalf("legacy target response = %s", response.Body.String())
	}
}

func withContractPath(request *http.Request, contractPath string) *http.Request {
	clone := request.Clone(request.Context())
	clone.URL.Path = contractPath
	return clone
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
