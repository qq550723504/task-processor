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

type stubRevisionService struct {
	applyPreview *listingkit.ListingKitPreview
	applyErr     error
}

func (s *stubRevisionService) CreateGenerateTask(ctx context.Context, req *listingkit.GenerateRequest) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetTaskResult(ctx context.Context, taskID string) (*listingkit.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetTaskPreview(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitPreview, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *listingkit.RevisionHistoryQuery) (*listingkit.ListingKitRevisionHistoryPage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *listingkit.RevisionHistoryDetailQuery) (*listingkit.ListingKitRevisionHistoryDetail, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) GetTaskExport(ctx context.Context, taskID string, platform string) (*listingkit.ListingKitExport, error) {
	return nil, errors.New("not implemented")
}

func (s *stubRevisionService) ApplyTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.ListingKitPreview, error) {
	return s.applyPreview, s.applyErr
}

func (s *stubRevisionService) ValidateTaskRevision(ctx context.Context, taskID string, req *listingkit.ApplyRevisionRequest) (*listingkit.RevisionValidationResult, error) {
	return nil, errors.New("not implemented")
}

func TestApplyTaskRevisionReturnsFieldErrors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubRevisionService{
		applyErr: &listingkit.RevisionValidationError{
			Fields: []listingkit.RevisionFieldError{{
				FieldPath: "shein.category_id",
				Code:      "invalid_value",
				Message:   "category_id 必须大于 0",
			}},
		},
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/revision", h.ApplyTaskRevision)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/revision", strings.NewReader(`{"platform":"shein","shein":{"category_id":0}}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
	var body struct {
		Error       string                          `json:"error"`
		FieldErrors []listingkit.RevisionFieldError `json:"field_errors"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.FieldErrors) != 1 || body.FieldErrors[0].FieldPath != "shein.category_id" {
		t.Fatalf("field errors = %+v", body.FieldErrors)
	}
}

