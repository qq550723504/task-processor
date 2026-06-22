package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {
	seed := listingworkspace.BuildRepairRevisionSeed(action, payload)
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
		patch:      cloneSheinRepairPatchPayload(patch),
		skeleton:   bundle.skeleton,
		request:    bundle.request,
		validation: buildSheinRepairValidationPreview(pkg, editorSection, bundle.request, bundle.skeleton),
	}
}
