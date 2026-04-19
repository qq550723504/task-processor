package productimage

import "strconv"

func applyGenerationMetadataMap(metadata map[string]string, generation *GenerationMetadata) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	if generation == nil {
		return metadata
	}
	metadata["model_provider"] = generation.Provider
	metadata["model_family"] = generation.ModelFamily
	metadata["generation_mode"] = generation.GenerationMode
	metadata["prompt_ref"] = generation.PromptRef
	metadata["prompt_key"] = generation.PromptKey
	metadata["prompt_source"] = generation.PromptSource
	metadata["prompt_version"] = generation.PromptVersion
	if generation.ReviewConfidence > 0 {
		metadata["review_confidence"] = strconv.FormatFloat(generation.ReviewConfidence, 'f', -1, 64)
	}
	return metadata
}
