package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	a1688 "task-processor/internal/product/sourcehandoff/a1688"
	sourcea1688 "task-processor/internal/product/sourcehandoff/a1688/httpapi"
)

func TestAlibaba1688HTTPReplayCreatesTaskAndPreservesSourceFacts(t *testing.T) {
	creator := &replayGenerateTaskCreator{}
	router := newAlibaba1688ReplayRouter(creator)
	rec := performAuthenticatedReplayRequest(t, router, sourcea1688.CreateListingKitTaskRequest{
		URL:             "https://detail.1688.com/offer/321.html?spm=replay",
		Product:         replayProduct1688("321"),
		RawSnapshot:     "replay-snapshot-321",
		SourceRunID:     "replay-run-321",
		RequestID:       "replay-request-321",
		SourceAccountID: 3001,
		Platforms:       []string{" SHEIN ", "shein"},
		Country:         " US ",
		Language:        " en_US ",
		SheinStoreID:    168811,
	}, listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-1688"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response sourcea1688.CreateListingKitTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.TaskID != "task-replay-321" || response.TenantID != "101" {
		t.Fatalf("task response = %+v, want deterministic task and tenant", response)
	}
	if response.SourceIdentity.SourceKey() != "crawler:1688:321" || response.SourceIdentity.SourceID != "321" {
		t.Fatalf("source identity = %+v, want normalized 1688 identity", response.SourceIdentity)
	}
	if response.ProductURL != "https://detail.1688.com/offer/321.html" {
		t.Fatalf("product URL = %q, want normalized source URL", response.ProductURL)
	}
	if len(response.SourceWarnings) != 0 {
		t.Fatalf("source warnings = %+v, want none for complete fixture", response.SourceWarnings)
	}
	if creator.calls != 1 || creator.request == nil {
		t.Fatalf("creator calls/request = %d/%+v, want one captured request", creator.calls, creator.request)
	}
	if creator.request.TenantID != "101" || creator.request.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want authenticated identity", creator.request.TenantID, creator.request.UserID)
	}
	if len(creator.request.Platforms) != 1 || creator.request.Platforms[0] != "shein" {
		t.Fatalf("request platforms = %#v, want normalized shein", creator.request.Platforms)
	}
	if creator.request.SheinStoreID != 168811 {
		t.Fatalf("request SHEIN store = %d, want 168811", creator.request.SheinStoreID)
	}
	if creator.request.ProductURL != "https://detail.1688.com/offer/321.html" {
		t.Fatalf("request product URL = %q, want normalized source URL", creator.request.ProductURL)
	}
	if creator.request.Source == nil || creator.request.Source.Key != "crawler:1688:321" ||
		creator.request.Source.Platform != "1688" || creator.request.Source.ID != "321" ||
		creator.request.Source.URL != "https://detail.1688.com/offer/321.html" {
		t.Fatalf("request source = %+v, want normalized source reference", creator.request.Source)
	}
	requestJSON, err := json.Marshal(creator.request)
	if err != nil {
		t.Fatalf("marshal generated request: %v", err)
	}
	for _, forbidden := range []string{"password", "cookie", "user_data_dir", "profile_path", "proxy"} {
		if strings.Contains(strings.ToLower(string(requestJSON)), forbidden) {
			t.Fatalf("generated request JSON = %s, must not contain %q", requestJSON, forbidden)
		}
	}
	for _, want := range []string{
		"Title: Insulated Lunch Bag",
		"Brand: Factory Lunch",
		"Description: Thermal lunch bag with zipper.",
		"Attribute category: Bags>Lunch Bags",
		"Attribute min_price: 18.8",
		"Variant count: 1",
	} {
		if !strings.Contains(creator.request.Text, want) {
			t.Fatalf("request text = %q, missing %q", creator.request.Text, want)
		}
	}
	if len(creator.request.ImageURLs) != 4 {
		t.Fatalf("request images = %#v, want four source/variant images", creator.request.ImageURLs)
	}
}

