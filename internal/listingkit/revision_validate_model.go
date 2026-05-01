package listingkit

import sheinworkspace "task-processor/internal/workspace/shein"

type RevisionValidationResult struct {
	TaskID       string                          `json:"task_id,omitempty"`
	Platform     string                          `json:"platform,omitempty"`
	Valid        bool                            `json:"valid"`
	FieldErrors  []RevisionFieldError            `json:"field_errors,omitempty"`
	ScenePresets []PlatformScenePresetSummary    `json:"scene_presets,omitempty"`
	Shein        *SheinRevisionValidationPayload `json:"shein,omitempty"`
}

type SheinRevisionValidationPayload = sheinworkspace.ValidationPayload[RevisionRestorePreviewPayload]

func buildRevisionValidationResult(taskID, platform string, result *ListingKitResult, err error, restorePreview *RevisionRestorePreviewPayload) *RevisionValidationResult {
	output := &RevisionValidationResult{
		TaskID:   taskID,
		Platform: platform,
		Valid:    err == nil,
	}
	if validationErr, ok := err.(*RevisionValidationError); ok {
		output.FieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
	}
	if result != nil && platform == "shein" && result.Shein != nil {
		output.ScenePresets = buildPlatformScenePresetSummaries(result.Shein.ImageBundle, result.AssetBundle)
		output.Shein = sheinworkspace.BuildValidationPayload(result.Shein, restorePreview)
	}
	return output
}
