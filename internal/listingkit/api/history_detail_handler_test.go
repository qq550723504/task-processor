package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

type stubHistoryDetailService struct {
	detail         *listingkit.ListingKitRevisionHistoryDetail
	err            error
	lastTaskID     string
	lastRevisionID string
}

func (s *stubHistoryDetailService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}
func (s *stubHistoryDetailService) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) (*listingkit.TaskListPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) UploadImages(ctx context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetUploadedImage(ctx context.Context, key string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskResult(ctx context.Context, taskID string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	s.lastTaskID = taskID
	s.lastRevisionID = revisionID
	return s.detail, s.err
}

func (s *stubHistoryDetailService) GetTaskExport(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) ApplyTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) ValidateTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubHistoryDetailService) SubmitTask(ctx context.Context, taskID string, req *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func TestGetTaskRevisionHistoryDetailReturnsRecord(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryDetailService{
		detail: &listingkit.ListingKitRevisionHistoryDetail{
			TaskID: "task-1",
			Navigation: &listingkit.RevisionHistoryNavigation{
				PrevRevisionID: "rev-0",
				NextRevisionID: "rev-2",
			},
			Record: &listingkit.ListingKitRevisionRecord{
				RevisionID: "rev-1",
				Platform:   "shein",
				ActionType: listingkit.RevisionActionTypeRestore,
				Timeline: &listingkit.ListingKitRevisionTimelineSummary{
					Headline: "恢复历史版本",
					Badge:    "回滚",
				},
			},
			RestorePayload: &listingkit.RevisionRestorePreviewPayload{
				Core: &listingkit.RevisionRestorePreviewCoreData{
					Draft: &listingkit.SheinEditorRevisionSkeleton{
						Platform: "shein",
						Reason:   "restore: manual adjustment",
					},
					RevisionPayload: &listingkit.ApplyRevisionRequest{
						Platform:              "shein",
						Actor:                 "desktop-client",
						Reason:                "restore: manual adjustment",
						RestoreFromRevisionID: "rev-1",
					},
					Context: &listingkit.RevisionHistoryRestoreContext{
						SourceRevisionID: "rev-1",
						SourceActionType: listingkit.RevisionActionTypeRestore,
						SourceHeadline:   "恢复历史版本",
						TargetRevisionID: "rev-0",
						TargetLabel:      "上一条",
						CompareMode:      "prev",
						ExecutionMode:    "restore_from_revision_id",
						RestoreReason:    "restore: manual adjustment",
						RestorePlatform:  "shein",
					},
					Safety: &listingkit.RevisionHistoryRestoreSafety{
						CanRestore:      true,
						RestoreWarnings: []string{"当前版本仍有人工备注待处理，恢复后建议再核对这些备注是否仍然适用"},
					},
					Compare: &listingkit.RevisionHistoryComparePreview{
						CompareTo:         "prev",
						CompareRevisionID: "rev-0",
						RelationLabel:     "上一条",
						DiffPreview: &listingkit.RevisionDiffPreview{
							ChangeCount: 1,
						},
					},
				},
				Presentation: &listingkit.RevisionInteractionPresentation{
					Scene:       "restore_preview",
					NextActions: []string{"确认后执行恢复"},
					Messages: &listingkit.RevisionResultMessages{
						Title:            "恢复历史版本",
						Description:      "将从 rev-1 恢复，并与上一条进行对齐。",
						SuccessLabel:     "恢复历史版本",
						WarningTitle:     "恢复前建议先确认以下事项",
						WarningSummaries: []string{"当前版本仍有人工备注待处理，恢复后建议再核对这些备注是否仍然适用"},
					},
					RecommendedView: &listingkit.RevisionRecommendedView{
						View:   "inspection",
						Reason: "恢复前还有提醒项，建议先确认影响范围。",
					},
					SummaryCard: &listingkit.RevisionSuccessSummaryCard{
						Status:        "ready_with_warnings",
						Title:         "恢复历史版本",
						Subtitle:      "可以恢复，但建议先确认潜在影响",
						PrimaryAction: "恢复历史版本",
						Highlights:    []string{"恢复自 rev-0"},
					},
				},
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", h.GetTaskRevisionHistoryDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history/rev-1?compare_to=prev", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastTaskID != "task-1" || svc.lastRevisionID != "rev-1" {
		t.Fatalf("service call = task %q revision %q", svc.lastTaskID, svc.lastRevisionID)
	}
	var body listingkit.ListingKitRevisionHistoryDetail
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Record == nil || body.Record.RevisionID != "rev-1" {
		t.Fatalf("detail = %+v", body)
	}
	if body.Navigation == nil || body.Navigation.PrevRevisionID != "rev-0" || body.Navigation.NextRevisionID != "rev-2" {
		t.Fatalf("navigation = %+v", body.Navigation)
	}
	if body.RestorePayload == nil || body.RestorePayload.Core == nil || body.RestorePayload.Core.Compare == nil || body.RestorePayload.Core.Compare.CompareRevisionID != "rev-0" {
		t.Fatalf("compare preview = %+v", body.RestorePayload)
	}
	if body.Record.Timeline == nil || body.Record.Timeline.Badge != "回滚" {
		t.Fatalf("timeline = %+v", body.Record)
	}
	if body.RestorePayload == nil || body.RestorePayload.Core == nil || body.RestorePayload.Core.Draft == nil || body.RestorePayload.Core.Draft.Platform != "shein" {
		t.Fatalf("restore payload = %+v", body.RestorePayload)
	}
	if body.RestorePayload.Core.RevisionPayload == nil || body.RestorePayload.Core.RevisionPayload.RestoreFromRevisionID != "rev-1" {
		t.Fatalf("restore payload = %+v", body.RestorePayload)
	}
	if body.RestorePayload.Core.Context == nil || body.RestorePayload.Core.Context.TargetRevisionID != "rev-0" {
		t.Fatalf("restore context = %+v", body.RestorePayload)
	}
	if body.RestorePayload.Presentation == nil || body.RestorePayload.Presentation.Scene != "restore_preview" || body.RestorePayload.Presentation.Messages == nil || body.RestorePayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("restore messages = %+v", body.RestorePayload)
	}
	if body.RestorePayload.Core.Safety == nil || !body.RestorePayload.Core.Safety.CanRestore {
		t.Fatalf("restore safety = %+v", body.RestorePayload)
	}
	if body.RestorePayload.Presentation == nil || body.RestorePayload.Presentation.SummaryCard == nil || body.RestorePayload.Presentation.SummaryCard.Status != "ready_with_warnings" {
		t.Fatalf("restore overview = %+v", body.RestorePayload)
	}
	if len(body.RestorePayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore overview = %+v", body.RestorePayload)
	}
}

func TestGetTaskRevisionHistoryDetailReturnsNotFound(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryDetailService{err: listingkit.ErrRevisionHistoryRecordNotFound}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", h.GetTaskRevisionHistoryDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history/rev-missing", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestGetTaskRevisionHistoryDetailReturnsNotFoundForMissingCompareTarget(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryDetailService{err: listingkit.ErrRevisionHistoryCompareTargetNotFound}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", h.GetTaskRevisionHistoryDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history/rev-1?compare_to=next", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestGetTaskRevisionHistoryDetailReturnsComparePreviewForCurrent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubHistoryDetailService{
		detail: &listingkit.ListingKitRevisionHistoryDetail{
			TaskID: "task-1",
			Record: &listingkit.ListingKitRevisionRecord{
				RevisionID: "rev-1",
				Platform:   "shein",
			},
			RestorePayload: &listingkit.RevisionRestorePreviewPayload{
				Core: &listingkit.RevisionRestorePreviewCoreData{
					Compare: &listingkit.RevisionHistoryComparePreview{
						CompareTo:         "current",
						CompareRevisionID: "current",
						RelationLabel:     "当前版本",
						DiffPreview: &listingkit.RevisionDiffPreview{
							ChangeCount: 2,
						},
					},
				},
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/revision-history/:revision_id", h.GetTaskRevisionHistoryDetail)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/revision-history/rev-1?compare_to=current", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	var body listingkit.ListingKitRevisionHistoryDetail
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.RestorePayload == nil || body.RestorePayload.Core == nil || body.RestorePayload.Core.Compare == nil || body.RestorePayload.Core.Compare.CompareRevisionID != "current" {
		t.Fatalf("compare preview = %+v", body.RestorePayload)
	}
}
