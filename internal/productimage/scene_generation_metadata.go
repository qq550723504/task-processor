package productimage

func applySceneGenerationMetadata(metadata map[string]string, options scenePromptOptions) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["scene_defaults_source"] = firstNonEmpty(options.DefaultsSource, "fallback")
	setGenerationMetadataValue(metadata, "scene_category", options.Category)
	setGenerationMetadataValue(metadata, "scene_style", options.SceneStyle)
	setGenerationMetadataValue(metadata, "background_tone", options.BackgroundTone)
	setGenerationMetadataValue(metadata, "composition", options.Composition)
	setGenerationMetadataValue(metadata, "props_level", options.PropsLevel)
	setGenerationMetadataValue(metadata, "audience_hint", options.AudienceHint)
	setGenerationMetadataValue(metadata, "custom_scene_hint", options.CustomSceneHint)
	return metadata
}

func sceneGenerationMetadataFromOptions(options scenePromptOptions, resolved resolvedProductImagePrompt, provider, modelFamily, generationMode string) *GenerationMetadata {
	return &GenerationMetadata{
		Provider:            provider,
		ModelFamily:         modelFamily,
		GenerationMode:      generationMode,
		PromptRef:           resolved.Key,
		PromptKey:           resolved.Key,
		PromptSource:        resolved.Source,
		PromptVersion:       resolved.Version,
		SceneDefaultsSource: firstNonEmpty(options.DefaultsSource, "fallback"),
		SceneCategory:       options.Category,
		SceneStyle:          options.SceneStyle,
		BackgroundTone:      options.BackgroundTone,
		Composition:         options.Composition,
		PropsLevel:          options.PropsLevel,
		AudienceHint:        options.AudienceHint,
		CustomSceneHint:     options.CustomSceneHint,
	}
}

