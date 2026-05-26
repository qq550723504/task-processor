package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestGetSDSBaselineReadinessBindsQueryAndReturnsPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		baselineReadiness: &listingkit.SDSBaselineReadiness{
			BaselineKey: "baseline-key",
			Status:      "ready",
		},
	}
	h, err := NewHandler(svc, WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
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
	if svc.baselineReadinessQuery == nil {
		t.Fatal("expected GetSDSBaselineReadiness to receive query")
	}
	if svc.baselineReadinessQuery.TenantID != "tenant-a" ||
		svc.baselineReadinessQuery.ParentProductID != 9001 ||
		svc.baselineReadinessQuery.PrototypeGroupID != 7001 ||
		svc.baselineReadinessQuery.VariantID != 101 {
		t.Fatalf("query = %+v, want bound ids", svc.baselineReadinessQuery)
	}
	if len(svc.baselineReadinessQuery.SelectedVariantIDs) != 2 ||
		svc.baselineReadinessQuery.SelectedVariantIDs[0] != 101 ||
		svc.baselineReadinessQuery.SelectedVariantIDs[1] != 102 {
		t.Fatalf("query = %+v, want selected variant ids", svc.baselineReadinessQuery)
	}

	var body listingkit.SDSBaselineReadiness
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body.Status != "ready" || body.BaselineKey != "baseline-key" {
		t.Fatalf("body = %+v, want readiness payload", body)
	}
}

func TestGetSDSBaselineReadinessReturnsBadRequestForInvalidSelectedVariantIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{}, WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
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
