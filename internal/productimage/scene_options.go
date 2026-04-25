package productimage

import (
	"strings"

	"task-processor/internal/productimage/domain"
)

type SceneGenerationOptions = domain.SceneGenerationOptions

func DefaultSceneGenerationOptionsForMarketplace(marketplace string) *SceneGenerationOptions {
	return resolveScenePreset(marketplace, "").Options
}

func MergeSceneGenerationOptions(base, override *SceneGenerationOptions) *SceneGenerationOptions {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		return override.Clone()
	}

	merged := base.Clone()
	if override == nil {
		return merged
	}
	if value := strings.TrimSpace(override.SceneCategory); value != "" {
		merged.SceneCategory = value
	}
	if value := strings.TrimSpace(override.SceneStyle); value != "" {
		merged.SceneStyle = value
	}
	if value := strings.TrimSpace(override.BackgroundTone); value != "" {
		merged.BackgroundTone = value
	}
	if value := strings.TrimSpace(override.Composition); value != "" {
		merged.Composition = value
	}
	if value := strings.TrimSpace(override.PropsLevel); value != "" {
		merged.PropsLevel = value
	}
	if value := strings.TrimSpace(override.AudienceHint); value != "" {
		merged.AudienceHint = value
	}
	if value := strings.TrimSpace(override.CustomSceneHint); value != "" {
		merged.CustomSceneHint = value
	}
	return merged
}
