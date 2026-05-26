package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func TestTaskRevisionServiceApplyTaskRevisionInvokesSheinCollaborators(t *testing.T) {
	t.Parallel()

	repo := &stubApplyRevisionRepo{}
	task := &Task{
		ID:     "task-revision-service-1",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			TaskID: "task-revision-service-1",
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

	var manualResolveCalls int
	var refreshCalls int
	var previewCalls int
	revision := newTaskRevisionService(taskRevisionServiceConfig{
		repo: repo,
		resolveManualSheinSaleAttributeValueIDs: func(ctx context.Context, task *Task, req *ApplyRevisionRequest) error {
			manualResolveCalls++
			return nil
		},
		mutateTaskResult: func(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
			if err := mutate(task); err != nil {
				return nil, err
			}
			return task, nil
		},
		refreshSheinDerivedState: func(task *Task, req *ApplyRevisionRequest) {
			refreshCalls++
		},
		buildTaskPreview: func(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
			previewCalls++
			return &ListingKitPreview{TaskID: task.ID}, nil
		},
	})

	newName := "New Bottle"
	preview, err := revision.ApplyTaskRevision(context.Background(), task.ID, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SpuName:       &newName,
			ProductNameEn: &newName,
		},
	})
	if err != nil {
		t.Fatalf("ApplyTaskRevision() error = %v", err)
	}
	if manualResolveCalls != 1 {
		t.Fatalf("manual resolve calls = %d, want 1", manualResolveCalls)
	}
	if refreshCalls != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCalls)
	}
	if previewCalls != 1 {
		t.Fatalf("preview calls = %d, want 1", previewCalls)
	}
	if preview == nil || preview.TaskID != task.ID {
		t.Fatalf("preview = %+v, want preview for task", preview)
	}
	if preview.AppliedChanges == nil || preview.AppliedChanges.ChangeCount == 0 {
		t.Fatalf("applied changes = %+v, want populated diff preview", preview.AppliedChanges)
	}
}
