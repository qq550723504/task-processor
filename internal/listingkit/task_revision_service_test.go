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
	recovery := newTaskSubmissionRecoveryService(taskSubmissionRecoveryServiceConfig{
		repo: repo,
	})
	revision := newTaskRevisionService(taskRevisionServiceConfig{
		repo: repo,
		resolveManualSheinSaleAttributeValueIDs: func(ctx context.Context, task *Task, req *ApplyRevisionRequest) error {
			manualResolveCalls++
			return nil
		},
		recovery: recovery,
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

func TestTaskRevisionServiceGetTaskRevisionHistoryAttachesStoreResolution(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubApplyRevisionRepo{
		task: &Task{
			ID: "task-revision-history-service-1",
			SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
				StoreID:          903,
				Site:             "GB",
				Strategy:         "country",
				Reason:           "matched country route",
				MatchedRuleKinds: []string{"country"},
				MatchedProfileID: 17,
				ResolvedAt:       now,
			},
			Result: &ListingKitResult{
				TaskID:               "task-revision-history-service-1",
				RevisionHistoryTotal: 1,
				RevisionHistory: []ListingKitRevisionRecord{
					{Platform: "shein", UpdatedAt: now.Add(-time.Minute)},
				},
			},
		},
	}
	revision := newTaskRevisionService(taskRevisionServiceConfig{
		repo: repo,
	})

	page, err := revision.GetTaskRevisionHistory(context.Background(), repo.task.ID, &RevisionHistoryQuery{Limit: 10})
	if err != nil {
		t.Fatalf("GetTaskRevisionHistory() error = %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v, want 1 history item", page.Items)
	}
	if page.Items[0].StoreResolution == nil || page.Items[0].StoreResolution.StoreID != 903 {
		t.Fatalf("store resolution = %+v, want snapshot-backed item", page.Items[0].StoreResolution)
	}
}
