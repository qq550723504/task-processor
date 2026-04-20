package productimage

import "strconv"

func applyGenerationMetadataMap(metadata map[string]string, generation *GenerationMetadata) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	if generation == nil {
		return metadata
	}
	setGenerationMetadataValue(metadata, "model_provider", generation.Provider)
	setGenerationMetadataValue(metadata, "model_family", generation.ModelFamily)
	setGenerationMetadataValue(metadata, "generation_mode", generation.GenerationMode)
	setGenerationMetadataValue(metadata, "prompt_ref", generation.PromptRef)
	setGenerationMetadataValue(metadata, "prompt_key", generation.PromptKey)
	setGenerationMetadataValue(metadata, "prompt_source", generation.PromptSource)
	setGenerationMetadataValue(metadata, "prompt_version", generation.PromptVersion)
	setGenerationMetadataValue(metadata, "scene_defaults_source", generation.SceneDefaultsSource)
	setGenerationMetadataValue(metadata, "scene_category", generation.SceneCategory)
	setGenerationMetadataValue(metadata, "scene_style", generation.SceneStyle)
	setGenerationMetadataValue(metadata, "background_tone", generation.BackgroundTone)
	setGenerationMetadataValue(metadata, "composition", generation.Composition)
	setGenerationMetadataValue(metadata, "props_level", generation.PropsLevel)
	setGenerationMetadataValue(metadata, "audience_hint", generation.AudienceHint)
	setGenerationMetadataValue(metadata, "custom_scene_hint", generation.CustomSceneHint)
	if generation.ReviewConfidence > 0 {
		metadata["review_confidence"] = strconv.FormatFloat(generation.ReviewConfidence, 'f', -1, 64)
	}
	return metadata
}

func setGenerationMetadataValue(metadata map[string]string, key string, value string) {
	if metadata == nil || value == "" {
		return
	}
	metadata[key] = value
}
