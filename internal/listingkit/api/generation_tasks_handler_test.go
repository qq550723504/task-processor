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

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/listingkit"
)

type stubGenerationTaskService struct {
	page                    *listingkit.GenerationTaskPage
	queue                   *listingkit.GenerationQueuePage
	action                  *listingkit.GenerationActionExecutionResult
	navigation              *listingkit.GenerationReviewNavigationDispatchResponse
	reviewSession           *listingkit.GenerationReviewSessionResponse
	reviewPreview           *listingkit.GenerationReviewPreviewResponse
	uploadResponse          *listingkit.UploadImagesResponse
	uploadedImageFile       *listingkit.UploadedImageFile
	deletedUploadedImage    *listingkit.DeletedUploadedImage
	studioDesigns           *listingkit.StudioDesignResponse
	studioProductImages     *listingkit.StudioProductImageResponse
	aiSettings              *listingkit.AIClientSettings
	storeProfiles           []listingkit.ListingKitStoreProfile
	upsertedStoreProfile    *listingkit.ListingKitStoreProfile
	storeRoutingSettings    *listingkit.ListingKitStoreRoutingSettings
	err                     error
	lastTask                string
	uploadedImageKey        string
	deletedUploadedImageKey string
	query                   *listingkit.GenerationTaskQuery
	queueQuery              *listingkit.GenerationQueueQuery
	retryReq                *listingkit.RetryGenerationTasksRequest
	childRetryReq           *listingkit.RetryChildTaskRequest
	childRetryResult        *listingkit.TaskResult
	actionReq               *listingkit.ExecuteGenerationActionRequest
	navigationReq           *listingkit.GenerationReviewNavigationDispatchRequest
	reviewSessionQuery      *listingkit.GenerationQueueQuery
	reviewPreviewQuery      *listingkit.GenerationQueueQuery
	uploadImagesReq         *listingkit.UploadImagesRequest
	studioDesignReq         *listingkit.StudioDesignRequest
	studioProductImageReq   *listingkit.StudioProductImageRequest
	aiSettingsReq           *listingkit.AIClientSettings
	upsertStoreProfileReq   *listingkit.ListingKitStoreProfile
	updateStoreRoutingReq   *listingkit.ListingKitStoreRoutingSettings
}

func (s *stubGenerationTaskService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}
func (s *stubGenerationTaskService) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) (*listingkit.TaskListPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskResult(ctx context.Context, taskID string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	s.lastTask = taskID
	s.query = query
	return s.page, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	s.lastTask = taskID
	s.queueQuery = query
	return s.queue, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	s.lastTask = taskID
	s.reviewSessionQuery = query
	return s.reviewSession, s.err
}

func (s *stubGenerationTaskService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	s.lastTask = taskID
	s.retryReq = req
	return s.page, s.err
}

func (s *stubGenerationTaskService) RetryTaskChildTask(ctx context.Context, taskID string, req *listingkit.RetryChildTaskRequest) (*listingkit.TaskResult, error) {
	s.lastTask = taskID
	s.childRetryReq = req
	return s.childRetryResult, s.err
}

func (s *stubGenerationTaskService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	s.lastTask = taskID
	s.actionReq = req
	return s.action, s.err
}

func (s *stubGenerationTaskService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	s.lastTask = taskID
	s.navigationReq = req
	return s.navigation, s.err
}

func (s *stubGenerationTaskService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	s.lastTask = taskID
	s.reviewPreviewQuery = query
	return s.reviewPreview, s.err
}

func (s *stubGenerationTaskService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) GetTaskExport(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) ApplyTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) ValidateTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubGenerationTaskService) SubmitTask(ctx context.Context, taskID string, req *listingkit.SubmitTaskRequest) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func TestGetTaskGenerationTasksReturnsPage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{
			TaskID: "task-1",
			Summary: &listingkit.AssetGenerationSummary{
				TotalTasks: 1,
			},
			Tasks: []assetgeneration.Task{{
				ID:            "amazon:amazon-lifestyle",
				Platform:      "amazon",
				RecipeID:      "amazon-lifestyle",
				AssetKind:     asset.KindSceneImage,
				ExecutionMode: assetgeneration.ExecutionModeRendererBacked,
			}},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-tasks", h.GetTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-tasks", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastTask != "task-1" {
		t.Fatalf("last task = %q, want task-1", svc.lastTask)
	}
	if svc.query == nil {
		t.Fatal("expected query binding")
	}
	var body listingkit.GenerationTaskPage
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Summary == nil || body.Summary.TotalTasks != 1 {
		t.Fatalf("body = %+v, want total_tasks=1", body)
	}
}

