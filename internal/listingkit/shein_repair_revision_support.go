package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview] {
	seed := sheinworkspace.BuildRepairRevisionSeed(action, patch)
	var request *ApplyRevisionRequest
	if seed.Input != nil && seed.Skeleton != nil {
		request = &ApplyRevisionRequest{
			Platform: seed.Skeleton.Platform,
			Actor:    seed.Skeleton.Actor,
			Reason:   seed.Skeleton.Reason,
			Shein:    sheinworkspace.CloneRevisionInput(seed.Skeleton.Shein),
		}
	}
	var validation *SheinRepairValidationPreview
	if request != nil && seed.Skeleton != nil && seed.Skeleton.Shein != nil {
		valid := true
		var fieldErrors []RevisionFieldError
		if validationErr, ok := validateApplyRevisionRequest(request).(*RevisionValidationError); ok {
			valid = false
			fieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
		}
		validation = sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, seed.Skeleton, valid, fieldErrors)
	}
	return sheinworkspace.RepairArtifacts[SheinRepairPatchPayload, SheinEditorRevisionSkeleton, ApplyRevisionRequest, SheinRepairValidationPreview]{
		Patch:      sheinworkspace.CloneRepairPatchPayload(patch),
		Skeleton:   seed.Skeleton,
		Request:    request,
		Validation: validation,
	}
}
