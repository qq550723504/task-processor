package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type stubStudioBatchRunService struct {
	run          *listingkit.StudioBatchRunRecord
	items        []listingkit.StudioBatchRunItemRecord
	createErr    error
	getErr       error
	listItemsErr error
	cancelErr    error
	recoverErr   error
	createCtx    context.Context
	createReq    *listingkit.CreateStudioBatchRunRequest
	getCtx       context.Context
	getRunID     string
	listItemsCtx context.Context
	listItemsID  string
	cancelCtx    context.Context
	cancelRunID  string
	recoverCtx   context.Context
	recoverRunID string
}

func (s *stubStudioBatchRunService) CreateStudioBatchRun(ctx context.Context, req *listingkit.CreateStudioBatchRunRequest) (*listingkit.StudioBatchRunRecord, []listingkit.StudioBatchRunItemRecord, error) {
	s.createCtx = ctx
	s.createReq = req
	return s.run, s.items, s.createErr
}

func (s *stubStudioBatchRunService) GetStudioBatchRun(ctx context.Context, runID string) (*listingkit.StudioBatchRunRecord, error) {
	s.getCtx = ctx
	s.getRunID = runID
	return s.run, s.getErr
}

func (s *stubStudioBatchRunService) ListStudioBatchRunItems(ctx context.Context, runID string) ([]listingkit.StudioBatchRunItemRecord, error) {
	s.listItemsCtx = ctx
	s.listItemsID = runID
	return s.items, s.listItemsErr
}

func (s *stubStudioBatchRunService) CancelStudioBatchRun(ctx context.Context, runID string) error {
	s.cancelCtx = ctx
	s.cancelRunID = runID
	return s.cancelErr
}

func (s *stubStudioBatchRunService) RecoverStudioBatchRun(ctx context.Context, runID string) error {
	s.recoverCtx = ctx
	s.recoverRunID = runID
	return s.recoverErr
}

func TestCreateStudioBatchRunReturnsAcceptedRunPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{
		run: &listingkit.StudioBatchRunRecord{
			ID:           "run-1",
			Status:       listingkit.StudioBatchRunStatusPending,
			TotalBatches: 2,
		},
		items: []listingkit.StudioBatchRunItemRecord{
			{ID: "run-1:1", RunID: "run-1", BatchID: "batch-1", Position: 1, Status: listingkit.StudioBatchRunItemStatusPending},
			{ID: "run-1:2", RunID: "run-1", BatchID: "batch-2", Position: 2, Status: listingkit.StudioBatchRunItemStatusPending},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs", h.CreateStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs", strings.NewReader(`{"batch_ids":["batch-1","batch-2"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "tenant-red")
	req.Header.Set("X-User-ID", "user-red")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}
	if svc.createReq == nil || len(svc.createReq.BatchIDs) != 2 || svc.createReq.BatchIDs[0] != "batch-1" || svc.createReq.BatchIDs[1] != "batch-2" {
		t.Fatalf("create req = %+v, want ordered batch ids", svc.createReq)
	}
	if got := listingkit.TenantIDFromContext(svc.createCtx); got != "tenant-red" {
		t.Fatalf("tenant id = %q, want tenant-red", got)
	}
	if got := listingkit.RequestUserIDFromContext(svc.createCtx); got != "user-red" {
		t.Fatalf("user id = %q, want user-red", got)
	}

	var body struct {
		Run   *listingkit.StudioBatchRunRecord      `json:"run"`
		Items []listingkit.StudioBatchRunItemRecord `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Run == nil || body.Run.ID != "run-1" {
		t.Fatalf("body.run = %+v, want run-1", body.Run)
	}
	if len(body.Items) != 2 || body.Items[1].BatchID != "batch-2" {
		t.Fatalf("body.items = %+v, want ordered items", body.Items)
	}
}

func TestGetStudioBatchRunReturnsRunPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{
		run: &listingkit.StudioBatchRunRecord{
			ID:           "run-9",
			Status:       listingkit.StudioBatchRunStatusRunning,
			TotalBatches: 3,
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/batch-runs/:run_id", h.GetStudioBatchRun)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batch-runs/run-9", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	if svc.getRunID != "run-9" {
		t.Fatalf("get run id = %q, want run-9", svc.getRunID)
	}
}

func TestListStudioBatchRunItemsReturnsItemsPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{
		items: []listingkit.StudioBatchRunItemRecord{
			{ID: "run-5:1", RunID: "run-5", BatchID: "batch-1", Position: 1, Status: listingkit.StudioBatchRunItemStatusSucceeded},
			{ID: "run-5:2", RunID: "run-5", BatchID: "batch-2", Position: 2, Status: listingkit.StudioBatchRunItemStatusPending},
		},
	}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/studio/batch-runs/:run_id/items", h.ListStudioBatchRunItems)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batch-runs/run-5/items", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", resp.Code, resp.Body.String())
	}
	if svc.listItemsID != "run-5" {
		t.Fatalf("list items run id = %q, want run-5", svc.listItemsID)
	}
}

func TestCancelStudioBatchRunReturnsAccepted(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs/:run_id/cancel", h.CancelStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs/run-cancel/cancel", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}
	if svc.cancelRunID != "run-cancel" {
		t.Fatalf("cancel run id = %q, want run-cancel", svc.cancelRunID)
	}
}