func TestGetTaskGenerationTasksBindsQueryFilters(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-tasks", h.GetTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-tasks?platform=shein&slot=main&execution_mode=renderer_backed&execution_status=completed&satisfied_by=generated_asset&page=2&page_size=10&sort_by=platform&sort_order=asc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.query == nil {
		t.Fatal("expected query")
	}
	if svc.query.Platform != "shein" || svc.query.Slot != "main" || svc.query.ExecutionMode != "renderer_backed" || svc.query.ExecutionStatus != "completed" || svc.query.SatisfiedBy != "generated_asset" {
		t.Fatalf("query = %+v, want all filters bound", svc.query)
	}
	if svc.query.Page != 2 || svc.query.PageSize != 10 || svc.query.SortBy != "platform" || svc.query.SortOrder != "asc" {
		t.Fatalf("query = %+v, want paging and sorting bound", svc.query)
	}
}

func TestGetTaskGenerationQueueReturnsNotModified(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{
			TaskID:      "task-1",
			DeltaToken:  "delta-queue-1",
			NotModified: true,
			Page:        1,
			PageSize:    20,
			Total:       3,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?platform=shein&delta_token=delta-queue-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.queueQuery == nil || svc.queueQuery.Platform != "shein" || svc.queueQuery.DeltaToken != "delta-queue-1" {
		t.Fatalf("queue query = %+v, want bound delta token query", svc.queueQuery)
	}
	if resp.Header().Get("ETag") != `"delta-queue-1"` {
		t.Fatalf("etag = %q, want quoted queue delta token", resp.Header().Get("ETag"))
	}
	var body listingkit.GenerationQueuePage
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.NotModified || body.DeltaToken != "delta-queue-1" || body.Summary != nil || len(body.Items) != 0 {
		t.Fatalf("body = %+v, want not_modified queue page", body)
	}
}

func TestGetTaskGenerationQueueUsesIfNoneMatchAndReturns304(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{
			TaskID:      "task-1",
			DeltaToken:  "delta-queue-2",
			NotModified: true,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?platform=shein", nil)
	req.Header.Set("If-None-Match", `"delta-queue-2"`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", resp.Code)
	}
	if svc.queueQuery == nil || svc.queueQuery.IfMatch != "delta-queue-2" {
		t.Fatalf("queue query = %+v, want If-None-Match bound into if_match", svc.queueQuery)
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty body for 304", resp.Body.String())
	}
}

func TestRetryTaskGenerationTasksReturnsBadRequestForNonRetryableTask(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		err: listingkit.ErrGenerationTaskNotRetryable,
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"task_ids":["amazon:amazon-lifestyle"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
	if svc.retryReq == nil || len(svc.retryReq.TaskIDs) != 1 {
		t.Fatalf("retry req = %+v, want one task id", svc.retryReq)
	}
}

func TestRetryTaskGenerationTasksBindsFallbackFilters(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"fallback_only":true,"slots":["main","gallery"]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.retryReq == nil || !svc.retryReq.FallbackOnly {
		t.Fatalf("retry req = %+v, want fallback_only", svc.retryReq)
	}
	if len(svc.retryReq.Slots) != 2 {
		t.Fatalf("retry req = %+v, want slot filters", svc.retryReq)
	}
}

func TestRetryTaskGenerationTasksBindsExecutionQualityFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"execution_quality":"renderer_output","renderer_only":true}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.retryReq == nil || svc.retryReq.ExecutionQuality != "renderer_output" || !svc.retryReq.RendererOnly {
		t.Fatalf("retry req = %+v, want execution_quality and renderer_only bound", svc.retryReq)
	}
}

func TestRetryTaskGenerationTasksBindsExecutionQualityLabelFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"execution_quality_label":"Renderer Output"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.retryReq == nil || svc.retryReq.ExecutionQualityLabel != "Renderer Output" {
		t.Fatalf("retry req = %+v, want execution_quality_label bound", svc.retryReq)
	}
}

