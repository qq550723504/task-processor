package listingkit

func buildSheinRepairRevisionSkeleton(action string, payload *SheinRepairPatchPayload) *SheinEditorRevisionSkeleton {
	input := buildSheinRepairRevisionInput(payload)
	if input == nil {
		return nil
	}
	minimal := pruneSheinRevisionInput(input)
	if minimal == nil || isEmptySheinRevisionInput(minimal) {
		return nil
	}
	return &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   buildSheinRepairReason(action),
		Shein:    minimal,
	}
}

func buildSheinRepairApplyRequest(action string, payload *SheinRepairPatchPayload) *ApplyRevisionRequest {
	skeleton := buildSheinRepairRevisionSkeleton(action, payload)
	if skeleton == nil {
		return nil
	}
	return &ApplyRevisionRequest{
		Platform: skeleton.Platform,
		Actor:    skeleton.Actor,
		Reason:   skeleton.Reason,
		Shein:    cloneHistorySheinRevisionInput(skeleton.Shein),
	}
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
