package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubStudioBatchActionService struct {
	getBatchDetailCtx     context.Context
	getBatchDetailBatchID string
	getBatchDetailResult  *listingkit.StudioBatchDetail
	getBatchDetailErr     error
	prepareGenerateCtx    context.Context
	prepareGenerateBatchID string
	prepareGenerateResult *listingkit.StudioBatchDetail
	prepareGenerateErr    error
	prepareRetryCtx       context.Context
	prepareRetryBatchID   string
	prepareRetryReq       *listingkit.RetryStudioBatchItemsRequest
	prepareRetryResult    *listingkit.StudioBatchDetail
	prepareRetryErr       error
	resumeCtx            context.Context
	resumeBatchID        string
	resumeResult         *listingkit.StudioBatchDetail
	resumeErr            error
	resumeCalled         chan struct{}
	resumeBlock          chan struct{}
	resumeCalls          int
	startCtx              context.Context
	startBatchID          string
	startResult           *listingkit.StudioBatchDetail
	startErr              error
	retryCtx              context.Context
	retryBatchID          string
	retryReq              *listingkit.RetryStudioBatchItemsRequest
	retryResult           *listingkit.StudioBatchDetail
	retryErr              error
	approveCtx            context.Context
	approveBatchID        string
	approveReq            *listingkit.ApproveStudioBatchDesignsRequest
	approveResult         *listingkit.StudioBatchDetail
	approveErr            error
	createTasksCtx        context.Context
	createTasksBatchID    string
	createTasksReq        *listingkit.CreateStudioBatchTasksRequest
	createTasksResult     *listingkit.CreateStudioBatchTasksResult
	createTasksErr        error
}

func (s *stubStudioBatchActionService) ListStudioSessionGallery(context.Context, int) (*listingkit.StudioSessionGalleryResponse, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) ListStudioBatches(context.Context, int) (*listingkit.StudioBatchListResponse, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) GetStudioBatch(context.Context, string) (*listingkit.StudioBatchDraftDetail, error) {
	return nil, nil
}

func (s *stubStudioBatchActionService) GetStudioBatchDetail(ctx context.Context, batchID string) (*listingkit.StudioBatchDetail, error) {
	s.getBatchDetailCtx = ctx
	s.getBatchDetailBatchID = batchID
	return s.getBatchDetailResult, s.getBatchDetailErr
}

func (s *stubStudioBatchActionService) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*listingkit.StudioBatchDetail, error) {
	s.prepareGenerateCtx = ctx
	s.prepareGenerateBatchID = batchID
	return s.prepareGenerateResult, s.prepareGenerateErr
}

func (s *stubStudioBatchActionService) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *listingkit.RetryStudioBatchItemsRequest) (*listingkit.StudioBatchDetail, error) {
	s.prepareRetryCtx = ctx
	s.prepareRetryBatchID = batchID
	s.prepareRetryReq = req
	return s.prepareRetryResult, s.prepareRetryErr
}

func (s *stubStudioBatchActionService) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*listingkit.StudioBatchDetail, error) {
	s.resumeCtx = ctx
	s.resumeBatchID = batchID
	s.resumeCalls++
	if s.resumeCalled != nil {
		select {
		case <-s.resumeCalled:
		default:
			close(s.resumeCalled)
		}
	}
	if s.resumeBlock != nil {
		<-s.resumeBlock
	}
	return s.resumeResult, s.resumeErr
}

func (s *stubStudioBatchActionService) UpsertStudioBatch(context.Context, *listingkit.UpsertStudioBatchRequest) (*listingkit.StudioBatchDraftDetail, error) {
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
		prepareGenerateResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
			Items: []listingkit.StudioBatchItemDetail{{
				Item: listingkit.StudioBatchItemRecord{ID: "item-1"},
			}},
		},
		resumeCalled: make(chan struct{}),
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
	if svc.prepareGenerateBatchID != "batch-1" {
		t.Fatalf("prepare generate batch id = %q, want batch-1", svc.prepareGenerateBatchID)
	}
	select {
	case <-svc.resumeCalled:
	case <-time.After(time.Second):
		t.Fatal("background resume was not launched for generate")
	}
	if !strings.Contains(rec.Body.String(), "\"items\"") {
		t.Fatalf("body = %s, want itemized detail payload", rec.Body.String())
	}
}

func TestStudioBatchGetHandlerReturnsItemizedDetail(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		getBatchDetailResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
			Items: []listingkit.StudioBatchItemDetail{{
				Item: listingkit.StudioBatchItemRecord{ID: "item-1"},
			}},
		},
		resumeCalled: make(chan struct{}),
	}
	h := &studioSessionHandler{service: svc}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/batches/:batch_id", h.GetStudioBatch)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batches/batch-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	select {
	case <-svc.resumeCalled:
	case <-time.After(time.Second):
		t.Fatal("resume flow was not triggered")
	}
	if svc.resumeBatchID != "batch-1" {
		t.Fatalf("resume batch id = %q, want batch-1", svc.resumeBatchID)
	}
	if svc.getBatchDetailBatchID != "batch-1" {
		t.Fatalf("detail batch id = %q, want handler to return current detail", svc.getBatchDetailBatchID)
	}
	if !strings.Contains(rec.Body.String(), "\"batch\"") || !strings.Contains(rec.Body.String(), "\"items\"") {
		t.Fatalf("body = %s, want itemized batch detail payload", rec.Body.String())
	}
}

func TestStudioBatchGetHandlerCoalescesConcurrentResumeLaunches(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		getBatchDetailResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
		},
		resumeCalled: make(chan struct{}),
		resumeBlock:  make(chan struct{}),
	}
	h := &studioSessionHandler{
		service:          svc,
		resumeDispatcher: newStudioBatchResumeDispatcher(),
	}
	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/batches/:batch_id", h.GetStudioBatch)

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batches/batch-1", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	select {
	case <-svc.resumeCalled:
	case <-time.After(time.Second):
		t.Fatal("first resume launch was not triggered")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batches/batch-1", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("statuses = %d/%d, want 200/200", rec1.Code, rec2.Code)
	}
	if svc.resumeCalls != 1 {
		t.Fatalf("resume call count = %d, want 1 coalesced launch", svc.resumeCalls)
	}

	close(svc.resumeBlock)
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
		prepareRetryResult: &listingkit.StudioBatchDetail{
			Batch: &listingkit.StudioBatchRecord{ID: "batch-1"},
		},
		resumeCalled: make(chan struct{}),
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
	if svc.prepareRetryBatchID != "batch-1" {
		t.Fatalf("prepare retry batch id = %q, want batch-1", svc.prepareRetryBatchID)
	}
	if svc.prepareRetryReq == nil || len(svc.prepareRetryReq.ItemIDs) != 1 || svc.prepareRetryReq.ItemIDs[0] != "item-1" {
		t.Fatalf("prepare retry req = %+v, want bound item id", svc.prepareRetryReq)
	}
	select {
	case <-svc.resumeCalled:
	case <-time.After(time.Second):
		t.Fatal("background resume was not launched for retry")
	}
}

func TestStudioBatchRetryItemsHandlerReturnsBadRequestForValidationErrors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchActionService{
		prepareRetryErr: listingkit.NewStudioBatchActionValidationError("item item-1 is not retryable"),
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
	if !errors.Is(svc.prepareRetryErr, listingkit.ErrStudioBatchActionValidation) {
		t.Fatalf("prepare retry err = %v, want validation sentinel", svc.prepareRetryErr)
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
