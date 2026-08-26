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
	if value := strings.TrimSpace(override.SlotRole); value != "" {
		merged.SlotRole = value
	}
	if value := strings.TrimSpace(override.SlotBrief); value != "" {
		merged.SlotBrief = value
	}
	if styleReferenceIDs := normalizedSceneStyleReferenceIDs(override.StyleReferenceIDs); len(styleReferenceIDs) > 0 {
		merged.StyleReferenceIDs = styleReferenceIDs
	}
	return merged
}

func normalizedSceneStyleReferenceIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}
