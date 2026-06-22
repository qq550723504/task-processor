package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

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
