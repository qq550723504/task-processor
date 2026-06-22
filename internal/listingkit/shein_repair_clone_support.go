package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

func clonePlatformImageSetForEditor(set *PlatformImageSet) *PlatformImageSet {
	if set == nil {
		return nil
	}
	return &PlatformImageSet{
		MainImage:    set.MainImage,
		WhiteBgImage: set.WhiteBgImage,
		Gallery:      append([]string(nil), set.Gallery...),
		SourceImages: append([]string(nil), set.SourceImages...),
	}
}

func cloneSheinRepairArtifacts(patch *SheinRepairPatchPayload, skeleton *SheinEditorRevisionSkeleton, request *ApplyRevisionRequest, validation *SheinRepairValidationPreview) sheinRepairArtifacts {
	return sheinRepairArtifacts{
		patch:      listingworkspace.CloneRepairPatchPayload(patch),
		skeleton:   cloneSheinEditorRevisionSkeleton(skeleton),
		request:    cloneApplyRevisionRequest(request),
		validation: cloneSheinRepairValidationPreview(validation),
	}
}

func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {
	if src == nil {
		return nil
	}
	return &SheinRepairValidationPreview{
		Valid:                       src.Valid,
		Status:                      src.Status,
		FieldErrors:                 append([]RevisionFieldError(nil), src.FieldErrors...),
		RevisionDiffPreview:         cloneRevisionDiffPreview(src.RevisionDiffPreview),
		AffectedSections:            append([]string(nil), src.AffectedSections...),
		CategoryPreviewEffects:      append([]SheinEditorEffect(nil), src.CategoryPreviewEffects...),
		AttributePreviewEffects:     append([]SheinEditorEffect(nil), src.AttributePreviewEffects...),
		SaleAttributePreviewEffects: append([]SheinEditorEffect(nil), src.SaleAttributePreviewEffects...),
	}
}

func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {
	if src == nil {
		return nil
	}
	cloned := &RevisionDiffPreview{
		ChangeCount: src.ChangeCount,
	}
	if len(src.Changes) > 0 {
		cloned.Changes = append([]RevisionFieldChange(nil), src.Changes...)
	}
	return cloned
}
