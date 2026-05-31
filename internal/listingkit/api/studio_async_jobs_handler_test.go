package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingsubscription"
)

func TestStudioAsyncJobStartsAndReturnsSucceededDesignJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	subscriptionService := activeStudioSubscriptionService(t)
	h, err := NewHandler(svc, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	var started struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal start body: %v", err)
	}
	if started.JobID == "" || started.Status != "running" {
		t.Fatalf("started = %+v, want running job id", started)
	}

	var polled struct {
		Status string                           `json:"status"`
		Result *listingkit.StudioDesignResponse `json:"result"`
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+started.JobID, nil)
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200 body=%s", resp.Code, resp.Body.String())
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &polled); err != nil {
			t.Fatalf("unmarshal poll body: %v", err)
		}
		if polled.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if polled.Status != "succeeded" || polled.Result == nil || len(polled.Result.Images) != 1 {
		t.Fatalf("polled = %+v, want succeeded design result", polled)
	}
	if svc.studioDesignReq == nil || svc.studioDesignReq.Prompt != "retro cherries" {
		t.Fatalf("studio design req = %+v, want bound prompt", svc.studioDesignReq)
	}
	summary, err := subscriptionService.GetSummary(t.Context(), listingkit.DefaultTenantID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	var studioUsage int
	for _, item := range summary.Entitlements {
		if item.Module.Code == listingsubscription.ModuleStudio {
			studioUsage = item.Used["design_jobs"]
			break
		}
	}
	if studioUsage != 1 {
		t.Fatalf("studio design_jobs usage = %d, want 1", studioUsage)
	}
}

func TestStartStudioAsyncJobUsesSharedStudioBatchExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "inline service should not run",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-inline",
				ImageURL: "https://example.com/inline.png",
			}},
		},
	}
	h, err := NewHandler(svc, WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	originalExecute := executeStudioDesignBatch
	executeStudioDesignBatch = func(ctx context.Context, service listingkit.StudioMediaService, input listingkit.StudioBatchGenerateExecutionInput) (*listingkit.StudioBatchGenerateExecutionOutput, error) {
		if service != svc {
			t.Fatalf("service = %T, want stubGenerationTaskService", service)
		}
		if input.Request == nil || input.Request.Prompt != "retro cherries" {
			t.Fatalf("input request = %+v, want bound prompt", input.Request)
		}
		return &listingkit.StudioBatchGenerateExecutionOutput{
			Response: &listingkit.StudioDesignResponse{
				Prompt: "shared seam response",
				Images: []listingkit.StudioGeneratedImage{{
					ID:       "design-shared",
					ImageURL: "/uploads/design-shared.png",
				}},
			},
			SessionID: input.SessionID,
		}, nil
	}
	t.Cleanup(func() {
		executeStudioDesignBatch = originalExecute
	})

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	var started struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &started); err != nil {
		t.Fatalf("unmarshal start body: %v", err)
	}

	var polled struct {
		Status string                           `json:"status"`
		Result *listingkit.StudioDesignResponse `json:"result"`
	}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/"+started.JobID, nil)
		resp = httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200 body=%s", resp.Code, resp.Body.String())
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &polled); err != nil {
			t.Fatalf("unmarshal poll body: %v", err)
		}
		if polled.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if polled.Status != "succeeded" || polled.Result == nil || polled.Result.Prompt != "shared seam response" {
		t.Fatalf("polled = %+v, want shared seam result", polled)
	}
	if svc.studioDesignReq != nil {
		t.Fatalf("studio design req = %+v, want handler to avoid inline service execution", svc.studioDesignReq)
	}
}