func TestExecuteTaskGenerationActionBindsTarget(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		action: &listingkit.GenerationActionExecutionResult{
			ActionKey:       "generate_missing_assets",
			InteractionMode: "retryable",
			ReviewSession: &listingkit.GenerationReviewSession{
				SelectedPlatform: "shein",
				SelectedSlot:     "main",
				FocusCapability:  "detail_preview",
				DefaultTarget: &listingkit.GenerationReviewTarget{
					Platform:   "shein",
					Slot:       "main",
					Capability: "detail_preview",
					ActionKey:  "review_detail_previews",
					SectionKey: "detail_preview",
					FocusKey:   "shein:main:detail_preview",
					SessionQuery: &listingkit.GenerationQueueQuery{
						Platform:          "shein",
						Slot:              "main",
						PreviewCapability: "detail_preview",
						ResponseMode:      "patch_only",
					},
					NavigationTarget: &listingkit.GenerationReviewNavigationTarget{
						QueueQuery: &listingkit.GenerationQueueQuery{
							Platform:          "shein",
							Slot:              "main",
							PreviewCapability: "detail_preview",
						},
						SessionQuery: &listingkit.GenerationQueueQuery{
							Platform:          "shein",
							Slot:              "main",
							PreviewCapability: "detail_preview",
							ResponseMode:      "patch_only",
						},
						PreviewQuery: &listingkit.GenerationQueueQuery{
							Platform:          "shein",
							Slot:              "main",
							AssetID:           "asset-preview-1",
							PreviewCapability: "detail_preview",
						},
					},
					PanelState: &listingkit.GenerationReviewPanelState{
						SelectedPlatform:  "shein",
						SelectedSlot:      "main",
						FocusCapability:   "detail_preview",
						FocusedSectionKey: "detail_preview",
					},
					NavigationDelta: &listingkit.GenerationReviewNavigationDelta{},
				},
				FocusedToolbar: &listingkit.GenerationReviewToolbarInput{
					Platform:   "shein",
					Slot:       "main",
					Capability: "detail_preview",
					PreviewViewer: &listingkit.GenerationReviewPreviewViewer{
						Platform:      "shein",
						Slot:          "main",
						AssetID:       "asset-preview-1",
						PreviewFormat: "svg",
						NavigationTarget: &listingkit.GenerationReviewNavigationTarget{
							QueueQuery: &listingkit.GenerationQueueQuery{
								Platform:          "shein",
								Slot:              "main",
								PreviewCapability: "detail_preview",
							},
							SessionQuery: &listingkit.GenerationQueueQuery{
								Platform:          "shein",
								Slot:              "main",
								PreviewCapability: "detail_preview",
								ResponseMode:      "patch_only",
							},
							PreviewQuery: &listingkit.GenerationQueueQuery{
								Platform:          "shein",
								Slot:              "main",
								AssetID:           "asset-preview-1",
								PreviewCapability: "detail_preview",
							},
						},
						PreviewQuery: &listingkit.GenerationQueueQuery{
							Platform:          "shein",
							Slot:              "main",
							AssetID:           "asset-preview-1",
							PreviewCapability: "detail_preview",
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
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", h.ExecuteTaskGenerationAction)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-actions/execute", strings.NewReader(`{"action_key":"generate_missing_assets","target":{"action_key":"generate_missing_assets","interaction_mode":"retryable","queue_query":{"quality_grade":"missing"},"retry_request":{"quality_grade":"missing"}}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.actionReq == nil || svc.actionReq.Target == nil {
		t.Fatalf("action req = %+v, want target bound", svc.actionReq)
	}
	if svc.actionReq.ActionKey != "generate_missing_assets" || svc.actionReq.Target.InteractionMode != "retryable" {
		t.Fatalf("action req = %+v, want action key and interaction mode bound", svc.actionReq)
	}
	var body listingkit.GenerationActionExecutionResult
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.ReviewSession == nil || body.ReviewSession.DefaultTarget == nil {
		t.Fatalf("body = %+v, want review session target", body)
	}
	if body.ReviewSession.DefaultTarget.ActionKey != "review_detail_previews" || body.ReviewSession.DefaultTarget.FocusKey != "shein:main:detail_preview" {
		t.Fatalf("review session target = %+v, want review target metadata", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.DefaultTarget.SessionQuery == nil || body.ReviewSession.DefaultTarget.SessionQuery.ResponseMode != "patch_only" {
		t.Fatalf("review session target = %+v, want navigation session query", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.DefaultTarget.NavigationTarget == nil || body.ReviewSession.DefaultTarget.NavigationTarget.SessionQuery == nil || body.ReviewSession.DefaultTarget.NavigationTarget.PreviewQuery == nil {
		t.Fatalf("review session target = %+v, want unified navigation target", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.DefaultTarget.NavigationTarget.QueueQuery == nil || body.ReviewSession.DefaultTarget.NavigationTarget.QueueQuery.Platform != "shein" {
		t.Fatalf("review session target = %+v, want queue navigation target", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.DefaultTarget.PanelState == nil || body.ReviewSession.DefaultTarget.PanelState.SelectedPlatform != "shein" {
		t.Fatalf("review session target = %+v, want panel state", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.DefaultTarget.NavigationDelta == nil {
		t.Fatalf("review session target = %+v, want navigation delta", body.ReviewSession.DefaultTarget)
	}
	if body.ReviewSession.FocusedToolbar == nil || body.ReviewSession.FocusedToolbar.Capability != "detail_preview" {
		t.Fatalf("review session focused toolbar = %+v, want focused toolbar", body.ReviewSession.FocusedToolbar)
	}
	if body.ReviewSession.FocusedToolbar.PreviewViewer == nil || body.ReviewSession.FocusedToolbar.PreviewViewer.PreviewFormat != "svg" {
		t.Fatalf("review session focused toolbar = %+v, want preview viewer", body.ReviewSession.FocusedToolbar)
	}
	if body.ReviewSession.FocusedToolbar.PreviewViewer.PreviewQuery == nil || body.ReviewSession.FocusedToolbar.PreviewViewer.PreviewQuery.AssetID != "asset-preview-1" {
		t.Fatalf("review session focused toolbar = %+v, want preview query", body.ReviewSession.FocusedToolbar)
	}
	if body.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget == nil || body.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery == nil || body.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.PreviewQuery.AssetID != "asset-preview-1" {
		t.Fatalf("review session focused toolbar = %+v, want unified preview navigation target", body.ReviewSession.FocusedToolbar)
	}
	if body.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.QueueQuery == nil || body.ReviewSession.FocusedToolbar.PreviewViewer.NavigationTarget.QueueQuery.Platform != "shein" {
		t.Fatalf("review session focused toolbar = %+v, want queue navigation target", body.ReviewSession.FocusedToolbar)
	}
}

func TestExecuteTaskGenerationActionBindsPatchOnlyResponseMode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		action: &listingkit.GenerationActionExecutionResult{
			ActionKey:       "approve_section_review",
			InteractionMode: "review_only",
			ResponseMode:    "patch_only",
			DeltaToken:      "delta-1",
			ReviewPatch: &listingkit.GenerationReviewSessionPatch{
				DeltaToken:       "delta-1",
				SelectedPlatform: "shein",
				SelectedSlot:     "main",
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", h.ExecuteTaskGenerationAction)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-actions/execute", strings.NewReader(`{"action_key":"approve_section_review","response_mode":"patch_only"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.actionReq == nil || svc.actionReq.ResponseMode != "patch_only" {
		t.Fatalf("action req = %+v, want patch_only response mode bound", svc.actionReq)
	}
	var body listingkit.GenerationActionExecutionResult
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.ResponseMode != "patch_only" || body.DeltaToken != "delta-1" || body.ReviewPatch == nil || body.ReviewPatch.DeltaToken != "delta-1" {
		t.Fatalf("body = %+v, want patch-only action response with delta token", body)
	}
	if resp.Header().Get("ETag") != `"delta-1"` {
		t.Fatalf("etag = %q, want quoted action delta token", resp.Header().Get("ETag"))
	}
}

func TestExecuteTaskGenerationActionReturnsNotFoundForUnknownAction(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		err: listingkit.ErrGenerationActionNotFound,
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-actions/execute", h.ExecuteTaskGenerationAction)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-actions/execute", strings.NewReader(`{"action_key":"unknown_action"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestGetTaskGenerationReviewPreviewBindsQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		reviewPreview: &listingkit.GenerationReviewPreviewResponse{
			TaskID: "task-1",
			Viewer: &listingkit.GenerationReviewPreviewViewer{
				Platform:      "shein",
				Slot:          "main",
				AssetID:       "asset-preview-1",
				PreviewFormat: "svg",
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-review-preview", h.GetTaskGenerationReviewPreview)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-review-preview?platform=shein&slot=main&preview_capability=detail_preview", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.reviewPreviewQuery == nil || svc.reviewPreviewQuery.Platform != "shein" || svc.reviewPreviewQuery.Slot != "main" || svc.reviewPreviewQuery.PreviewCapability != "detail_preview" {
		t.Fatalf("review preview query = %+v, want bound review preview query", svc.reviewPreviewQuery)
	}
}

func TestGetTaskGenerationReviewPreviewUsesIfNoneMatchAndReturns304(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		reviewPreview: &listingkit.GenerationReviewPreviewResponse{
			TaskID:      "task-1",
			DeltaToken:  "delta-preview-1",
			NotModified: true,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-review-preview", h.GetTaskGenerationReviewPreview)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-review-preview?platform=shein&slot=main", nil)
	req.Header.Set("If-None-Match", `"delta-preview-1"`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", resp.Code)
	}
	if svc.reviewPreviewQuery == nil || svc.reviewPreviewQuery.IfMatch != "delta-preview-1" {
		t.Fatalf("review preview query = %+v, want If-None-Match bound", svc.reviewPreviewQuery)
	}
	if resp.Header().Get("ETag") != `"delta-preview-1"` {
		t.Fatalf("etag = %q, want quoted preview delta token", resp.Header().Get("ETag"))
	}
}

func TestGetTaskGenerationReviewSessionBindsQueryAndReturnsNotModified(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		reviewSession: &listingkit.GenerationReviewSessionResponse{
			TaskID:      "task-1",
			DeltaToken:  "delta-1",
			NotModified: true,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-review-session", h.GetTaskGenerationReviewSession)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-review-session?platform=shein&slot=main&delta_token=delta-1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.reviewSessionQuery == nil || svc.reviewSessionQuery.Platform != "shein" || svc.reviewSessionQuery.Slot != "main" || svc.reviewSessionQuery.DeltaToken != "delta-1" {
		t.Fatalf("review session query = %+v, want bound query", svc.reviewSessionQuery)
	}
	if resp.Header().Get("ETag") != `"delta-1"` {
		t.Fatalf("etag = %q, want quoted review session delta token", resp.Header().Get("ETag"))
	}
	var body listingkit.GenerationReviewSessionResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if !body.NotModified || body.DeltaToken != "delta-1" {
		t.Fatalf("body = %+v, want not_modified response", body)
	}
}

func TestGetTaskGenerationReviewSessionUsesIfNoneMatchAndReturns304(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		reviewSession: &listingkit.GenerationReviewSessionResponse{
			TaskID:      "task-1",
			DeltaToken:  "delta-session-1",
			NotModified: true,
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-review-session", h.GetTaskGenerationReviewSession)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-review-session?platform=shein&slot=main", nil)
	req.Header.Set("If-None-Match", `"delta-session-1"`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", resp.Code)
	}
	if svc.reviewSessionQuery == nil || svc.reviewSessionQuery.IfMatch != "delta-session-1" {
		t.Fatalf("review session query = %+v, want If-None-Match bound", svc.reviewSessionQuery)
	}
	if resp.Header().Get("ETag") != `"delta-session-1"` {
		t.Fatalf("etag = %q, want quoted review session delta token", resp.Header().Get("ETag"))
	}
}

func TestGetTaskGenerationReviewSessionBindsPatchOnlyNavigationQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		reviewSession: &listingkit.GenerationReviewSessionResponse{
			TaskID:       "task-1",
			DeltaToken:   "delta-2",
			ResponseMode: "patch_only",
			Patch: &listingkit.GenerationReviewSessionPatch{
				DeltaToken:       "delta-2",
				SelectedPlatform: "shein",
				SelectedSlot:     "gallery",
			},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-review-session", h.GetTaskGenerationReviewSession)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-review-session?platform=shein&slot=gallery&preview_capability=badge_preview&response_mode=patch_only&from_platform=shein&from_slot=main&from_capability=detail_preview", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.reviewSessionQuery == nil || svc.reviewSessionQuery.ResponseMode != "patch_only" || svc.reviewSessionQuery.FromSlot != "main" || svc.reviewSessionQuery.FromCapability != "detail_preview" {
		t.Fatalf("review session query = %+v, want patch_only navigation query bound", svc.reviewSessionQuery)
	}
	var body listingkit.GenerationReviewSessionResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.ResponseMode != "patch_only" || body.Patch == nil || body.Patch.SelectedSlot != "gallery" {
		t.Fatalf("body = %+v, want patch_only response body", body)
	}
}

func TestRetryTaskGenerationTasksBindsQualityGradeLabelFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"quality_grade_label":"Provisional"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.retryReq == nil || svc.retryReq.QualityGradeLabel != "Provisional" {
		t.Fatalf("retry req = %+v, want quality_grade_label bound", svc.retryReq)
	}
}

func TestRetryTaskGenerationTasksBindsQualityGradeFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		page: &listingkit.GenerationTaskPage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/generation-tasks/retry", h.RetryTaskGenerationTasks)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/generation-tasks/retry", strings.NewReader(`{"quality_grade":"provisional"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.retryReq == nil || svc.retryReq.QualityGrade != "provisional" {
		t.Fatalf("retry req = %+v, want quality_grade bound", svc.retryReq)
	}
}

func TestGetTaskGenerationQueueReturnsPage(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{
			TaskID: "task-1",
			Summary: &listingkit.GenerationWorkQueueSummary{
				TotalItems: 1,
			},
			Items: []listingkit.GenerationWorkQueueItem{{
				Platform: "amazon",
				Slot:     "main",
				State:    "ready",
			}},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.lastTask != "task-1" || svc.queueQuery == nil {
		t.Fatalf("last task/query = %q %+v, want task-1 and bound query", svc.lastTask, svc.queueQuery)
	}
}

func TestGetTaskGenerationQueueBindsQueryFilters(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?platform=shein&slot=main&state=stubbed&execution_mode=deferred_stub&retryable=true&page=2&page_size=5&sort_by=state&sort_order=asc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.queueQuery == nil {
		t.Fatal("expected queue query")
	}
	if svc.queueQuery.Platform != "shein" || svc.queueQuery.Slot != "main" || svc.queueQuery.State != "stubbed" || svc.queueQuery.ExecutionMode != "deferred_stub" || !svc.queueQuery.Retryable || svc.queueQuery.Page != 2 || svc.queueQuery.PageSize != 5 || svc.queueQuery.SortBy != "state" || svc.queueQuery.SortOrder != "asc" {
		t.Fatalf("queue query = %+v, want all filters bound", svc.queueQuery)
	}
}

func TestGetTaskGenerationQueueBindsExecutionQualityFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?execution_quality=renderer_output&sort_by=execution_quality&sort_order=desc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.queueQuery == nil || svc.queueQuery.ExecutionQuality != "renderer_output" || svc.queueQuery.SortBy != "execution_quality" || svc.queueQuery.SortOrder != "desc" {
		t.Fatalf("queue query = %+v, want execution_quality filter and sort bound", svc.queueQuery)
	}
}

func TestGetTaskGenerationQueueBindsQualityGradeFilter(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?quality_grade=provisional&quality_grade_label=Provisional&sort_by=quality_grade&sort_order=asc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.queueQuery == nil || svc.queueQuery.QualityGrade != "provisional" || svc.queueQuery.QualityGradeLabel != "Provisional" || svc.queueQuery.SortBy != "quality_grade" {
		t.Fatalf("queue query = %+v, want quality grade filters bound", svc.queueQuery)
	}
}

func TestGetTaskGenerationQueueBindsRenderPreviewFilters(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubGenerationTaskService{
		queue: &listingkit.GenerationQueuePage{TaskID: "task-1"},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.GET("/api/v1/listing-kits/tasks/:task_id/generation-queue", h.GetTaskGenerationQueue)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks/task-1/generation-queue?render_preview_available=true&preview_capability=detail_preview&sort_by=render_preview_available&sort_order=desc", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if svc.queueQuery == nil || !svc.queueQuery.RenderPreviewAvailable || !svc.queueQuery.RenderPreviewAvailablePresent || svc.queueQuery.PreviewCapability != "detail_preview" || svc.queueQuery.SortBy != "render_preview_available" {
		t.Fatalf("queue query = %+v, want render preview filters bound", svc.queueQuery)
	}
}