func TestRecoverStudioBatchRunReturnsAccepted(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubStudioBatchRunService{}
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs/:run_id/recover", h.RecoverStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs/run-recover/recover", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%s", resp.Code, resp.Body.String())
	}
	if svc.recoverRunID != "run-recover" {
		t.Fatalf("recover run id = %q, want run-recover", svc.recoverRunID)
	}
}

func TestCreateStudioBatchRunReturnsNotImplementedWithoutService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs", h.CreateStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs", strings.NewReader(`{"batch_ids":["batch-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 body=%s", resp.Code, resp.Body.String())
	}
}

func TestCreateStudioBatchRunReturnsBadRequestForInvalidJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(&stubStudioBatchRunService{}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/studio/batch-runs", h.CreateStudioBatchRun)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs", strings.NewReader(`{"batch_ids":[`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", resp.Code, resp.Body.String())
	}
}

func TestCreateStudioBatchRunClassifiesValidationNotFoundAndInternalErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "validation", serviceErr: errors.New("batch_ids is required"), wantStatus: http.StatusBadRequest},
		{name: "duplicate", serviceErr: errors.New("duplicate batch_id: batch-1"), wantStatus: http.StatusBadRequest},
		{name: "missing batch", serviceErr: listingkit.ErrStudioSessionNotFound, wantStatus: http.StatusNotFound},
		{name: "repo not found", serviceErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", serviceErr: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			svc := &stubStudioBatchRunService{createErr: tc.serviceErr}
			h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			router := gin.New()
			router.POST("/api/v1/listing-kits/studio/batch-runs", h.CreateStudioBatchRun)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs", strings.NewReader(`{"batch_ids":["batch-1"]}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tc.wantStatus, resp.Body.String())
			}
		})
	}
}

func TestGetStudioBatchRunReturnsNotFoundAndInternalErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "not found", serviceErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", serviceErr: errors.New("query failed"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			svc := &stubStudioBatchRunService{getErr: tc.serviceErr}
			h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			router := gin.New()
			router.GET("/api/v1/listing-kits/studio/batch-runs/:run_id", h.GetStudioBatchRun)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batch-runs/run-9", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tc.wantStatus, resp.Body.String())
			}
		})
	}
}

func TestListStudioBatchRunItemsReturnsNotFoundAndInternalErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "not found", serviceErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", serviceErr: errors.New("list failed"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			svc := &stubStudioBatchRunService{listItemsErr: tc.serviceErr}
			h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			router := gin.New()
			router.GET("/api/v1/listing-kits/studio/batch-runs/:run_id/items", h.ListStudioBatchRunItems)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/studio/batch-runs/run-5/items", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tc.wantStatus, resp.Body.String())
			}
		})
	}
}

func TestCancelStudioBatchRunReturnsNotFoundAndInternalErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "not found", serviceErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", serviceErr: errors.New("cancel failed"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			svc := &stubStudioBatchRunService{cancelErr: tc.serviceErr}
			h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			router := gin.New()
			router.POST("/api/v1/listing-kits/studio/batch-runs/:run_id/cancel", h.CancelStudioBatchRun)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs/run-cancel/cancel", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tc.wantStatus, resp.Body.String())
			}
		})
	}
}

func TestRecoverStudioBatchRunReturnsBadRequestNotFoundAndInternalErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		serviceErr error
		wantStatus int
	}{
		{name: "validation", serviceErr: listingkit.NewStudioBatchActionValidationError("run cannot be recovered from status running"), wantStatus: http.StatusBadRequest},
		{name: "not found", serviceErr: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "internal", serviceErr: errors.New("recover failed"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			svc := &stubStudioBatchRunService{recoverErr: tc.serviceErr}
			h, err := NewHandler(&stubHandlerCoreService{}, WithStudioBatchRunService(svc))
			if err != nil {
				t.Fatalf("new handler: %v", err)
			}

			router := gin.New()
			router.POST("/api/v1/listing-kits/studio/batch-runs/:run_id/recover", h.RecoverStudioBatchRun)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/studio/batch-runs/run-recover/recover", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tc.wantStatus, resp.Body.String())
			}
		})
	}
}
