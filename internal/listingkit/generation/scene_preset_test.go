package generation

import "testing"

func TestScenePresetSummaryFromMetadataTrimsAndKeepsPresetFields(t *testing.T) {
	t.Parallel()

	got := ScenePresetSummaryFromMetadata(map[string]string{
		"prompt_key":            " productimage.scene.hero ",
		"scene_defaults_source": " listingkit ",
		"scene_category":        " lifestyle ",
	})
	if got == nil {
		t.Fatalf("ScenePresetSummaryFromMetadata() = nil, want summary")
	}
	if got.PromptKey != "productimage.scene.hero" || got.DefaultsSource != "listingkit" || got.SceneCategory != "lifestyle" {
		t.Fatalf("ScenePresetSummaryFromMetadata() = %+v, want trimmed fields", got)
	}
}

func TestScenePresetSummaryFromMetadataRejectsEmptyMetadata(t *testing.T) {
	t.Parallel()

	if got := ScenePresetSummaryFromMetadata(map[string]string{"prompt_key": "other.prompt"}); got != nil {
		t.Fatalf("ScenePresetSummaryFromMetadata() = %+v, want nil", got)
	}
	if HasScenePresetSummary(nil) {
		t.Fatalf("HasScenePresetSummary(nil) = true, want false")
	}
}
