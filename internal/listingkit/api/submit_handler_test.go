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
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
)

type stubSubmitService struct {
	stubTaskLifecycleHandlerService
	preview *listingkit.ListingKitPreview
	err     error
	lastReq *listingkit.SubmitTaskRequest
}

func (s *stubSubmitService) SubmitTask(_ context.Context, taskID string, req *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	s.lastReq = req
	return s.preview, s.err
}
func (s *stubSubmitService) RefreshSubmissionStatus(_ context.Context, taskID string) (*listingkit.ListingKitPreview, error) {
	return s.preview, s.err
}

func TestSubmitTaskReturnsBadRequestWhenBlocked(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{err: listingkit.ErrSubmitBlocked}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submit", h.SubmitTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submit", strings.NewReader(`{"platform":"shein","action":"publish"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestSubmitTaskReturnsConflictWhenSubmitInProgress(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{err: core.ErrSubmitInProgress}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submit", h.SubmitTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submit", strings.NewReader(`{"platform":"shein","action":"publish"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.Code)
	}
}

func TestSubmitTaskConflictIncludesCurrentPhaseAndLease(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	leaseExpiresAt := time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC)
	svc := &stubSubmitService{err: &submission.SubmitInProgressError{
		Platform:       "shein",
		Action:         "publish",
		Phase:          "submit_remote",
		RequestID:      "request-123",
		LeaseExpiresAt: &leaseExpiresAt,
	}}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submit", h.SubmitTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submit", strings.NewReader(`{"platform":"shein","action":"publish"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["current_phase"] != "submit_remote" || body["current_request_id"] != "request-123" {
		t.Fatalf("conflict body = %+v", body)
	}
	if body["lease_expires_at"] == "" {
		t.Fatalf("lease_expires_at missing from body: %+v", body)
	}
}

func TestSubmitTaskReturnsPreviewPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{
		preview: &listingkit.ListingKitPreview{
			TaskID: "task-1",
			Shein: &listingkit.SheinPreviewPayload{
				Submission: &listingkit.SheinSubmissionReport{
					LastAction: "publish",
					LastStatus: "success",
				},
			},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submit", h.SubmitTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submit", strings.NewReader(`{"platform":"shein","action":"publish"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var body listingkit.ListingKitPreview
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Shein == nil || body.Shein.Submission == nil || body.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submit preview = %+v", body.Shein)
	}
}

func TestSubmitTaskMapsIdempotencyKeyHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{preview: &listingkit.ListingKitPreview{TaskID: "task-1"}}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submit", h.SubmitTask)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submit", strings.NewReader(`{"platform":"shein","action":"publish"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "submit-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastReq == nil {
		t.Fatal("expected submit request")
	}
	if svc.lastReq.IdempotencyKey != "submit-123" {
		t.Fatalf("idempotency key = %q, want submit-123", svc.lastReq.IdempotencyKey)
	}
}

func TestRefreshSubmissionStatusReturnsPreviewPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{
		preview: &listingkit.ListingKitPreview{
			TaskID: "task-1",
			Shein: &listingkit.SheinPreviewPayload{
				Submission: &listingkit.SheinSubmissionReport{
					RemoteStatus: "confirmed",
				},
			},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithTaskLifecycleService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/submission-status/refresh", h.RefreshSubmissionStatus)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/submission-status/refresh", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var body listingkit.ListingKitPreview
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Shein == nil || body.Shein.Submission == nil || body.Shein.Submission.RemoteStatus != "confirmed" {
		t.Fatalf("refresh preview = %+v", body.Shein)
	}
}
