package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubSDSRetirementHandlerService struct {
	createCtx   context.Context
	createReq   *listingkit.CreateSDSRetirementRunRequest
	getCtx      context.Context
	getRunID    string
	updateCtx   context.Context
	updateRunID string
	updateReq   *listingkit.UpdateSDSRetirementSelectionRequest
	confirmCtx  context.Context
	confirmID   string
	confirmReq  *listingkit.ConfirmSDSRetirementRunRequest
	retryCtx    context.Context
	retryRunID  string
	detail      *listingkit.SDSRetirementRunDetail
}

func (s *stubSDSRetirementHandlerService) CreateSDSRetirementRun(ctx context.Context, req *listingkit.CreateSDSRetirementRunRequest) (*listingkit.SDSRetirementRunDetail, error) {
	s.createCtx = ctx
	s.createReq = req
	return s.detail, nil
}

func (s *stubSDSRetirementHandlerService) GetSDSRetirementRun(ctx context.Context, runID string) (*listingkit.SDSRetirementRunDetail, error) {
	s.getCtx = ctx
	s.getRunID = runID
	return s.detail, nil
}

func (s *stubSDSRetirementHandlerService) UpdateSDSRetirementSelection(ctx context.Context, runID string, req *listingkit.UpdateSDSRetirementSelectionRequest) (*listingkit.SDSRetirementRunDetail, error) {
	s.updateCtx = ctx
	s.updateRunID = runID
	s.updateReq = req
	return s.detail, nil
}

func (s *stubSDSRetirementHandlerService) ConfirmSDSRetirementRun(ctx context.Context, runID string, req *listingkit.ConfirmSDSRetirementRunRequest) (*listingkit.SDSRetirementRunDetail, error) {
	s.confirmCtx = ctx
	s.confirmID = runID
	s.confirmReq = req
	return s.detail, nil
}

func (s *stubSDSRetirementHandlerService) RetrySDSRetirementRun(ctx context.Context, runID string) (*listingkit.SDSRetirementRunDetail, error) {
	s.retryCtx = ctx
	s.retryRunID = runID
	return s.detail, nil
}

