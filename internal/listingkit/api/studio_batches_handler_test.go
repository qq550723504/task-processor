package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubStudioBatchActionService struct {
	startCtx           context.Context
	startBatchID       string
	startResult        *listingkit.StudioBatchDetail
	startErr           error
	retryCtx           context.Context
	retryBatchID       string
	retryReq           *listingkit.RetryStudioBatchItemsRequest
	retryResult        *listingkit.StudioBatchDetail
	retryErr           error
	approveCtx         context.Context
	approveBatchID     string
	approveReq         *listingkit.ApproveStudioBatchDesignsRequest
	approveResult      *listingkit.StudioBatchDetail
	approveErr         error
	createTasksCtx     context.Context
	createTasksBatchID string
	createTasksReq     *listingkit.CreateStudioBatchTasksRequest
	createTasksResult  *listingkit.CreateStudioBatchTasksResult
	createTasksErr     error
}

func (s *stubStudioBatchActionService) EnsureStudioSession(context.Context, *listingkit.EnsureStudioSessionRequest) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) GetStudioSession(context.Context, string) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) UpdateStudioSession(context.Context, string, *listingkit.UpdateStudioSessionRequest) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) ReplaceStudioSessionDesigns(context.Context, string, *listingkit.ReplaceStudioSessionDesignsRequest) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) ListStudioSessionGallery(context.Context, int) (*listingkit.StudioSessionGalleryResponse, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) ListStudioBatches(context.Context, int) (*listingkit.StudioBatchListResponse, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) GetStudioBatch(context.Context, string) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) UpsertStudioBatch(context.Context, *listingkit.UpsertStudioBatchRequest) (*listingkit.SheinStudioSessionDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) DeleteStudioBatch(context.Context, string) error {
	return nil
}

func (s *stubStudioBatchActionService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*listingkit.StudioBatchDetail, error) {
	s.startCtx = ctx
	s.startBatchID = batchID
	return s.startResult, s.startErr
}

func (s *stubStudioBatchActionService) RetryStudioBatchItems(ctx context.Context, batchID string, req *listingkit.RetryStudioBatchItemsRequest) (*listingkit.StudioBatchDetail, error) {
	s.retryCtx = ctx
	s.retryBatchID = batchID
	s.retryReq = req
	return s.retryResult, s.retryErr
}

func (s *stubStudioBatchActionService) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *listingkit.ApproveStudioBatchDesignsRequest) (*listingkit.StudioBatchDetail, error) {
	s.approveCtx = ctx
	s.approveBatchID = batchID
	s.approveReq = req
	return s.approveResult, s.approveErr
}

func (s *stubStudioBatchActionService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *listingkit.CreateStudioBatchTasksRequest) (*listingkit.CreateStudioBatchTasksResult, error) {
	s.createTasksCtx = ctx
	s.createTasksBatchID = batchID
	s.createTasksReq = req
	return s.createTasksResult, s.createTasksErr
}

func TestStudioBatchGenerateHandlerStartsItemizedGeneration(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		startResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
			Items: []listingkit.StudioBatchItemDetail{{
				Item: listingkit.StudioBatchItemRecord{ID: "item-1"},
			}},
		},
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/generate", h.StartStudioBatchGeneration)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/generate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if svc.startBatchID != "batch-1" {
		t.Fatalf("start batch id = %q, want batch-1", svc.startBatchID)
	}
	if !strings.Contains(rec.Body.String(), "\"items\"") {
		t.Fatalf("body = %s, want itemized detail payload", rec.Body.String())
	}
}

func TestStudioBatchApproveDesignsHandlerBindsIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		approveResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
		},
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/design-approvals", h.ApproveStudioBatchDesigns)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/design-approvals", strings.NewReader(`{"design_ids":["design-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if svc.approveBatchID != "batch-1" {
		t.Fatalf("approve batch id = %q, want batch-1", svc.approveBatchID)
	}
	if svc.approveReq == nil || len(svc.approveReq.DesignIDs) != 1 || svc.approveReq.DesignIDs[0] != "design-1" {
		t.Fatalf("approve req = %+v, want bound design id", svc.approveReq)
	}
}

func TestStudioBatchRetryItemsHandlerBindsIDs(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		retryResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
		},
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/items/retry", h.RetryStudioBatchItems)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/items/retry", strings.NewReader(`{"item_ids":["item-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if svc.retryBatchID != "batch-1" {
		t.Fatalf("retry batch id = %q, want batch-1", svc.retryBatchID)
	}
	if svc.retryReq == nil || len(svc.retryReq.ItemIDs) != 1 || svc.retryReq.ItemIDs[0] != "item-1" {
		t.Fatalf("retry req = %+v, want bound item id", svc.retryReq)
	}
}

func TestStudioBatchRetryItemsHandlerReturnsBadRequestForValidationErrors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		retryErr: listingkit.NewStudioBatchActionValidationError("item item-1 is not retryable"),
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/items/retry", h.RetryStudioBatchItems)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/items/retry", strings.NewReader(`{"item_ids":["item-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	if !errors.Is(svc.retryErr, listingkit.ErrStudioBatchActionValidation) {
		t.Fatalf("retry err = %v, want validation sentinel", svc.retryErr)
	}
}

func TestStudioBatchTasksHandlerUsesApprovedDesignOwnership(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		createTasksResult: &listingkit.CreateStudioBatchTasksResult{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1", Status: listingkit.StudioBatchStatusTasksCreated},
			CreatedTasks: []listingkit.SheinStudioCreatedTask{{
				ID:       "task-1",
				DesignID: "design-1",
				Title:    "Style 1",
			}},
		},
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batches/:batch_id/tasks", h.CreateStudioBatchTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batches/batch-1/tasks", strings.NewReader(`{"design_ids":["design-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	if svc.createTasksBatchID != "batch-1" {
		t.Fatalf("create tasks batch id = %q, want batch-1", svc.createTasksBatchID)
	}
	if svc.createTasksReq == nil || len(svc.createTasksReq.DesignIDs) != 1 || svc.createTasksReq.DesignIDs[0] != "design-1" {
		t.Fatalf("create tasks req = %+v, want bound design id", svc.createTasksReq)
	}
}
