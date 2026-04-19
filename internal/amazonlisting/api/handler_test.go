package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type mockAmazonListingHandlerSvc struct {
	createResult    *amazonlisting.Task
	createErr       error
	getResult       *amazonlisting.TaskResult
	getErr          error
	listQueueResult *amazonlisting.TaskQueueResult
	listQueueErr    error
	workbenchResult *amazonlisting.TaskWorkbench
	workbenchErr    error
	reviewResult    *amazonlisting.TaskResult
	reviewErr       error
	submitResult    *amazonlisting.TaskResult
	submitErr       error
}

func (m *mockAmazonListingHandlerSvc) CreateGenerateTask(_ context.Context, _ *amazonlisting.GenerateRequest) (*amazonlisting.Task, error) {
	return m.createResult, m.createErr
}

func (m *mockAmazonListingHandlerSvc) GetTaskResult(_ context.Context, _ string) (*amazonlisting.TaskResult, error) {
	return m.getResult, m.getErr
}

func (m *mockAmazonListingHandlerSvc) ListTaskQueue(_ context.Context, _ amazonlisting.TaskQueueQuery) (*amazonlisting.TaskQueueResult, error) {
	return m.listQueueResult, m.listQueueErr
}

func (m *mockAmazonListingHandlerSvc) GetTaskWorkbench(_ context.Context, _ string) (*amazonlisting.TaskWorkbench, error) {
	return m.workbenchResult, m.workbenchErr
}

func (m *mockAmazonListingHandlerSvc) ReviewTask(_ context.Context, _ string, _ *amazonlisting.ReviewTaskRequest) (*amazonlisting.TaskResult, error) {
	return m.reviewResult, m.reviewErr
}

func (m *mockAmazonListingHandlerSvc) SubmitTask(_ context.Context, _ string, _ *amazonlisting.SubmitTaskRequest) (*amazonlisting.TaskResult, error) {
	return m.submitResult, m.submitErr
}

func newAmazonListingTestRouter(svc amazonlisting.HandlerService) *gin.Engine {
	h, _ := NewHandler(svc)
	r := gin.New()
	r.POST("/listings/generate", h.GenerateListing)
	r.GET("/listings/tasks", h.ListTaskQueue)
	r.GET("/listings/tasks/:task_id", h.GetTaskResult)
	r.GET("/listings/tasks/:task_id/workbench", h.GetTaskWorkbench)
	r.POST("/listings/tasks/:task_id/review", h.ReviewTask)
	r.POST("/listings/tasks/:task_id/submit", h.SubmitTask)
	return r
}

func TestReviewTask_ApplyEdits_Returns200(t *testing.T) {
	now := time.Now()
	svc := &mockAmazonListingHandlerSvc{
		reviewResult: &amazonlisting.TaskResult{
			TaskID:    "listing-1",
			Status:    amazonlisting.TaskStatusNeedsReview,
			CreatedAt: now,
			Result: &amazonlisting.AmazonListingDraft{
				TaskID: "listing-1",
				Brand:  "Acme",
			},
		},
	}
	r := newAmazonListingTestRouter(svc)

	body := `{
		"action":"apply_edits",
		"edits":[
			{"field":"brand","string_value":"Acme"},
			{"field":"title","string_value":"High Quality Ceramic Mug for Daily Home Kitchen Use"}
		]
	}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/listings/tasks/listing-1/review", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp["task_id"] != "listing-1" {
		t.Fatalf("task_id = %v, want listing-1", resp["task_id"])
	}
}

func TestReviewTask_UnsupportedAction_Returns400(t *testing.T) {
	svc := &mockAmazonListingHandlerSvc{
		reviewErr: errors.New("unsupported review action: noop"),
	}
	r := newAmazonListingTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/listings/tasks/listing-1/review", bytes.NewBufferString(`{"action":"noop"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetTaskWorkbench_ReturnsStructuredReviewItems(t *testing.T) {
	svc := &mockAmazonListingHandlerSvc{
		workbenchResult: &amazonlisting.TaskWorkbench{
			TaskID:      "listing-2",
			Status:      amazonlisting.TaskStatusNeedsReview,
			NeedsReview: true,
			ReviewItems: []amazonlisting.AmazonReviewItem{
				{
					Field:          "brand",
					Action:         amazonlisting.OperatorActionFillBrand,
					Reason:         "missing brand",
					RecommendedFix: "confirm or fill the selling brand",
					Confidence:     0.58,
					IsInferred:     true,
					Evidence: []amazonlisting.AmazonReviewEvidence{
						{Type: "user_text", Detail: `user input: "portable blender"`},
						{Type: "field_value", Detail: `brand = "Generic"`},
					},
				},
			},
		},
	}
	r := newAmazonListingTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/listings/tasks/listing-2/workbench", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		TaskID      string                           `json:"task_id"`
		ReviewItems []amazonlisting.AmazonReviewItem `json:"review_items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.TaskID != "listing-2" {
		t.Fatalf("task_id = %q, want listing-2", resp.TaskID)
	}
	if len(resp.ReviewItems) != 1 {
		t.Fatalf("len(review_items) = %d, want 1", len(resp.ReviewItems))
	}
	if resp.ReviewItems[0].Confidence != 0.58 || !resp.ReviewItems[0].IsInferred {
		t.Fatalf("unexpected review item trace fields: %+v", resp.ReviewItems[0])
	}
	if len(resp.ReviewItems[0].Evidence) != 2 || resp.ReviewItems[0].Evidence[0].Type != "user_text" {
		t.Fatalf("unexpected review item evidence: %+v", resp.ReviewItems[0].Evidence)
	}
}

func TestListTaskQueue_ReturnsFilteredItems(t *testing.T) {
	needsHuman := true
	svc := &mockAmazonListingHandlerSvc{
		listQueueResult: &amazonlisting.TaskQueueResult{
			Count: 1,
			Query: amazonlisting.TaskQueueQuery{
				Status:     []amazonlisting.TaskStatus{amazonlisting.TaskStatusNeedsReview},
				Action:     amazonlisting.OperatorActionFillBrand,
				NeedsHuman: &needsHuman,
				Limit:      20,
			},
			Items: []amazonlisting.TaskWorkbench{
				{
					TaskID:      "listing-queue-1",
					Status:      amazonlisting.TaskStatusNeedsReview,
					NeedsReview: true,
					ReviewItems: []amazonlisting.AmazonReviewItem{
						{Field: "brand", Action: amazonlisting.OperatorActionFillBrand, NeedsHuman: true},
					},
				},
			},
		},
	}
	r := newAmazonListingTestRouter(svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/listings/tasks?status=needs_review&action=fill_brand&needs_human=true&limit=20", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp amazonlisting.TaskQueueResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Count != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected queue response: %+v", resp)
	}
	if resp.Items[0].TaskID != "listing-queue-1" {
		t.Fatalf("task_id = %q, want listing-queue-1", resp.Items[0].TaskID)
	}
}
