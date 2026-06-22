package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinRepairApplyRequest(seed sheinworkspace.RepairRevisionSeed) *ApplyRevisionRequest {
	if seed.Input == nil || seed.Skeleton == nil {
		return nil
	}
	return &ApplyRevisionRequest{
		Platform: seed.Skeleton.Platform,
		Actor:    seed.Skeleton.Actor,
		Reason:   seed.Skeleton.Reason,
		Shein:    sheinworkspace.CloneRevisionInput(seed.Skeleton.Shein),
	}
}

func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {
	seed := sheinworkspace.BuildRepairRevisionSeed(action, patch)
	request := buildSheinRepairApplyRequest(seed)
	return sheinRepairArtifacts{
		Patch:      sheinworkspace.CloneRepairPatchPayload(patch),
		Skeleton:   seed.Skeleton,
		Request:    request,
		Validation: buildSheinRepairValidationPreview(pkg, editorSection, request, seed.Skeleton),
	}
}

func buildSheinRepairValidationPreview(pkg *SheinPackage, editorSection string, revision *ApplyRevisionRequest, skeleton *SheinEditorRevisionSkeleton) *SheinRepairValidationPreview {
	if revision == nil || skeleton == nil || skeleton.Shein == nil {
		return nil
	}
	valid := true
	var fieldErrors []RevisionFieldError
	if validationErr, ok := validateApplyRevisionRequest(revision).(*RevisionValidationError); ok {
		valid = false
		fieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
	}
	return sheinworkspace.BuildRepairValidationPreview(pkg, editorSection, skeleton, valid, fieldErrors)
}
