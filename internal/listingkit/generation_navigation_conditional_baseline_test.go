package listingkit

import (
	"context"
	"testing"
	"time"

	common "task-processor/internal/publishing/common"
)

func TestDispatchTaskGenerationNavigationUsesTargetConditionalBaseline(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-conditional-baseline-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-conditional-baseline-1",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "asset-preview-1",
					StateLabel:    "ready",
					TemplateLabel: "SHEIN Main",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	first, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform:          "shein",
		Slot:              "main",
		PreviewCapability: "detail_preview",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if first == nil || first.DeltaToken == "" {
		t.Fatalf("first = %+v, want delta token", first)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			Conditional:  &GenerationConditionalState{DeltaToken: first.DeltaToken},
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || !response.NotModified || response.DeltaToken != first.DeltaToken {
		t.Fatalf("response = %+v, want not_modified dispatch from target baseline", response)
	}
}

func TestDispatchTaskGenerationNavigationKeepsExplicitQueryOverConditionalBaseline(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-generation-navigation-conditional-baseline-2",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result: &ListingKitResult{
			TaskID: "task-generation-navigation-conditional-baseline-2",
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "asset-preview-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
				VisualMode:      "selling_point",
				LayerTypes:      []string{"detail", "text"},
			}},
			Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "shein",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "asset-preview-1",
					StateLabel:    "ready",
					TemplateLabel: "SHEIN Main",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.DispatchTaskGenerationNavigation(context.Background(), task.ID, &GenerationReviewNavigationDispatchRequest{
		ResponseMode: "patch_only",
		Target: &GenerationReviewNavigationTarget{
			DispatchKind: "preview",
			Conditional:  &GenerationConditionalState{DeltaToken: "delta-body"},
			PreviewQuery: &GenerationQueueQuery{
				Platform:          "shein",
				Slot:              "main",
				PreviewCapability: "detail_preview",
				IfMatch:           "delta-explicit",
			},
		},
	})
	if err != nil {
		t.Fatalf("DispatchTaskGenerationNavigation() error = %v", err)
	}
	if response == nil || response.NotModified {
		t.Fatalf("response = %+v, want explicit query to win over body baseline", response)
	}
}
