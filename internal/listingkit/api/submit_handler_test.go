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

	"task-processor/internal/listingkit"
)

type stubSubmitService struct {
	preview *listingkit.ListingKitPreview
	err     error
}

func (s *stubSubmitService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) (*listingkit.TaskListPage, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) UploadImages(ctx context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetUploadedImage(ctx context.Context, key string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskResult(ctx context.Context, taskID string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) GetTaskExport(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) ApplyTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) ValidateTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}
func (s *stubSubmitService) SubmitTask(ctx context.Context, taskID string, req *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	return s.preview, s.err
}

func TestSubmitTaskReturnsBadRequestWhenBlocked(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubSubmitService{err: listingkit.ErrSubmitBlocked}
	h, err := NewHandler(svc)
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
	h, err := NewHandler(svc)
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
