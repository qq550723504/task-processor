package listingkit

import "fmt"

func buildGenerationReviewDeltaToken(session *GenerationReviewSession) string {
	if session == nil {
		return ""
	}
	taskRevision := ""
	assetID := ""
	assetRevision := ""
	previewRevision := ""
	if session.FocusedRenderPreview != nil {
		assetID = session.FocusedRenderPreview.AssetID
		assetRevision = session.FocusedRenderPreview.AssetRevision
		previewRevision = session.FocusedRenderPreview.PreviewRevision
		taskRevision = session.FocusedRenderPreview.TaskRevision
	}
	if taskRevision == "" && session.FocusedTarget != nil {
		taskRevision = session.FocusedTarget.TaskRevision
		if assetID == "" {
			assetID = session.FocusedTarget.AssetID
		}
		if assetRevision == "" {
			assetRevision = session.FocusedTarget.AssetRevision
		}
		if previewRevision == "" {
			previewRevision = session.FocusedTarget.PreviewRevision
		}
	}
	approved := 0
	deferred := 0
	pending := 0
	if session.ReviewSummary != nil {
		approved = session.ReviewSummary.ApprovedSections
		deferred = session.ReviewSummary.DeferredSections
		pending = session.ReviewSummary.ReviewPendingSections
	}
	return hashRenderRevision(
		taskRevision,
		session.SelectedPlatform,
		session.SelectedSlot,
		session.FocusCapability,
		session.FocusedSectionKey,
		assetID,
		assetRevision,
		previewRevision,
		fmt.Sprintf("%d:%d:%d", approved, deferred, pending),
	)
}

func normalizeGenerationActionResponseMode(mode string) string {
	switch mode {
	case "patch_only":
		return "patch_only"
	default:
		return "full"
	}
}
