package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

type stubStoreProfileAdminService struct {
	profiles  []listingkit.ListingKitStoreProfile
	upserted  *listingkit.ListingKitStoreProfile
	upsertReq *listingkit.ListingKitStoreProfile
	err       error
}

func (s *stubStoreProfileAdminService) ListSheinStoreProfiles(context.Context) ([]listingkit.ListingKitStoreProfile, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]listingkit.ListingKitStoreProfile(nil), s.profiles...), nil
}

func (s *stubStoreProfileAdminService) UpsertSheinStoreProfile(_ context.Context, req *listingkit.ListingKitStoreProfile) (*listingkit.ListingKitStoreProfile, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.upsertReq = req
	if s.upserted != nil {
		return s.upserted, nil
	}
	return req, nil
}

func (s *stubStoreProfileAdminService) DeleteSheinStoreProfile(context.Context, int64) error {
	return s.err
}

func (s *stubStoreProfileAdminService) PreviewSheinPrice(context.Context, string, *listingkit.SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStoreProfileAdminService) SearchSheinCategories(context.Context, string, string) (*listingkit.SheinCategorySearchResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStoreProfileAdminService) UpdateSheinFinalDraft(context.Context, string, *listingkit.SheinFinalDraftUpdateRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStoreProfileAdminService) GetSubmissionEvents(context.Context, string) (*listingkit.SheinSubmissionEventPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStoreProfileAdminService) ClearSheinResolutionCache(context.Context, string, string) (*listingkit.SheinResolutionCacheClearResult, error) {
	return nil, errors.New("not implemented")
}

func TestListSheinStoreProfilesReturnsProfiles(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	adminSvc := &stubStoreProfileAdminService{
		profiles: []listingkit.ListingKitStoreProfile{
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
	h, err := NewHandler(&stubHandlerCoreService{}, WithStoreAdminService(adminSvc), WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
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
	adminSvc := &stubStoreProfileAdminService{
		upserted: &listingkit.ListingKitStoreProfile{
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
	h, err := NewHandler(&stubHandlerCoreService{}, WithStoreAdminService(adminSvc), WithSubscriptionService(activeStudioOnlySubscriptionService(t)))
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
	if adminSvc.upsertReq == nil {
		t.Fatal("expected UpsertSheinStoreProfile to be called")
	}
	if adminSvc.upsertReq.StoreID != 870 || adminSvc.upsertReq.DefaultSubmitMode != "save_draft" {
		t.Fatalf("request = %+v, want bound store profile request", adminSvc.upsertReq)
	}
}
