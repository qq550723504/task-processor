package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestListSheinStoreProfilesReturnsProfiles(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		storeProfiles: []listingkit.ListingKitStoreProfile{
			{
				ID:                1,
				TenantID:          101,
				StoreID:           869,
				Enabled:           true,
				Priority:          10,
				Site:              "US",
				WarehouseCode:     "WH-US-1",
				DefaultStock:      100,
				DefaultSubmitMode: "publish",
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	router.GET("/store-profiles", h.ListSheinStoreProfiles)

	req := httptest.NewRequest(http.MethodGet, "/store-profiles", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-profiles = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	var body struct {
		Items []listingkit.ListingKitStoreProfile `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].StoreID != 869 {
		t.Fatalf("body = %+v, want one shein store profile", body)
	}
}

func TestUpsertSheinStoreProfileBindsRequest(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		upsertedStoreProfile: &listingkit.ListingKitStoreProfile{
			ID:                2,
			TenantID:          101,
			StoreID:           870,
			Enabled:           true,
			Priority:          5,
			Site:              "US",
			WarehouseCode:     "WH-US-2",
			DefaultStock:      120,
			DefaultSubmitMode: "save_draft",
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	router.POST("/store-profiles", h.UpsertSheinStoreProfile)

	body, err := json.Marshal(map[string]any{
		"store_id":            870,
		"enabled":             true,
		"priority":            5,
		"site":                "us",
		"warehouse_code":      "WH-US-2",
		"default_stock":       120,
		"default_submit_mode": "save_draft",
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/store-profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("POST /store-profiles = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if svc.upsertStoreProfileReq == nil {
		t.Fatal("expected UpsertSheinStoreProfile to be called")
	}
	if svc.upsertStoreProfileReq.StoreID != 870 || svc.upsertStoreProfileReq.DefaultSubmitMode != "save_draft" {
		t.Fatalf("request = %+v, want bound store profile request", svc.upsertStoreProfileReq)
	}
}

func TestUpdateSheinStoreRoutingSettingsBindsRequest(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		storeRoutingSettings: &listingkit.ListingKitStoreRoutingSettings{
			TenantID:            101,
			SelectionStrategy:   "priority",
			FallbackStoreID:     870,
			AllowManualOverride: true,
			AllowFallback:       true,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	router.PUT("/store-routing", h.UpdateSheinStoreRoutingSettings)

	body, err := json.Marshal(map[string]any{
		"selection_strategy":    "priority",
		"fallback_store_id":     870,
		"allow_manual_override": true,
		"allow_fallback":        true,
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/store-routing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /store-routing = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}
	if svc.updateStoreRoutingReq == nil {
		t.Fatal("expected UpdateSheinStoreRoutingSettings to be called")
	}
	if svc.updateStoreRoutingReq.FallbackStoreID != 870 || svc.updateStoreRoutingReq.SelectionStrategy != "priority" {
		t.Fatalf("request = %+v, want bound routing settings request", svc.updateStoreRoutingReq)
	}
}
