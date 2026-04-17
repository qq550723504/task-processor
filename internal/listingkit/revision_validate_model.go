package listingkit

type RevisionValidationResult struct {
	TaskID      string                          `json:"task_id,omitempty"`
	Platform    string                          `json:"platform,omitempty"`
	Valid       bool                            `json:"valid"`
	FieldErrors []RevisionFieldError            `json:"field_errors,omitempty"`
	Shein       *SheinRevisionValidationPayload `json:"shein,omitempty"`
}

type SheinRevisionValidationPayload struct {
	DirtyHints                  *SheinEditorDirtyHints         `json:"dirty_hints,omitempty"`
	CategoryPreviewEffects      []SheinEditorEffect            `json:"category_preview_effects,omitempty"`
	AttributePreviewEffects     []SheinEditorEffect            `json:"attribute_preview_effects,omitempty"`
	SaleAttributePreviewEffects []SheinEditorEffect            `json:"sale_attribute_preview_effects,omitempty"`
	SuggestedMinimalRevision    *SheinEditorRevisionSkeleton   `json:"suggested_minimal_revision,omitempty"`
	RevisionDiffPreview         *RevisionDiffPreview           `json:"revision_diff_preview,omitempty"`
	RestorePreview              *RevisionRestorePreviewPayload `json:"restore_preview,omitempty"`
}

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
		minimal := buildSheinMinimalRevisionSkeleton(result.Shein)
		output.Shein = &SheinRevisionValidationPayload{
			DirtyHints:                  buildSheinEditorDirtyHints(result.Shein),
			CategoryPreviewEffects:      buildSheinCategoryEffects(),
			AttributePreviewEffects:     buildSheinAttributeEffects(),
			SaleAttributePreviewEffects: buildSheinSaleAttributeEffects(),
			SuggestedMinimalRevision:    minimal,
			RevisionDiffPreview:         buildSheinRevisionDiffPreview(result.Shein, minimal),
			RestorePreview:              restorePreview,
		}
	}
	return output
}
