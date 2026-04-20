package productimage

import "testing"

func TestApplyGenerationMetadataMapStampsNonEmptyValues(t *testing.T) {
	metadata := applyGenerationMetadataMap(map[string]string{}, &GenerationMetadata{
		Provider:       "openai",
		ModelFamily:    "gpt-image",
		GenerationMode: "scene_generation",
		PromptRef:      "productimage.scene.default",
		PromptKey:      "productimage.scene.default",
		PromptSource:   "registry",
		PromptVersion:  "default",
	})

	if metadata["model_provider"] != "openai" ||
		metadata["model_family"] != "gpt-image" ||
		metadata["generation_mode"] != "scene_generation" ||
		metadata["prompt_ref"] != "productimage.scene.default" ||
		metadata["prompt_key"] != "productimage.scene.default" ||
		metadata["prompt_source"] != "registry" ||
		metadata["prompt_version"] != "default" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestApplyGenerationMetadataMapPreservesExistingPromptValuesWhenGenerationFieldsEmpty(t *testing.T) {
	metadata := applyGenerationMetadataMap(map[string]string{
		"model_provider":    "openai_compatible",
		"model_family":      "nano-banana-fast",
		"generation_mode":   "scene_generation",
		"prompt_ref":        "productimage.scene.default",
		"prompt_key":        "productimage.scene.default",
		"prompt_source":     "registry",
		"prompt_version":    "default",
		"review_confidence": "0.75",
	}, &GenerationMetadata{})

	if metadata["model_provider"] != "openai_compatible" ||
		metadata["model_family"] != "nano-banana-fast" ||
		metadata["generation_mode"] != "scene_generation" ||
		metadata["prompt_ref"] != "productimage.scene.default" ||
		metadata["prompt_key"] != "productimage.scene.default" ||
		metadata["prompt_source"] != "registry" ||
		metadata["prompt_version"] != "default" {
		t.Fatalf("metadata = %+v", metadata)
	}
	if metadata["review_confidence"] != "0.75" {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestApplyGenerationMetadataMapPreservesExistingPromptValuesWhenGenerationFieldsPartial(t *testing.T) {
	metadata := applyGenerationMetadataMap(map[string]string{
		"prompt_ref":     "productimage.scene.default",
		"prompt_key":     "productimage.scene.default",
		"prompt_source":  "registry",
		"prompt_version": "default",
	}, &GenerationMetadata{
		PromptRef: "productimage.scene.default",
	})

	if metadata["prompt_ref"] != "productimage.scene.default" ||
		metadata["prompt_key"] != "productimage.scene.default" ||
		metadata["prompt_source"] != "registry" ||
		metadata["prompt_version"] != "default" {
		t.Fatalf("metadata = %+v", metadata)
	}
}
