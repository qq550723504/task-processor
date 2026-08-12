package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func TestDecorateSheinCookieAvailabilityPreviewUsesSheinTargetBundleWithoutCompatibilityScalar(t *testing.T) {
	sheinBundle := &asset.Bundle{Assets: []asset.Asset{{
		ID:   "shared-main",
		Kind: asset.KindMainImage,
		URL:  "https://cdn.example.test/shein-cookie-preview.jpg",
		Metadata: map[string]string{
			"prompt_key":            "productimage.scene.shein",
			"scene_defaults_source": "explicit",
			"scene_category":        "fashion",
		},
	}}}
	task := &Task{Result: &ListingKitResult{
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"temu": {Assets: []asset.Asset{{
				ID:       "shared-main",
				Kind:     asset.KindMainImage,
				URL:      "https://cdn.example.test/temu-cookie-preview.jpg",
				Metadata: map[string]string{"prompt_key": "productimage.scene.temu"},
			}}},
			"shein": sheinBundle,
		},
		Shein: &SheinPackage{
			ImageBundle:  &common.PublishImageBundle{Platform: "shein", Main: &common.BundleSlot{AssetID: "shared-main"}},
			RequestDraft: &SheinRequestDraft{},
		},
	}}
	preview := &ListingKitPreview{Shein: buildSheinPreviewPayload(task.Result.Shein, nil, nil, sheinBundle, nil)}

	(&service{}).decorateSheinCookieAvailabilityPreview(context.Background(), task, preview)

	if task.Result.AssetBundle != nil {
		t.Fatalf("compatibility asset scalar = %+v, want nil for multi-target result", task.Result.AssetBundle)
	}
	if preview.Shein == nil || len(preview.Shein.ScenePresets) != 1 || preview.Shein.ScenePresets[0].ScenePreset == nil {
		t.Fatalf("SHEIN scene presets = %+v, want non-empty target preset", preview.Shein)
	}
	if got := preview.Shein.ScenePresets[0].ScenePreset.PromptKey; got != "productimage.scene.shein" {
		t.Fatalf("SHEIN scene prompt = %q, want SHEIN target bundle prompt", got)
	}
}
