package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type stubApplyRevisionRepo struct {
	task *Task
}

func (r *stubApplyRevisionRepo) CreateTask(ctx context.Context, task *Task) error {
	r.task = task
	return nil
}
func (r *stubApplyRevisionRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return r.task, nil
}
func (r *stubApplyRevisionRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubApplyRevisionRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return nil
}
func (r *stubApplyRevisionRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubApplyRevisionRepo) PrepareRetry(ctx context.Context, taskID string) error { return nil }
func (r *stubApplyRevisionRepo) IncrementRetryCount(ctx context.Context, taskID string) error {
	return nil
}
func (r *stubApplyRevisionRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	r.task.Result = result
	return nil
}

func TestApplyTaskRevisionReturnsAppliedChanges(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:     "task-apply-1",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-1",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "asset-main",
					Kind: asset.KindMainImage,
					URL:  "https://cdn.example.com/old.jpg",
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.bags",
						"scene_defaults_source": "explicit",
						"scene_category":        "bags",
						"scene_style":           "studio",
					},
				}},
			},
			Shein: &SheinPackage{
				SpuName:       "Old Bottle",
				ProductNameEn: "Old Bottle",
				BrandName:     "Old Brand",
				Description:   "old desc",
				Images: &PlatformImageSet{
					MainImage: "https://cdn.example.com/old.jpg",
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID: "asset-main",
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	newName := "New Bottle"
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName:       &newName,
			ProductNameEn: &newName,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview == nil || preview.AppliedChanges == nil || preview.AppliedChanges.ChangeCount == 0 {
		t.Fatalf("applied changes = %+v", preview)
	}
	if preview.ApplyResult == nil || preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Presentation == nil || preview.ApplyResult.SuccessPayload.Presentation.Scene != revisionPresentationSceneApplySuccess || preview.ApplyResult.SuccessPayload.Presentation.SummaryCard == nil || preview.ApplyResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.Shein == nil || len(preview.Shein.ScenePresets) != 1 {
		t.Fatalf("shein scene presets = %+v", preview.Shein)
	}
	if preview.Shein.ScenePresets[0].ScenePreset == nil || preview.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.bags" {
		t.Fatalf("shein scene presets = %+v", preview.Shein.ScenePresets)
	}
	if len(preview.ApplyResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core == nil || preview.ApplyResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.Messages == nil || preview.ApplyResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.RecommendedView == nil || preview.ApplyResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpOverview == nil || preview.ApplyResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Mode != "apply" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if len(preview.RevisionHistory) != 1 || preview.RevisionHistory[0].AppliedChanges == nil {
		t.Fatalf("revision history = %+v", preview.RevisionHistory)
	}
	if preview.RevisionHistory[0].RevisionID == "" {
		t.Fatalf("revision history record missing revision_id: %+v", preview.RevisionHistory[0])
	}
	if preview.RevisionHistory[0].ActionType != RevisionActionTypeEdit {
		t.Fatalf("revision history action type = %+v", preview.RevisionHistory[0])
	}
	if preview.RevisionHistory[0].Timeline == nil || preview.RevisionHistory[0].Timeline.Badge != "编辑" {
		t.Fatalf("revision history timeline = %+v", preview.RevisionHistory[0].Timeline)
	}
	if preview.RevisionHistory[0].EditorContext == nil || preview.RevisionHistory[0].EditorContext.Basics == nil {
		t.Fatalf("revision history snapshot = %+v", preview.RevisionHistory[0].EditorContext)
	}
	if preview.RevisionHistoryMeta == nil || preview.RevisionHistoryMeta.TotalRecords != 1 {
		t.Fatalf("revision history meta = %+v", preview.RevisionHistoryMeta)
	}
}

func TestApplyTaskRevisionTrimsRevisionHistory(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	history := make([]ListingKitRevisionRecord, 0, maxRevisionHistoryRecords)
	for i := 0; i < maxRevisionHistoryRecords; i++ {
		history = append(history, ListingKitRevisionRecord{
			UpdatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			UpdatedBy: "tester",
			Reason:    "seed",
			Platform:  "shein",
		})
	}
	task := &Task{
		ID:     "task-apply-2",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID:               "task-apply-2",
			RevisionHistoryTotal: maxRevisionHistoryRecords,
			RevisionHistory:      history,
			Shein: &SheinPackage{
				SpuName:       "Old Bottle",
				ProductNameEn: "Old Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	newName := "Trimmed Bottle"
	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName: &newName,
		},
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if len(preview.RevisionHistory) != maxRevisionHistoryRecords {
		t.Fatalf("revision history length = %d, want %d", len(preview.RevisionHistory), maxRevisionHistoryRecords)
	}
	if preview.RevisionHistoryMeta == nil {
		t.Fatal("expected revision history meta")
	}
	if preview.RevisionHistoryMeta.TotalRecords != maxRevisionHistoryRecords+1 {
		t.Fatalf("total records = %d, want %d", preview.RevisionHistoryMeta.TotalRecords, maxRevisionHistoryRecords+1)
	}
	if !preview.RevisionHistoryMeta.HasMore {
		t.Fatalf("history meta = %+v, want has_more", preview.RevisionHistoryMeta)
	}
	last := preview.RevisionHistory[len(preview.RevisionHistory)-1]
	if last.RevisionID == "" || last.AppliedChanges == nil || last.EditorContext == nil {
		t.Fatalf("latest history record = %+v", last)
	}
}

func TestApplyTaskRevisionSupportsRestoreFromRevisionID(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	restoreName := "Restored Bottle"
	task := &Task{
		ID:     "task-apply-restore",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-restore",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "asset-main",
					Kind: asset.KindMainImage,
					URL:  "https://cdn.example.com/current.jpg",
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.shoes",
						"scene_defaults_source": "platform_category",
						"scene_category":        "shoes",
						"scene_style":           "lifestyle",
					},
				}},
			},
			Shein: &SheinPackage{
				SpuName:       "Current Bottle",
				ProductNameEn: "Current Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID: "asset-main",
					},
				},
			},
			RevisionHistory: []ListingKitRevisionRecord{{
				RevisionID: "rev-restore-1",
				Platform:   "shein",
				Reason:     "manual adjustment",
				EditorContext: &SheinEditorContext{
					RevisionSkeleton: &SheinEditorRevisionSkeleton{
						Platform: "shein",
						Reason:   "manual adjustment",
						Shein: &SheinRevisionInput{
							SpuName:       &restoreName,
							ProductNameEn: &restoreName,
						},
					},
				},
			}},
			RevisionHistoryTotal: 1,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	preview, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform:              "shein",
		RestoreFromRevisionID: "rev-restore-1",
	})
	if err != nil {
		t.Fatalf("apply task revision: %v", err)
	}
	if preview.Shein == nil || preview.Shein.Headline != restoreName {
		t.Fatalf("preview shein = %+v", preview.Shein)
	}
	if len(preview.Shein.ScenePresets) != 1 || preview.Shein.ScenePresets[0].ScenePreset == nil || preview.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.shoes" {
		t.Fatalf("shein scene presets = %+v", preview.Shein.ScenePresets)
	}
	if repo.task.Result.Shein == nil || repo.task.Result.Shein.SpuName != restoreName {
		t.Fatalf("result shein = %+v", repo.task.Result.Shein)
	}
	if repo.task.Result.Revision == nil || repo.task.Result.Revision.Reason != "restore: manual adjustment" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.ActionType != RevisionActionTypeRestore {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.Timeline == nil || repo.task.Result.Revision.Timeline.Badge != "回滚" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if repo.task.Result.Revision.RestoredFromRevisionID != "rev-restore-1" {
		t.Fatalf("revision summary = %+v", repo.task.Result.Revision)
	}
	if preview.RestoreResult == nil || preview.RestoreResult.SuccessPayload == nil || preview.RestoreResult.SuccessPayload.Core == nil || preview.RestoreResult.SuccessPayload.Core.SourceRevisionID != "rev-restore-1" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.ApplyResult == nil || preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Core == nil || preview.ApplyResult.SuccessPayload.Core.ActionType != RevisionActionTypeRestore {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if len(preview.ApplyResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.Messages == nil || preview.ApplyResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Presentation.RecommendedView == nil || preview.ApplyResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.FollowUpOverview == nil || preview.ApplyResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.ApplyResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.ApplyResult.SuccessPayload == nil || preview.ApplyResult.SuccessPayload.Mode != "apply" {
		t.Fatalf("apply result = %+v", preview.ApplyResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.ActionType != RevisionActionTypeRestore {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if len(preview.RestoreResult.SuccessPayload.Presentation.NextActions) == 0 {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.StatusSummary == nil {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.Messages == nil || preview.RestoreResult.SuccessPayload.Presentation.Messages.SuccessLabel == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.RecommendedView == nil || preview.RestoreResult.SuccessPayload.Presentation.RecommendedView.View == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.FollowUpChecklist == nil {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.FollowUpOverview == nil || preview.RestoreResult.SuccessPayload.Core.FollowUpOverview.Headline == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision == nil || preview.RestoreResult.SuccessPayload.Core.SuggestedFollowUpRevision.Platform != "shein" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload.Presentation.Scene != revisionPresentationSceneRestoreSuccess || preview.RestoreResult.SuccessPayload.Presentation.SummaryCard == nil || preview.RestoreResult.SuccessPayload.Presentation.SummaryCard.Title == "" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if preview.RestoreResult.SuccessPayload == nil || preview.RestoreResult.SuccessPayload.Mode != "restore" {
		t.Fatalf("restore result = %+v", preview.RestoreResult)
	}
	if len(preview.RevisionHistory) != 2 {
		t.Fatalf("revision history = %+v", preview.RevisionHistory)
	}
	last := preview.RevisionHistory[len(preview.RevisionHistory)-1]
	if last.ActionType != RevisionActionTypeRestore {
		t.Fatalf("latest revision history = %+v", last)
	}
	if last.Timeline == nil || last.Timeline.RelationText != "恢复自 rev-restore-1" {
		t.Fatalf("latest revision history = %+v", last)
	}
	if last.RestoredFromRevisionID != "rev-restore-1" {
		t.Fatalf("latest revision history = %+v", last)
	}
}

func TestApplyTaskRevisionReturnsNotFoundForMissingRestoreRevision(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:     "task-apply-restore-missing",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-apply-restore-missing",
			Shein: &SheinPackage{
				SpuName: "Current Bottle",
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.CreateTask(context.Background(), task)
	svc := &service{repo: repo}

	_, err := svc.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform:              "shein",
		RestoreFromRevisionID: "missing",
	})
	if err == nil || err != ErrRevisionHistoryRecordNotFound {
		t.Fatalf("error = %v, want %v", err, ErrRevisionHistoryRecordNotFound)
	}
}