func TestAlibaba1688HTTPReplayRejectsMissingFacts(t *testing.T) {
	creator := &replayGenerateTaskCreator{}
	router := newAlibaba1688ReplayRouter(creator)
	product := replayProduct1688("322")
	product.Title = ""
	product.MainImage = ""
	product.Images = nil
	product.ProductDetails = nil
	product.Variants = nil
	rec := performAuthenticatedReplayRequest(t, router, sourcea1688.CreateListingKitTaskRequest{
		URL:             "https://detail.1688.com/offer/322.html",
		Product:         product,
		SourceAccountID: 3001,
		Platforms:       []string{"shein"},
		SheinStoreID:    168811,
	}, listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-1688"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"] != "task_creation_failed" {
		t.Fatalf("error = %#v, want task_creation_failed", body["error"])
	}
	if !strings.Contains(body["message"].(string), "1688 source cannot create listingkit task") {
		t.Fatalf("message = %#v, want source task creation explanation", body["message"])
	}
	identity, ok := body["source_identity"].(map[string]any)
	if !ok || identity["SourceID"] != "322" {
		t.Fatalf("source identity = %#v, want source id 322", body["source_identity"])
	}
	warnings, ok := body["source_warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("source warnings = %#v, want missing-facts warning", body["source_warnings"])
	}
	if creator.calls != 0 {
		t.Fatalf("creator calls = %d, want no task creation", creator.calls)
	}
}

func TestAlibaba1688HTTPReplayPreservesSourceError(t *testing.T) {
	creator := &replayGenerateTaskCreator{}
	router := newAlibaba1688ReplayRouter(creator)
	rec := performAuthenticatedReplayRequest(t, router, sourcea1688.CreateListingKitTaskRequest{
		URL:             "https://detail.1688.com/offer/323.html",
		SourceError:     "controlled crawler failed",
		SourceAccountID: 3001,
		Platforms:       []string{"shein"},
		SheinStoreID:    168811,
	}, listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-1688"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"] != "task_creation_failed" {
		t.Fatalf("error = %#v, want task_creation_failed", body["error"])
	}
	identity, ok := body["source_identity"].(map[string]any)
	if !ok || identity["SourceID"] != "323" {
		t.Fatalf("source identity = %#v, want source id 323", body["source_identity"])
	}
	warnings, ok := body["source_warnings"].([]any)
	foundSourceError := false
	for _, warning := range warnings {
		if strings.Contains(fmt.Sprint(warning), "controlled crawler failed") {
			foundSourceError = true
			break
		}
	}
	if !ok || len(warnings) == 0 || !foundSourceError {
		t.Fatalf("source warnings = %#v, want controlled source error", body["source_warnings"])
	}
	if creator.calls != 0 {
		t.Fatalf("creator calls = %d, want no task creation", creator.calls)
	}
}

type replayGenerateTaskCreator struct {
	request *listingkit.GenerateRequest
	calls   int
}

func (f *replayGenerateTaskCreator) CreateGenerateTask(_ context.Context, request *listingkit.GenerateRequest) (*listingkit.Task, error) {
	f.calls++
	f.request = request
	return &listingkit.Task{
		ID:       "task-replay-321",
		TenantID: request.TenantID,
		UserID:   request.UserID,
		Request:  request,
		Status:   core.TaskStatusPending,
	}, nil
}

func TestAlibaba1688HTTPReplayRejectsUnavailableSourceAccountAndPreservesSheinTargetValidation(t *testing.T) {
	tests := []struct {
		name      string
		validator replayStoreAccessValidator
		wantCode  string
		wantText  string
	}{
		{
			name: "disabled source account",
			validator: replayStoreAccessValidator{errs: map[replayStoreAccessKey]error{
				{storeID: 3001, platform: "1688"}: listingkit.NewStoreAccessError(listingkit.StoreAccessDisabled, "store is disabled"),
			}},
			wantCode: listingkit.StoreAccessDisabled,
			wantText: "1688 login account",
		},
		{
			name: "foreign source account",
			validator: replayStoreAccessValidator{errs: map[replayStoreAccessKey]error{
				{storeID: 3001, platform: "1688"}: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable"),
			}},
			wantCode: listingkit.StoreAccessUnavailable,
			wantText: "1688 login account",
		},
		{
			name: "disabled SHEIN target store",
			validator: replayStoreAccessValidator{errs: map[replayStoreAccessKey]error{
				{storeID: 168811, platform: "SHEIN"}: listingkit.NewStoreAccessError(listingkit.StoreAccessDisabled, "store is disabled"),
			}},
			wantCode: listingkit.StoreAccessDisabled,
			wantText: "SHEIN target store",
		},
		{
			name: "foreign SHEIN target store",
			validator: replayStoreAccessValidator{errs: map[replayStoreAccessKey]error{
				{storeID: 168811, platform: "SHEIN"}: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable"),
			}},
			wantCode: listingkit.StoreAccessUnavailable,
			wantText: "SHEIN target store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &replayGenerateTaskCreator{}
			router := newAlibaba1688ReplayRouterWithValidator(creator, tt.validator)
			rec := performAuthenticatedReplayRequest(t, router, sourcea1688.CreateListingKitTaskRequest{
				URL:             "https://detail.1688.com/offer/324.html",
				Product:         replayProduct1688("324"),
				SourceAccountID: 3001,
				Platforms:       []string{"shein"},
				SheinStoreID:    168811,
			}, listingkit.AuthenticatedIdentity{TenantID: "101", UserID: "user-1688"})

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if body["error"] != tt.wantCode {
				t.Fatalf("error = %#v, want %q", body["error"], tt.wantCode)
			}
			if !strings.Contains(body["message"].(string), tt.wantText) {
				t.Fatalf("message = %#v, want %q", body["message"], tt.wantText)
			}
			if creator.calls != 0 {
				t.Fatalf("creator calls = %d, want no task creation", creator.calls)
			}
		})
	}
}