func TestApplyTaskRevisionReturnsAppliedChanges(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubRevisionService{
		applyPreview: &listingkit.ListingKitPreview{
			TaskID: "task-1",
			ApplyResult: &listingkit.RevisionApplyResult{
				Applied: true,
				SuccessPayload: &listingkit.RevisionSuccessPayload{
					Mode: "apply",
					Core: &listingkit.RevisionSuccessCoreData{
						ActionType:  listingkit.RevisionActionTypeEdit,
						Headline:    "更新 SHEIN 资料",
						ChangeCount: 1,
						StatusSummary: &listingkit.RevisionStatusSummary{
							Status:      "ready",
							Headline:    "SHEIN 资料包已可进入提交流程",
							Subheadline: "关键字段已满足提交前要求",
						},
						FollowUpChecklist: &listingkit.RevisionFollowUpChecklist{
							Recommended: []listingkit.SheinChecklistGroupItem{{
								Key:             "manual_notes",
								Label:           "人工备注",
								Status:          "warning",
								Message:         "仍有人工备注未处理，建议在提交前再次确认",
								SuggestedAction: "处理备注",
							}},
						},
						FollowUpOverview: &listingkit.RevisionFollowUpOverview{
							Status:           "ready",
							Headline:         "保存后可以继续提交流程",
							Subheadline:      "关键字段已满足提交前要求",
							RecommendedCount: 1,
							NextActions:      []string{"继续提交流程"},
						},
						SuggestedFollowUpRevision: &listingkit.SheinEditorRevisionSkeleton{
							Platform: "shein",
							Actor:    "desktop-client",
							Reason:   "follow-up after apply",
						},
					},
					Presentation: &listingkit.RevisionInteractionPresentation{
						Scene:       "apply_success",
						NextActions: []string{"继续提交流程"},
						Messages: &listingkit.RevisionResultMessages{
							Title:        "更新 SHEIN 资料",
							Description:  "本次已保存 1 个字段的更新。",
							SuccessLabel: "保存成功",
						},
						RecommendedView: &listingkit.RevisionRecommendedView{
							View:   "submit",
							Reason: "保存后可以直接继续提交流程。",
						},
						SummaryCard: &listingkit.RevisionSuccessSummaryCard{
							Status:        "ready",
							Title:         "更新 SHEIN 资料",
							Subtitle:      "本次共更新 1 个字段。",
							PrimaryAction: "继续提交流程",
							PrimaryView:   "submit",
							Highlights:    []string{"SPU 名称"},
						},
					},
				},
			},
			AppliedChanges: &listingkit.RevisionDiffPreview{
				ChangeCount: 1,
				Changes: []listingkit.RevisionFieldChange{{
					FieldPath: "shein.spu_name",
					Label:     "SPU 名称",
				}},
			},
			RevisionHistory: []listingkit.ListingKitRevisionRecord{{
				Platform:               "shein",
				ActionType:             listingkit.RevisionActionTypeRestore,
				RestoredFromRevisionID: "rev-0",
				EditorContext: &listingkit.SheinEditorContext{
					Basics: &listingkit.SheinEditorBasicsContext{SpuName: "New"},
				},
			}},
			RestoreResult: &listingkit.RevisionRestoreResult{
				Applied: true,
				SuccessPayload: &listingkit.RevisionSuccessPayload{
					Mode: "restore",
					Core: &listingkit.RevisionSuccessCoreData{
						ActionType:       listingkit.RevisionActionTypeRestore,
						Headline:         "恢复历史版本",
						ChangeCount:      1,
						SourceRevisionID: "rev-0",
						RelationText:     "恢复自 rev-0",
						StatusSummary: &listingkit.RevisionStatusSummary{
							Status:      "ready",
							Headline:    "SHEIN 资料包已可进入提交流程",
							Subheadline: "关键字段已满足提交前要求",
						},
						FollowUpChecklist: &listingkit.RevisionFollowUpChecklist{
							Recommended: []listingkit.SheinChecklistGroupItem{{
								Key:             "manual_notes",
								Label:           "人工备注",
								Status:          "warning",
								Message:         "仍有人工备注未处理，建议在提交前再次确认",
								SuggestedAction: "处理备注",
							}},
						},
						FollowUpOverview: &listingkit.RevisionFollowUpOverview{
							Status:           "ready",
							Headline:         "恢复后可以继续提交流程",
							Subheadline:      "关键字段已满足提交前要求",
							RecommendedCount: 1,
							NextActions:      []string{"继续提交流程"},
						},
						SuggestedFollowUpRevision: &listingkit.SheinEditorRevisionSkeleton{
							Platform: "shein",
							Actor:    "desktop-client",
							Reason:   "follow-up after restore",
						},
					},
					Presentation: &listingkit.RevisionInteractionPresentation{
						Scene:       "restore_success",
						NextActions: []string{"继续提交流程"},
						Messages: &listingkit.RevisionResultMessages{
							Title:        "恢复历史版本",
							Description:  "已恢复到历史版本 rev-0，共覆盖 1 个字段。",
							SuccessLabel: "恢复成功",
						},
						RecommendedView: &listingkit.RevisionRecommendedView{
							View:   "submit",
							Reason: "恢复后可以直接继续提交流程。",
						},
						SummaryCard: &listingkit.RevisionSuccessSummaryCard{
							Status:        "ready",
							Title:         "恢复历史版本",
							Subtitle:      "关键字段已满足提交前要求",
							PrimaryAction: "继续提交流程",
							PrimaryView:   "submit",
							Highlights:    []string{"恢复自 rev-0"},
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
	router.POST("/api/v1/listing-kits/tasks/:task_id/revision", h.ApplyTaskRevision)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/revision", strings.NewReader(`{"platform":"shein","shein":{"spu_name":"New"}}`))
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
	if body.AppliedChanges == nil || body.AppliedChanges.ChangeCount != 1 {
		t.Fatalf("applied changes = %+v", body.AppliedChanges)
	}
	if body.ApplyResult == nil || body.ApplyResult.SuccessPayload == nil || body.ApplyResult.SuccessPayload.Presentation == nil || body.ApplyResult.SuccessPayload.Presentation.Scene != "apply_success" || body.ApplyResult.SuccessPayload.Presentation.SummaryCard == nil || body.ApplyResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if len(body.ApplyResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Core == nil || body.ApplyResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Presentation.Messages == nil || body.ApplyResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Presentation.RecommendedView == nil || body.ApplyResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Core.FollowUpOverview == nil || body.ApplyResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || body.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if body.ApplyResult.SuccessPayload == nil || body.ApplyResult.SuccessPayload.Mode != "apply" {
		t.Fatalf("apply result = %+v", body.ApplyResult)
	}
	if len(body.RevisionHistory) != 1 {
		t.Fatalf("revision history = %+v", body.RevisionHistory)
	}
	if body.RevisionHistory[0].ActionType != listingkit.RevisionActionTypeRestore {
		t.Fatalf("revision history = %+v", body.RevisionHistory)
	}
	if body.RevisionHistory[0].RestoredFromRevisionID != "rev-0" {
		t.Fatalf("revision history = %+v", body.RevisionHistory)
	}
	if body.RevisionHistory[0].EditorContext == nil || body.RevisionHistory[0].EditorContext.Basics == nil {
		t.Fatalf("revision history snapshot = %+v", body.RevisionHistory[0].EditorContext)
	}
	if body.RestoreResult == nil || body.RestoreResult.SuccessPayload == nil || body.RestoreResult.SuccessPayload.Core == nil || body.RestoreResult.SuccessPayload.Core.SourceRevisionID != "rev-0" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if len(body.RestoreResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Presentation.Messages == nil || body.RestoreResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Presentation.RecommendedView == nil || body.RestoreResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Core.FollowUpOverview == nil || body.RestoreResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || body.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload.Presentation.Scene != "restore_success" || body.RestoreResult.SuccessPayload.Presentation.SummaryCard == nil || body.RestoreResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
	if body.RestoreResult.SuccessPayload == nil || body.RestoreResult.SuccessPayload.Mode != "restore" {
		t.Fatalf("restore result = %+v", body.RestoreResult)
	}
}

func TestApplyTaskRevisionReturnsNotFoundForMissingRestoreRevision(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	svc := &stubRevisionService{
		applyErr: listingkit.ErrRevisionHistoryRecordNotFound,
	}
	h, err := NewHandler(svc)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	router := gin.New()
	router.POST("/api/v1/listing-kits/tasks/:task_id/revision", h.ApplyTaskRevision)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/tasks/task-1/revision", strings.NewReader(`{"platform":"shein","restore_from_revision_id":"missing"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}
