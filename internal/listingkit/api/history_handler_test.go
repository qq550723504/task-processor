package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubHistoryService struct {
	page      *listingkit.ListingKitRevisionHistoryPage
	err       error
	lastTask  string
	lastQuery *listingkit.RevisionHistoryQuery
}

func (s *stubHistoryService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}
func (s *stubHistoryService) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) (*listingkit.TaskListPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) UploadImages(ctx context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetUploadedImage(ctx context.Context, key string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskResult(ctx context.Context, taskID string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	s.lastTask = taskID
	s.lastQuery = query
	return s.page, s.err
}

func (s *stubHistoryService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) GetTaskExport(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) ApplyTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) ValidateTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryService) SubmitTask(ctx context.Context, taskID string, req *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func TestGetTaskRevisionHistoryReturnsPage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	svc := &stubHistoryService{
		page: &listingkit.ListingKitRevisionHistoryPage{
			TaskID: "task-1",
			Items: []listingkit.ListingKitRevisionRecord{{
				Platform:   "shein",
				ActionType: listingkit.RevisionActionTypeEdit,
				UpdatedAt:  now,
				Timeline: &listingkit.ListingKitRevisionTimelineSummary{
					Headline: "更新 SHEIN 资料",
					Badge:    "编辑",
				},
			}},
			Meta: &listingkit.ListingKitRevisionHistoryPageMeta{
				TotalRecords:    5,
				ReturnedRecords: 1,
				HasMore:         true,
				NextBefore:      now.Format(time.RFC3339),
				Counts: &listingkit.ListingKitRevisionHistoryCounts{
					All:  5,
					Edit: 4,
				},
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history", h.GetTaskRevisionHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history?limit=5&before=2026-04-17T10:00:00Z&action_type=edit", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastTask != "task-1" || svc.lastQuery == nil || svc.lastQuery.Limit != 5 || svc.lastQuery.Before != "2026-04-17T10:00:00Z" || svc.lastQuery.ActionType != "edit" {
		t.Fatalf("service call = task %q query %+v", svc.lastTask, svc.lastQuery)
	}
	var body listingkit.ListingKitRevisionHistoryPage
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Meta == nil || body.Meta.NextBefore == "" {
		t.Fatalf("history page = %+v", body)
	}
	if body.Meta.Counts == nil || body.Meta.Counts.All != 5 {
		t.Fatalf("history page meta = %+v", body.Meta)
	}
	if len(body.Items) != 1 || body.Items[0].Timeline == nil {
		t.Fatalf("history page items = %+v", body.Items)
	}
}

func TestGetTaskRevisionHistoryReturnsBadRequestForInvalidCursor(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryService{err: listingkit.ErrInvalidRevisionHistoryCursor}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history", h.GetTaskRevisionHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history?before=bad-cursor", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestGetTaskRevisionHistoryReturnsBadRequestForInvalidActionType(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryService{err: listingkit.ErrInvalidRevisionHistoryActionType}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history", h.GetTaskRevisionHistory)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history?action_type=archive", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}
