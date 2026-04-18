package listingkit

import (
	"fmt"
	"strconv"
)

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

func buildGenerationQueueDeltaToken(page *GenerationQueuePage, query *GenerationQueueQuery) string {
	if page == nil {
		return ""
	}
	summarySig := ""
	if page.Summary != nil {
		summarySig = fmt.Sprintf(
			"%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d:%d",
			page.Summary.TotalItems,
			page.Summary.ReadyItems,
			page.Summary.FallbackItems,
			page.Summary.MissingItems,
			page.Summary.QueuedItems,
			page.Summary.RunningItems,
			page.Summary.CompletedItems,
			page.Summary.FailedItems,
			page.Summary.StubbedItems,
			page.Summary.RetryableItems,
			page.Summary.PreviewableItems,
			page.Summary.ApprovedSections,
			page.Summary.ReviewPendingSections,
		)
	}
	itemSig := ""
	for _, item := range page.Items {
		itemSig += hashRenderRevision(
			item.TaskID,
			item.Platform,
			item.Slot,
			item.State,
			item.ExecutionMode,
			item.ExecutionQuality,
			item.QualityGrade,
			item.AssetID,
			item.ReviewDecision,
			item.ReviewStatus,
			strconv.FormatBool(item.RenderPreviewAvailable),
		)
	}
	querySig := ""
	if query != nil {
		querySig = hashRenderRevision(
			query.Platform,
			query.Slot,
			query.State,
			query.ExecutionMode,
			query.ExecutionQuality,
			query.QualityGrade,
			query.QualityGradeLabel,
			query.PreviewCapability,
			query.SortBy,
			query.SortOrder,
			strconv.Itoa(resolveGenerationQueuePage(query)),
			strconv.Itoa(resolveGenerationQueuePageSize(query)),
			strconv.FormatBool(query.RenderPreviewAvailablePresent && query.RenderPreviewAvailable),
			strconv.FormatBool(query.RetryablePresent && query.Retryable),
		)
	}
	return hashRenderRevision(
		page.TaskID,
		page.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		strconv.Itoa(page.Total),
		summarySig,
		itemSig,
		querySig,
	)
}
