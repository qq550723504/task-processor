package listingkit

import "strings"

func resolveRevisionValidationRequest(result *ListingKitResult, req *ApplyRevisionRequest) (*ApplyRevisionRequest, *RevisionRestorePreviewPayload, error) {
	if req == nil {
		return nil, nil, nil
	}
	restoreID := strings.TrimSpace(req.RestoreFromRevisionID)
	if restoreID == "" {
		return req, nil, nil
	}

	detail, err := buildRevisionHistoryDetail(result, restoreID, nil)
	if err != nil {
		return nil, nil, err
	}
	effective := cloneApplyRevisionRequest(req)
	if detail.RestorePayload != nil && detail.RestorePayload.Core != nil && detail.RestorePayload.Core.Draft != nil {
		applyRestoreDraftToRevisionRequest(effective, detail.RestorePayload.Core.Draft)
	}
	return effective, buildRevisionRestorePreviewFromDetail(detail), nil
}

func cloneApplyRevisionRequest(req *ApplyRevisionRequest) *ApplyRevisionRequest {
	if req == nil {
		return nil
	}
	cloned := &ApplyRevisionRequest{
		Platform:              req.Platform,
		Actor:                 req.Actor,
		Reason:                req.Reason,
		RestoreFromRevisionID: req.RestoreFromRevisionID,
		Amazon:                cloneAmazonRevisionInput(req.Amazon),
		Shein:                 cloneHistorySheinRevisionInput(req.Shein),
		Temu:                  cloneTemuRevisionInput(req.Temu),
		Walmart:               cloneWalmartRevisionInput(req.Walmart),
	}
	return cloned
}

func applyRestoreDraftToRevisionRequest(req *ApplyRevisionRequest, draft *SheinEditorRevisionSkeleton) {
	if req == nil || draft == nil {
		return
	}
	if draft.Platform != "" {
		req.Platform = draft.Platform
	}
	if draft.Actor != "" {
		req.Actor = draft.Actor
	}
	if draft.Reason != "" {
		req.Reason = draft.Reason
	}
	if draft.Shein != nil {
		req.Shein = cloneHistorySheinRevisionInput(draft.Shein)
	}
}

func cloneAmazonRevisionInput(src *AmazonRevisionInput) *AmazonRevisionInput {
	if src == nil {
		return nil
	}
	return &AmazonRevisionInput{
		Title:        cloneHistoryStringPointer(src.Title),
		Brand:        cloneHistoryStringPointer(src.Brand),
		BulletPoints: append([]string(nil), src.BulletPoints...),
		Description:  cloneHistoryStringPointer(src.Description),
	}
}

func cloneTemuRevisionInput(src *TemuRevisionInput) *TemuRevisionInput {
	if src == nil {
		return nil
	}
	return &TemuRevisionInput{
		GoodsName:        cloneHistoryStringPointer(src.GoodsName),
		ShortDescription: cloneHistoryStringPointer(src.ShortDescription),
		BulletPoints:     append([]string(nil), src.BulletPoints...),
		Images:           clonePlatformImageSetForEditor(src.Images),
		ReviewNotes:      append([]string(nil), src.ReviewNotes...),
	}
}

func cloneWalmartRevisionInput(src *WalmartRevisionInput) *WalmartRevisionInput {
	if src == nil {
		return nil
	}
	return &WalmartRevisionInput{
		ProductName:      cloneHistoryStringPointer(src.ProductName),
		Brand:            cloneHistoryStringPointer(src.Brand),
		ShortDescription: cloneHistoryStringPointer(src.ShortDescription),
		LongDescription:  cloneHistoryStringPointer(src.LongDescription),
		KeyFeatures:      append([]string(nil), src.KeyFeatures...),
		Images:           clonePlatformImageSetForEditor(src.Images),
		ReviewNotes:      append([]string(nil), src.ReviewNotes...),
	}
}
