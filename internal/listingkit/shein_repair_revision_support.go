package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {
	seed := sheinworkspace.BuildRepairRevisionSeed(action, payload)
	if seed.Input == nil || seed.Skeleton == nil {
		return sheinRepairRevisionBundle{}
	}
	return sheinRepairRevisionBundle{
		input:    seed.Input,
		skeleton: seed.Skeleton,
		request: &ApplyRevisionRequest{
			Platform: seed.Skeleton.Platform,
			Actor:    seed.Skeleton.Actor,
			Reason:   seed.Skeleton.Reason,
			Shein:    cloneHistorySheinRevisionInput(seed.Skeleton.Shein),
		},
	}
}

func buildSheinRepairArtifacts(pkg *SheinPackage, action string, editorSection string, patch *SheinRepairPatchPayload) sheinRepairArtifacts {
	bundle := buildSheinRepairRevisionBundle(action, patch)
	return sheinRepairArtifacts{
		patch:      sheinworkspace.CloneRepairPatchPayload(patch),
		skeleton:   bundle.skeleton,
		request:    bundle.request,
		validation: buildSheinRepairValidationPreview(pkg, editorSection, bundle.request, bundle.skeleton),
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
