package listingkit

func buildSheinRepairRevisionBundle(action string, payload *SheinRepairPatchPayload) sheinRepairRevisionBundle {
	input := buildSheinRepairRevisionInput(payload)
	if input == nil {
		return sheinRepairRevisionBundle{}
	}
	minimal := pruneSheinRevisionInput(input)
	if minimal == nil || isEmptySheinRevisionInput(minimal) {
		return sheinRepairRevisionBundle{}
	}
	skeleton := &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildSheinRepairReason(action),
		Shein:    minimal,
	}
	return sheinRepairRevisionBundle{
		input:    input,
		skeleton: skeleton,
		request: &ApplyRevisionRequest{
			Platform: skeleton.Platform,
			Actor:    skeleton.Actor,
			Reason:   skeleton.Reason,
			Shein:    cloneHistorySheinRevisionInput(skeleton.Shein),
		},
	}
}

func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {
	return buildSheinRepairRevisionBundle(action, payload).skeleton
}

func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {
	return buildSheinRepairRevisionBundle(action, payload).request
}

func buildSheinRepairRevisionInput(payload *SheinRepairPatchPayload) *SheinRevisionInput {
	if payload == nil {
		return nil
	}
	input := &SheinRevisionInput{
		CategoryResolution:      cloneSheinCategoryResolutionPatch(payload.CategoryResolution),
		AttributeResolution:     cloneSheinAttributeResolutionPatch(payload.AttributeResolution),
		SaleAttributeResolution: cloneSheinSaleAttributeResolutionPatch(payload.SaleAttributeResolution),
		SKCPatches:              cloneSheinSKCRevisionPatches(payload.SKCPatches),
		Images:                  clonePlatformImageSetForEditor(payload.Images),
		ReviewNotes:             append([]string(nil), payload.ReviewNotes...),
	}
	if isEmptySheinRevisionInput(input) {
		return nil
	}
	return input
}

func buildSheinRepairReason(action string) string {
	if action == "" {
		return "repair suggested issue"
	}
	return "repair: " + action
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
