package sourcea1688

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authz"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	a1688 "task-processor/internal/product/sourcehandoff/a1688"
	"task-processor/internal/product/sourcing"
)

func TestCreateListingKitTaskReturnsCreatedTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeTaskCommandService{}
	router := gin.New()
	router.POST("/tasks", NewHandler(service).CreateListingKitTask)

	body := CreateListingKitTaskRequest{
		URL:           "https://detail.1688.com/offer/999.html?spm=http",
		Product:       httpProduct1688("999"),
		SourceStoreID: 3001,
		Platforms:     []string{" SHEIN ", "shein"},
		Country:       "US",
		Language:      "en_US",
		SheinStoreID:  168811,
	}
	rec := performJSONRequest(t, router, http.MethodPost, "/tasks", body, map[string]string{"X-Tenant-ID": " tenant-http ", "X-User-ID": " user-http "})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.command.TenantID != "tenant-http" || service.command.UserID != "user-http" {
		t.Fatalf("command tenant/user = %q/%q, want header fallback", service.command.TenantID, service.command.UserID)
	}
	if service.command.SourceStoreID != 3001 || service.command.SheinStoreID != 168811 {
		t.Fatalf("store ids = (%d, %d), want source and shein stores preserved", service.command.SourceStoreID, service.command.SheinStoreID)
	}
	var got CreateListingKitTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.TaskID != "task-http" || got.TenantID != "tenant-http" || got.Status != listingkit.TaskStatusPending {
		t.Fatalf("response task = (%q, %q, %q), want created task", got.TaskID, got.TenantID, got.Status)
	}
	if got.SourceIdentity.SourceID != "999" || got.ProductURL != "https://detail.1688.com/offer/999.html" {
		t.Fatalf("response source = %+v product_url=%q, want normalized source", got.SourceIdentity, got.ProductURL)
	}
}

func TestCreateListingKitTaskReturnsBadRequestWithHandoff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeTaskCommandService{err: errors.New("1688 source cannot create listingkit task: crawler failed")}
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request: sourcing.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/1000.html"},
		Error:   errors.New("crawler failed"),
	})
	service.result = &a1688.CreateTaskResult{Handoff: &a1688.ListingKitTaskHandoff{Envelope: envelope}}
	router := gin.New()
	router.POST("/tasks", NewHandler(service).CreateListingKitTask)

	rec := performJSONRequest(t, router, http.MethodPost, "/tasks", CreateListingKitTaskRequest{URL: "https://detail.1688.com/offer/1000.html", SourceError: "crawler failed"}, map[string]string{"X-Tenant-ID": "tenant-http", "X-User-ID": "user-http"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	identity, ok := got["source_identity"].(map[string]any)
	if !ok || identity["SourceID"] != "1000" {
		t.Fatalf("source_identity = %#v, want source id 1000", got["source_identity"])
	}
	warnings, ok := got["source_warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("source_warnings = %#v, want warnings", got["source_warnings"])
	}
}

func TestCreateListingKitTaskRejectsInvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/tasks", NewHandler(&fakeTaskCommandService{}).CreateListingKitTask)

	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCreateListingKitTaskRequiresVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeTaskCommandService{}
	router := gin.New()
	router.POST("/tasks", NewHandler(service).CreateListingKitTask)

	rec := performJSONRequest(t, router, http.MethodPost, "/tasks", CreateListingKitTaskRequest{URL: "https://detail.1688.com/offer/1001.html"}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.command.URL != "" {
		t.Fatalf("command = %+v, want no service call", service.command)
	}
}

func TestCreateListingKitTaskIgnoresForgedBodyIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeTaskCommandService{}
	router := gin.New()
	router.POST("/tasks", NewHandler(service).CreateListingKitTask)
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"url":"https://detail.1688.com/offer/1002.html","tenant_id":"attacker","user_id":"attacker"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "verified-tenant")
	req.Header.Set("X-User-ID", "verified-user")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || service.command.TenantID != "verified-tenant" || service.command.UserID != "verified-user" {
		t.Fatalf("status=%d command=%+v, want verified identity", rec.Code, service.command)
	}
}

func TestAppendRouteDescriptorsIncludesCreateRoute(t *testing.T) {
	routes := AppendRouteDescriptors(nil, NewHandler(&fakeTaskCommandService{}))
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1", len(routes))
	}
	if routes[0].Method != http.MethodPost || routes[0].Path != "/api/v1/product-sourcing/1688/listingkit/tasks" || routes[0].Module != ModuleName || routes[0].Permission != authz.PermissionProductSourcingWrite || routes[0].Handler == nil {
		t.Fatalf("route = %+v, want 1688 listingkit task route", routes[0])
	}
}

func performJSONRequest(t *testing.T, router http.Handler, method string, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type fakeTaskCommandService struct {
	command a1688.CreateTaskCommand
	result  *a1688.CreateTaskResult
	err     error
}

func (f *fakeTaskCommandService) CreateTask(_ context.Context, command a1688.CreateTaskCommand) (*a1688.CreateTaskResult, error) {
	f.command = command
	if f.err != nil {
		return f.result, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request: sourcing.Alibaba1688CrawlRequestInput{URL: command.URL, StoreID: command.SourceStoreID},
		Product: command.Product,
	})
	return &a1688.CreateTaskResult{
		Task: &listingkit.Task{ID: "task-http", TenantID: command.TenantID, Status: listingkit.TaskStatusPending},
		Handoff: &a1688.ListingKitTaskHandoff{
			Envelope: envelope,
			Request:  listingkit.GenerateRequest{ProductURL: sourcing.NormalizeAlibaba1688URL(command.URL)},
		},
	}, nil
}

func httpProduct1688(id string) *alibaba1688model.Product1688 {
	return &alibaba1688model.Product1688{
		ID:       id,
		Title:    "Insulated Lunch Bag",
		URL:      "https://detail.1688.com/offer/" + id + ".html?foo=bar",
		Images:   []string{"https://img.example/" + id + "-main.jpg"},
		MinPrice: 18.8,
		Currency: "CNY",
		Category: "Bags>Lunch Bags",
		Brand:    "Factory Lunch",
	}
}
