package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func TestBuildGenerationReviewSessionIncludesFocusedScenePreset(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		TaskID: "task-scene-preset-session-1",
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{{
				ID:   "scene-asset-1",
				Kind: asset.KindSceneImage,
				Metadata: map[string]string{
					"prompt_key":            "productimage.scene.shoes",
					"scene_defaults_source": "platform_category",
					"scene_category":        "shoes",
					"scene_style":           "studio",
					"background_tone":       "bright",
					"composition":           "centered",
					"props_level":           "none",
					"audience_hint":         "premium",
				},
			}},
		},
		AssetRenderPreviews: []AssetRenderPreview{{
			AssetID:    "scene-asset-1",
			PreviewSVG: "<svg/>",
		}},
		Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
			Platform: "amazon",
			Main: &common.BundleSlot{
				Key:     "main",
				AssetID: "scene-asset-1",
			},
		}},
	}

	session := buildGenerationReviewSession(result, nil, &GenerationQueueQuery{
		Platform: "amazon",
		Slot:     "main",
	})
	if session == nil || session.FocusedScenePreset == nil {
		t.Fatalf("session = %+v, want focused scene preset", session)
	}
	if session.FocusedScenePreset.PromptKey != "productimage.scene.shoes" {
		t.Fatalf("focused scene preset = %+v, want scene prompt key", session.FocusedScenePreset)
	}
	if session.FocusedScenePreset.DefaultsSource != "platform_category" || session.FocusedScenePreset.SceneStyle != "studio" {
		t.Fatalf("focused scene preset = %+v, want preserved scene metadata", session.FocusedScenePreset)
	}
}

func TestGetTaskGenerationReviewPreviewIncludesScenePreset(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	svc := &service{repo: repo}
	task := &Task{
		ID:        "task-scene-preset-preview-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"amazon"}},
		Result: &ListingKitResult{
			TaskID: "task-scene-preset-preview-1",
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{{
					ID:   "scene-asset-1",
					Kind: asset.KindSceneImage,
					Metadata: map[string]string{
						"prompt_key":            "productimage.scene.bags",
						"scene_defaults_source": "platform_category",
						"scene_category":        "bags",
						"scene_style":           "lifestyle",
					},
				}},
			},
			AssetRenderPreviews: []AssetRenderPreview{{
				AssetID:         "scene-asset-1",
				AssetRevision:   "asset-rev-1",
				PreviewRevision: "preview-rev-1",
				TaskRevision:    "task-rev-1",
				PreviewFormat:   "svg",
				PreviewSVG:      "<svg/>",
			}},
			Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
				Platform: "amazon",
				Main: &common.BundleSlot{
					Key:           "main",
					AssetID:       "scene-asset-1",
					StateLabel:    "ready",
					TemplateLabel: "Amazon Main",
				},
			}},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	response, err := svc.GetTaskGenerationReviewPreview(context.Background(), task.ID, &GenerationQueueQuery{
		Platform: "amazon",
		Slot:     "main",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationReviewPreview() error = %v", err)
	}
	if response.ScenePreset == nil {
		t.Fatalf("response = %+v, want scene preset", response)
	}
	if response.ScenePreset.PromptKey != "productimage.scene.bags" || response.ScenePreset.SceneCategory != "bags" {
		t.Fatalf("scene preset = %+v, want bag scene metadata", response.ScenePreset)
	}
}

func TestBuildGenerationWorkQueueIncludesScenePreset(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		TaskID: "task-scene-preset-queue-1",
		AssetBundle: &asset.Bundle{
			Assets: []asset.Asset{{
				ID:   "scene-asset-1",
				Kind: asset.KindSceneImage,
				Metadata: map[string]string{
					"prompt_key":            "productimage.scene.jewelry",
					"scene_defaults_source": "platform_category",
					"scene_category":        "jewelry",
					"scene_style":           "lifestyle",
				},
			}},
		},
		AssetRenderPreviews: []AssetRenderPreview{{
			AssetID:    "scene-asset-1",
			PreviewSVG: "<svg/>",
		}},
		Amazon: &AmazonPackage{ImageBundle: &common.PublishImageBundle{
			Platform: "amazon",
			Main: &common.BundleSlot{
				Key:           "main",
				AssetID:       "scene-asset-1",
				StateLabel:    "ready",
				TemplateLabel: "Amazon Main",
			},
		}},
	}

	queue := buildGenerationWorkQueue(result)
	if queue == nil || len(queue.Items) != 1 {
		t.Fatalf("queue = %+v, want one queue item", queue)
	}
	if queue.Items[0].ScenePreset == nil {
		t.Fatalf("queue item = %+v, want scene preset", queue.Items[0])
	}
	if queue.Items[0].ScenePreset.PromptKey != "productimage.scene.jewelry" || queue.Items[0].ScenePreset.SceneCategory != "jewelry" {
		t.Fatalf("queue item scene preset = %+v, want jewelry scene metadata", queue.Items[0].ScenePreset)
	}
}