type replayStoreAccessKey struct {
	storeID  int64
	platform string
}

type replayStoreAccessValidator struct {
	errs map[replayStoreAccessKey]error
}

func (v replayStoreAccessValidator) ValidateStoreAccess(_ context.Context, tenantID, storeID int64, platform string) (listingkit.StoreAccess, error) {
	if err := v.errs[replayStoreAccessKey{storeID: storeID, platform: platform}]; err != nil {
		return listingkit.StoreAccess{}, err
	}
	if tenantID == 101 &&
		((storeID == 3001 && platform == "1688") || (storeID == 168811 && platform == "SHEIN")) {
		return listingkit.StoreAccess{ID: storeID, TenantID: tenantID, Platform: platform, Enabled: true}, nil
	}
	return listingkit.StoreAccess{}, fmt.Errorf("unexpected replay store access: tenant=%d store=%d platform=%s", tenantID, storeID, platform)
}

func newAlibaba1688ReplayRouter(creator *replayGenerateTaskCreator) http.Handler {
	return newAlibaba1688ReplayRouterWithValidator(creator, replayStoreAccessValidator{})
}

func newAlibaba1688ReplayRouterWithValidator(creator *replayGenerateTaskCreator, validator replayStoreAccessValidator) http.Handler {
	gin.SetMode(gin.TestMode)
	service := a1688.NewTaskCommandService(creator, validator)
	router := gin.New()
	router.POST("/api/v1/product-sourcing/1688/listingkit/tasks", sourcea1688.NewHandler(service).CreateListingKitTask)
	return router
}

func performAuthenticatedReplayRequest(t *testing.T, router http.Handler, body sourcea1688.CreateListingKitTaskRequest, identity listingkit.AuthenticatedIdentity) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal replay request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/product-sourcing/1688/listingkit/tasks", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(listingkit.WithAuthenticatedIdentity(req.Context(), identity))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func replayProduct1688(id string) *alibaba1688model.Product1688 {
	return &alibaba1688model.Product1688{
		ID:        id,
		Title:     "Insulated Lunch Bag",
		URL:       "https://detail.1688.com/offer/" + id + ".html?from=replay",
		MainImage: "https://img.example/" + id + "-main.jpg",
		Images:    []string{"https://img.example/" + id + "-main.jpg", "https://img.example/" + id + "-side.jpg"},
		MinPrice:  18.8,
		Currency:  "CNY",
		Category:  "Bags>Lunch Bags",
		Brand:     "Factory Lunch",
		Supplier:  alibaba1688model.SupplierInfo{ID: "supplier-" + id, Name: "Lunch Factory"},
		ProductDetails: []alibaba1688model.ProductDetail{{
			Content: "Thermal lunch bag with zipper.",
			Images:  []string{"https://img.example/" + id + "-detail.jpg"},
		}},
		Variants: []alibaba1688model.Variant{{
			Name:       "Black",
			Image:      "https://img.example/" + id + "-black.jpg",
			Price:      19.9,
			Attributes: map[string]any{"Color": "Black"},
		}},
	}
}
