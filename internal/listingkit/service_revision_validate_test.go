package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

type stubValidateRepo struct {
	task *Task
}

func (r *stubValidateRepo) CreateTask(ctx context.Context, task *Task) error {
	r.task = task
	return nil
}
func (r *stubValidateRepo) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return r.task, nil
}
func (r *stubValidateRepo) ListTasks(ctx context.Context, query *TaskListQuery) ([]Task, int64, error) {
	if r.task == nil {
		return []Task{}, 0, nil
	}
	return []Task{*r.task}, 1, nil
}
func (r *stubValidateRepo) MarkProcessing(ctx context.Context, taskID string) error { return nil }
func (r *stubValidateRepo) MarkCompleted(ctx context.Context, taskID string, result *ListingKitResult) error {
	return nil
}
func (r *stubValidateRepo) MarkNeedsReview(ctx context.Context, taskID string, result *ListingKitResult, reason string) error {
	return nil
}
func (r *stubValidateRepo) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return nil
}
func (r *stubValidateRepo) PrepareRetry(ctx context.Context, taskID string) error        { return nil }
func (r *stubValidateRepo) IncrementRetryCount(ctx context.Context, taskID string) error { return nil }
func (r *stubValidateRepo) SaveTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	return nil
}

func TestValidateTaskRevisionReturnsFieldErrorsAndHints(t *testing.T) {
	t.Parallel()

	repo := &stubValidateRepo{}
	task := &Task{
		ID: "task-validate-1",
		Result: &ListingKitResult{
			TaskID: "task-validate-1",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "asset-main",
					Kind: asset.KindMainImage,
					URL:  "https://cdn.example.com/main.jpg",
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.bags",
						"scene_defaults_source": "explicit",
						"scene_category":        "bags",
						"scene_style":           "studio",
					},
				}},
			},
			Shein: &SheinPackage{
				CategoryID: 123,
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID: "asset-main",
					},
				},
				RequestDraft: &SheinRequestDraft{
					SKCList: []SheinSKCRequestDraft{{SupplierCode: "SKC-1"}},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc := &service{repo: repo}

	result, err := svc.ValidateTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryID: ptrInt(0),
		},
	})
	if err != nil {
		t.Fatalf("validate task revision: %v", err)
	}
	if result.Valid {
		t.Fatalf("valid = true, want false; result=%+v", result)
	}
	if len(result.FieldErrors) == 0 {
		t.Fatalf("field errors = %+v, want validation details", result)
	}
	if result.Shein == nil || result.Shein.DirtyHints == nil || len(result.Shein.CategoryPreviewEffects) == 0 {
		t.Fatalf("shein validation payload = %+v", result.Shein)
	}
	if result.Shein.SuggestedMinimalRevision == nil || result.Shein.SuggestedMinimalRevision.Shein == nil {
		t.Fatalf("minimal revision = %+v", result.Shein.SuggestedMinimalRevision)
	}
	if len(result.ScenePresets) != 1 {
		t.Fatalf("scene presets = %+v, want 1 summary", result.ScenePresets)
	}
	if result.ScenePresets[0].ScenePreset == nil || result.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.bags" {
		t.Fatalf("scene presets = %+v, want bag scene prompt", result.ScenePresets)
	}
	if result.Shein.RevisionDiffPreview == nil {
		t.Fatalf("revision diff preview = %+v", result.Shein.RevisionDiffPreview)
	}
}

func TestValidateTaskRevisionReturnsRestorePreview(t *testing.T) {
	t.Parallel()

	repo := &stubValidateRepo{}
	spuName := "Restore Bottle"
	task := &Task{
		ID: "task-validate-restore",
		Result: &ListingKitResult{
			TaskID: "task-validate-restore",
			Shein: &SheinPackage{
				CategoryID: 123,
			},
			RevisionHistory: []ListingKitRevisionRecord{{
				RevisionID: "rev-restore-1",
				Platform:   "shein",
				Reason:     "manual adjustment",
				EditorContext: &SheinEditorContext{
					RevisionSkeleton: &SheinEditorRevisionSkeleton{
						Platform: "shein",
						Shein: &SheinRevisionInput{
							SpuName: &spuName,
						},
					},
				},
			}},
			RevisionHistoryTotal: 1,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc := &service{repo: repo}

	result, err := svc.ValidateTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform:              "shein",
		RestoreFromRevisionID: "rev-restore-1",
	})
	if err != nil {
		t.Fatalf("validate task revision: %v", err)
	}
	if result.Shein == nil || result.Shein.RestorePreview == nil {
		t.Fatalf("restore preview = %+v", result.Shein)
	}
	if result.Shein.RestorePreview.Core == nil || result.Shein.RestorePreview.Core.Context == nil || result.Shein.RestorePreview.Core.Context.SourceRevisionID != "rev-restore-1" {
		t.Fatalf("restore preview = %+v", result.Shein.RestorePreview)
	}
	if result.Shein.RestorePreview.Core.Draft == nil || result.Shein.RestorePreview.Core.Draft.Shein == nil || result.Shein.RestorePreview.Core.Draft.Shein.SpuName == nil {
		t.Fatalf("restore draft = %+v", result.Shein.RestorePreview.Core.Draft)
	}
	if result.Shein.RestorePreview.Core.Compare == nil || result.Shein.RestorePreview.Core.Compare.DiffPreview == nil {
		t.Fatalf("restore preview compare = %+v", result.Shein.RestorePreview)
	}
	if result.Shein.RestorePreview.Presentation == nil || result.Shein.RestorePreview.Presentation.Scene != revisionPresentationSceneRestorePreview || result.Shein.RestorePreview.Presentation.SummaryCard == nil {
		t.Fatalf("restore preview presentation = %+v", result.Shein.RestorePreview)
	}
}
