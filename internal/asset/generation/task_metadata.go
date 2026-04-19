package generation

import (
	"strconv"
	"strings"

	"task-processor/internal/productimage"
)

func cloneTaskMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func taskMetadataFromAssetMetadata(src map[string]string) map[string]string {
	dst := cloneTaskMetadata(src)
	attachGenerationMetadataMap(&dst, generationMetadataFromMetadataMap(src))
	return dst
}

func reviewConfidenceFromMetadata(metadata map[string]string) float64 {
	if len(metadata) == 0 {
		return 0
	}
	raw := metadata["review_confidence"]
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}

func AttachGenerationMetadata(task *Task, metadata *productimage.GenerationMetadata) {
	if task == nil {
		return
	}
	attachGenerationMetadataMap(&task.Metadata, metadata)
}

func attachGenerationMetadataMap(target *map[string]string, metadata *productimage.GenerationMetadata) {
	if target == nil || metadata == nil {
		return
	}
	dst := *target
	if dst == nil {
		dst = map[string]string{}
	}
	setTaskMetadataValue(dst, "model_provider", metadata.Provider)
	setTaskMetadataValue(dst, "model_family", metadata.ModelFamily)
	setTaskMetadataValue(dst, "generation_mode", metadata.GenerationMode)
	setTaskMetadataValue(dst, "prompt_ref", metadata.PromptRef)
	setTaskMetadataValue(dst, "prompt_key", metadata.PromptKey)
	setTaskMetadataValue(dst, "prompt_source", metadata.PromptSource)
	setTaskMetadataValue(dst, "prompt_version", metadata.PromptVersion)
	*target = dst
}

func generationMetadataFromMetadataMap(metadata map[string]string) *productimage.GenerationMetadata {
	if len(metadata) == 0 {
		return nil
	}
	result := &productimage.GenerationMetadata{
		Provider:       strings.TrimSpace(metadata["model_provider"]),
		ModelFamily:    strings.TrimSpace(metadata["model_family"]),
		GenerationMode: strings.TrimSpace(metadata["generation_mode"]),
		PromptRef:      strings.TrimSpace(metadata["prompt_ref"]),
		PromptKey:      strings.TrimSpace(metadata["prompt_key"]),
		PromptSource:   strings.TrimSpace(metadata["prompt_source"]),
		PromptVersion:  strings.TrimSpace(metadata["prompt_version"]),
	}
	if result.Provider == "" &&
		result.ModelFamily == "" &&
		result.GenerationMode == "" &&
		result.PromptRef == "" &&
		result.PromptKey == "" &&
		result.PromptSource == "" &&
		result.PromptVersion == "" {
		return nil
	}
	return result
}

func setTaskMetadataValue(metadata map[string]string, key string, value string) {
	if metadata == nil || strings.TrimSpace(key) == "" {
		return
	}
	if strings.TrimSpace(value) == "" {
		return
	}
	metadata[key] = value
}
