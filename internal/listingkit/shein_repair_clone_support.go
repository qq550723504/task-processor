package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

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
		Patch:      sheinworkspace.CloneRepairPatchPayload(patch),
		Skeleton:   sheinworkspace.CloneEditorRevisionSkeleton(skeleton),
		Request:    cloneApplyRevisionRequest(request),
		Validation: cloneSheinRepairValidationPreview(validation),
	}
}

func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {
	return sheinworkspace.CloneRepairValidationPreview(src)
}