func activeStudioSubscriptionService(t *testing.T) *listingsubscription.Service {
	t.Helper()
	svc, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertEntitlement(t.Context(), listingkit.DefaultTenantID, listingsubscription.ModuleStudio, listingsubscription.EntitlementInput{Status: listingsubscription.StatusActive}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestStudioAsyncJobRejectsUnknownPath(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/unknown","body":{}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestStudioAsyncJobRequiresStudioSubscription(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	subscriptionService, err := listingsubscription.NewService(listingsubscription.NewMemRepository())
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	h, err := NewHandler(&stubGenerationTaskService{}, WithSubscriptionService(subscriptionService))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"subscription_required"`) {
		t.Fatalf("body = %s, want subscription_required", resp.Body.String())
	}
}

func TestStudioAsyncJobReturnsNotFoundForMissingJob(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubGenerationTaskService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/async-jobs/:job_id", h.GetStudioAsyncJob)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/async-jobs/missing", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestStudioAsyncJobSyncsSessionWhenDesignJobStarts(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	h, err := NewHandler(svc, WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","session_id":"session-1","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	if svc.updatedStudioSessionID != "session-1" {
		t.Fatalf("updated session id = %q, want session-1", svc.updatedStudioSessionID)
	}
	if svc.updatedStudioSessionReq == nil || svc.updatedStudioSessionReq.Status == nil {
		t.Fatalf("updated session req = %+v, want synced session status", svc.updatedStudioSessionReq)
	}
	if got := *svc.updatedStudioSessionReq.Status; got != listingkit.SheinStudioSessionStatusGenerating && got != listingkit.SheinStudioSessionStatusGenerated {
		t.Fatalf("session status = %q, want generating/generated", got)
	}
	if svc.updatedStudioSessionReq.GenerationJobID == nil || strings.TrimSpace(*svc.updatedStudioSessionReq.GenerationJobID) == "" {
		t.Fatalf("generation job id = %+v, want non-empty", svc.updatedStudioSessionReq.GenerationJobID)
	}
}

func TestStudioAsyncJobPropagatesTraceContextToDesignExecution(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		studioDesigns: &listingkit.StudioDesignResponse{
			Prompt: "retro cherries",
			Images: []listingkit.StudioGeneratedImage{{
				ID:       "design-1",
				ImageURL: "https://example.com/design.png",
			}},
		},
	}
	h, err := NewHandler(svc, WithSubscriptionService(activeStudioSubscriptionService(t)))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/async-jobs", h.StartStudioAsyncJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/async-jobs", strings.NewReader(`{"path":"/studio/designs","session_id":"session-1","body":{"prompt":"retro cherries","count":1}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ListingKit-Batch-Run-Id", "run-1")
	req.Header.Set("X-ListingKit-Batch-Id", "batch-1")
	req.Header.Set("X-ListingKit-Studio-Session-Id", "session-1")
	req.Header.Set("X-ListingKit-Queue-Mode", "generate")
	req.Header.Set("X-ListingKit-Queue-Index", "2")
	req.Header.Set("X-ListingKit-Queue-Total", "5")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		if svc.studioDesignCtx != nil && svc.updatedStudioSessionCtx != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if svc.studioDesignCtx == nil {
		t.Fatal("studio design ctx was not captured")
	}
	if svc.updatedStudioSessionCtx == nil {
		t.Fatal("updated studio session ctx was not captured")
	}

	trace := listingkit.RequestTraceFromContext(svc.studioDesignCtx)
	if trace.BatchRunID != "run-1" || trace.BatchID != "batch-1" || trace.SessionID != "session-1" {
		t.Fatalf("studio design trace = %+v, want run/batch/session ids", trace)
	}
	if trace.QueueMode != "generate" || trace.QueueIndex != 2 || trace.QueueTotal != 5 {
		t.Fatalf("studio design trace = %+v, want queue fields", trace)
	}

	sessionTrace := listingkit.RequestTraceFromContext(svc.updatedStudioSessionCtx)
	if sessionTrace.BatchRunID != "run-1" || sessionTrace.SessionID != "session-1" {
		t.Fatalf("updated session trace = %+v, want propagated trace fields", sessionTrace)
	}
}