func TestCreateSDSRetirementRunBindsRequestAndTenantScope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-1", Platform: "shein", Status: listingkit.SDSRetirementRunStatusReady},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements", h.CreateSDSRetirementRun)

	body := `{"platform":"shein","store_id":177,"parent_product_id":238915,"prototype_group_id":28345,"variant_id":238916,"created_by":"author"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	req.Header.Set("X-User-ID", "operator-a")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if svc.createReq == nil {
		t.Fatal("expected create request")
	}
	if svc.createReq.StoreID != 177 || svc.createReq.Platform != "shein" || svc.createReq.TenantID != "18" {
		t.Fatalf("create request = %+v", svc.createReq)
	}
	if got, ok := listingkit.TenantScopeFromContext(svc.createCtx); !ok || got != "18" {
		t.Fatalf("tenant scope = %q ok=%v, want 18 true", got, ok)
	}
	if got := listingkit.RequestUserIDFromContext(svc.createCtx); got != "operator-a" {
		t.Fatalf("user id = %q, want operator-a", got)
	}
}

func TestGetSDSRetirementRunUsesTenantScopedContext(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-2", Platform: "shein", Status: listingkit.SDSRetirementRunStatusReady},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/sds/retirements/:run_id", h.GetSDSRetirementRun)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/sds/retirements/run-2", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if svc.getRunID != "run-2" {
		t.Fatalf("get run id = %q, want run-2", svc.getRunID)
	}
	if got := listingkit.TenantIDFromContext(svc.getCtx); got != "18" {
		t.Fatalf("tenant id = %q, want 18", got)
	}
	if got, ok := listingkit.TenantScopeFromContext(svc.getCtx); !ok || got != "18" {
		t.Fatalf("tenant scope = %q ok=%v, want 18 true", got, ok)
	}
}

func TestUpdateSDSRetirementSelectionBindsRequestAndTenantScope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-3", Platform: "shein", Status: listingkit.SDSRetirementRunStatusReady},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.PATCH("/api/v1/listing-kits/sds/retirements/:run_id/items", h.UpdateSDSRetirementSelection)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/listing-kits/sds/retirements/run-3/items", strings.NewReader(`{"items":[{"item_id":"item-1","selected":false,"site_selection":"US"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if svc.updateRunID != "run-3" {
		t.Fatalf("update run id = %q, want run-3", svc.updateRunID)
	}
	if svc.updateReq == nil || len(svc.updateReq.Items) != 1 || svc.updateReq.Items[0].ItemID != "item-1" || svc.updateReq.Items[0].SiteSelection != "US" {
		t.Fatalf("update req = %+v", svc.updateReq)
	}
	if got, ok := listingkit.TenantScopeFromContext(svc.updateCtx); !ok || got != "18" {
		t.Fatalf("tenant scope = %q ok=%v, want 18 true", got, ok)
	}
}

func TestConfirmSDSRetirementRunBindsRequestAndTenantScope(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-4", Platform: "shein", Status: listingkit.SDSRetirementRunStatusSucceeded},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements/:run_id/confirm", h.ConfirmSDSRetirementRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements/run-4/confirm", strings.NewReader(`{"confirmed_by":"operator-confirm"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if svc.confirmID != "run-4" {
		t.Fatalf("confirm run id = %q, want run-4", svc.confirmID)
	}
	if svc.confirmReq == nil || svc.confirmReq.ConfirmedBy != "operator-confirm" {
		t.Fatalf("confirm req = %+v", svc.confirmReq)
	}
	if got, ok := listingkit.TenantScopeFromContext(svc.confirmCtx); !ok || got != "18" {
		t.Fatalf("tenant scope = %q ok=%v, want 18 true", got, ok)
	}
}

func TestRetrySDSRetirementRunUsesTenantScopedContext(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-5", Platform: "shein", Status: listingkit.SDSRetirementRunStatusSucceeded},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements/:run_id/retry", h.RetrySDSRetirementRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements/run-5/retry", nil)
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if svc.retryRunID != "run-5" {
		t.Fatalf("retry run id = %q, want run-5", svc.retryRunID)
	}
	if got, ok := listingkit.TenantScopeFromContext(svc.retryCtx); !ok || got != "18" {
		t.Fatalf("tenant scope = %q ok=%v, want 18 true", got, ok)
	}
}

func TestCreateSDSRetirementRunReturnsNotImplementedWithoutService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements", h.CreateSDSRetirementRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 body=%s", resp.Code, resp.Body.String())
	}
}

func TestCreateSDSRetirementRunReturnsBadRequestForInvalidJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(&stubSDSRetirementHandlerService{}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements", h.CreateSDSRetirementRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements", strings.NewReader(`{"platform":`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
}

func TestCreateSDSRetirementRunReturnsDetailPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSRetirementHandlerService{
		detail: &listingkit.SDSRetirementRunDetail{
			Run: listingkit.SDSRetirementRunRecord{ID: "run-9", Platform: "shein", Status: listingkit.SDSRetirementRunStatusReady},
			Items: []listingkit.SDSRetirementItemRecord{
				{ID: "item-1", RunID: "run-9", Platform: "shein", Status: listingkit.SDSRetirementItemStatusSelected},
			},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithSDSRetirementService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/sds/retirements", h.CreateSDSRetirementRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/sds/retirements", strings.NewReader(`{"platform":"shein","store_id":177,"parent_product_id":1,"prototype_group_id":2,"variant_id":3}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "18")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var body listingkit.SDSRetirementRunDetail
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Run.ID != "run-9" || len(body.Items) != 1 || body.Items[0].ID != "item-1" {
		t.Fatalf("body = %+v", body)
	}
}
