package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestDispatchTaskGenerationNavigationBindsTarget(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		navigation: &listingkit.GenerationReviewNavigationDispatchResponse{
			TaskID:       "task-1",
			DispatchKind: "preview",
			ReviewPreview: &listingkit.GenerationReviewPreviewResponse{
				TaskID: "task-1",
			},
			PanelUpdate: &listingkit.GenerationReviewPanelUpdate{
				DispatchKind: "preview",
			},
		},
	}
	h := newGenerationTaskHandler(t, svc)

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", h.DispatchTaskGenerationNavigation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-navigation/dispatch", strings.NewReader(`{"response_mode":"patch_only","target":{"dispatch_kind":"preview","preview_query":{"platform":"shein","slot":"main","asset_id":"asset-preview-1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.navigationReq == nil || svc.navigationReq.ResponseMode != "patch_only" || svc.navigationReq.Target == nil || svc.navigationReq.Target.DispatchKind != "preview" {
		t.Fatalf("navigation req = %+v, want dispatch target bound", svc.navigationReq)
	}
	if svc.navigationReq.Target.PreviewQuery == nil || svc.navigationReq.Target.PreviewQuery.AssetID != "asset-preview-1" {
		t.Fatalf("navigation req = %+v, want preview query bound", svc.navigationReq)
	}
	if resp.Header().Get("ETag") != "" {
		t.Fatalf("etag = %q, want empty ETag when response has no delta token", resp.Header().Get("ETag"))
	}
	var body listingkit.GenerationReviewNavigationDispatchResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.DispatchKind != "preview" || body.ReviewPreview == nil {
		t.Fatalf("body = %+v, want preview dispatch response", body)
	}
	if body.PanelUpdate == nil || body.PanelUpdate.DispatchKind != "preview" {
		t.Fatalf("body = %+v, want normalized panel update", body)
	}
}

func TestDispatchTaskGenerationNavigationAppliesIfNoneMatchToTargetAndWritesETag(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		navigation: &listingkit.GenerationReviewNavigationDispatchResponse{
			TaskID:       "task-1",
			DispatchKind: "preview",
			DeltaToken:   "delta-nav-1",
			NotModified:  true,
			ReviewPreview: &listingkit.GenerationReviewPreviewResponse{
				TaskID:      "task-1",
				DeltaToken:  "delta-nav-1",
				NotModified: true,
			},
			PanelUpdate: &listingkit.GenerationReviewPanelUpdate{
				DispatchKind: "preview",
				DeltaToken:   "delta-nav-1",
				NoChanges:    true,
			},
		},
	}
	h := newGenerationTaskHandler(t, svc)

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", h.DispatchTaskGenerationNavigation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-navigation/dispatch", strings.NewReader(`{"response_mode":"patch_only","target":{"dispatch_kind":"preview","preview_query":{"platform":"shein","slot":"main","asset_id":"asset-preview-1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-None-Match", `"delta-nav-1"`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.navigationReq == nil || svc.navigationReq.Target == nil || svc.navigationReq.Target.PreviewQuery == nil || svc.navigationReq.Target.PreviewQuery.IfMatch != "delta-nav-1" {
		t.Fatalf("navigation req = %+v, want If-None-Match propagated into preview query", svc.navigationReq)
	}
	if resp.Header().Get("ETag") != `"delta-nav-1"` {
		t.Fatalf("etag = %q, want quoted navigation delta token", resp.Header().Get("ETag"))
	}
	var body listingkit.GenerationReviewNavigationDispatchResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.NotModified || body.DeltaToken != "delta-nav-1" {
		t.Fatalf("body = %+v, want navigation not_modified body", body)
	}
}

func TestDispatchTaskGenerationNavigationUsesTargetConditionalBaselineBeforeHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		navigation: &listingkit.GenerationReviewNavigationDispatchResponse{
			TaskID:       "task-1",
			DispatchKind: "preview",
		},
	}
	h := newGenerationTaskHandler(t, svc)

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-navigation/dispatch", h.DispatchTaskGenerationNavigation)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-navigation/dispatch", strings.NewReader(`{"target":{"dispatch_kind":"preview","conditional":{"delta_token":"delta-body"},"preview_query":{"platform":"shein","slot":"main","asset_id":"asset-preview-1"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-None-Match", `"delta-header"`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.navigationReq == nil || svc.navigationReq.Target == nil || svc.navigationReq.Target.PreviewQuery == nil {
		t.Fatalf("navigation req = %+v, want bound preview query", svc.navigationReq)
	}
	if svc.navigationReq.Target.PreviewQuery.IfMatch != "delta-body" {
		t.Fatalf("navigation req = %+v, want body conditional baseline to win over header", svc.navigationReq)
	}
}
