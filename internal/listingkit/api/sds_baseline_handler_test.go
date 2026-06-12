package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubSDSBaselineHandlerService struct {
	stubTaskLifecycleHandlerService
	readiness *listingkit.SDSBaselineReadiness
	query     *listingkit.SDSBaselineReadinessQuery
	err       error
}

func (s *stubSDSBaselineHandlerService) GetSDSBaselineReadiness(_ context.Context, query *listingkit.SDSBaselineReadinessQuery) (*listingkit.SDSBaselineReadiness, error) {
	s.query = query
	return s.readiness, s.err
}

func TestGetSDSBaselineReadinessBindsQueryAndReturnsPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSDSBaselineHandlerService{
		readiness: &listingkit.SDSBaselineReadiness{
			BaselineKey: "baseline-key",
			Status:      listingkit.SDSBaselineStatusBaselineCached,
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc), WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/sds/baselines/readiness", h.GetSDSBaselineReadiness)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/listing-kits/sds/baselines/readiness?tenant_id=tenant-a&parent_product_id=9001&prototype_group_id=7001&variant_id=101&selected_variant_ids=101,102",
		nil,
	)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET readiness = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if svc.query == nil {
		t.Fatal("expected GetSDSBaselineReadiness to receive query")
	}
	if svc.query.TenantID != "tenant-a" ||
		svc.query.ParentProductID != 9001 ||
		svc.query.PrototypeGroupID != 7001 ||
		svc.query.VariantID != 101 {
		t.Fatalf("query = %+v, want bound ids", svc.query)
	}
	if len(svc.query.SelectedVariantIDs) != 2 ||
		svc.query.SelectedVariantIDs[0] != 101 ||
		svc.query.SelectedVariantIDs[1] != 102 {
		t.Fatalf("query = %+v, want selected variant ids", svc.query)
	}

	var body listingkit.SDSBaselineReadiness
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body.Status != listingkit.SDSBaselineStatusBaselineCached || body.BaselineKey != "baseline-key" {
		t.Fatalf("body = %+v, want readiness payload", body)
	}
}

func TestGetSDSBaselineReadinessReturnsBadRequestForInvalidSelectedVariantIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/sds/baselines/readiness", h.GetSDSBaselineReadiness)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/listing-kits/sds/baselines/readiness?parent_product_id=9001&prototype_group_id=7001&variant_id=101&selected_variant_ids=101,abc",
		nil,
	)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET readiness = %d, want 400; body=%s", resp.Code, resp.Body.String())
	}
}
