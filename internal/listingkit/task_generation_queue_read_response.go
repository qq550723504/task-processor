package listingkit

import (
	"strconv"

	listinggeneration "task-processor/internal/listingkit/generation"
)

type taskGenerationQueueReadResponsePhase struct{}

func buildTaskGenerationQueueReadResponsePhase() *taskGenerationQueueReadResponsePhase {
	return &taskGenerationQueueReadResponsePhase{}
}

func (p *taskGenerationQueueReadResponsePhase) run(taskID string, page *GenerationQueuePage, query *GenerationQueueQuery) *GenerationQueuePage {
	if page == nil {
		page = &GenerationQueuePage{TaskID: taskID}
	}
	page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
	if query != nil && listinggeneration.IsReadNotModified(query.DeltaToken, query.IfMatch, page.DeltaToken) {
		return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
			TaskID:      page.TaskID,
			DeltaToken:  page.DeltaToken,
			NotModified: true,
			Page:        page.Page,
			PageSize:    page.PageSize,
			Total:       page.Total,
			UpdatedAt:   page.UpdatedAt,
		})
	}
	return applyGenerationConditionalStateToQueuePage(page)
}

func buildGenerationQueueDeltaToken(page *GenerationQueuePage, query *GenerationQueueQuery) string {
	if page == nil {
		return ""
	}
	summarySig := ""
	if page.Summary != nil {
		summarySig = fmtRenderSummarySignature(page.Summary)
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

func fmtRenderSummarySignature(summary *GenerationWorkQueueSummary) string {
	if summary == nil {
		return ""
	}
	return hashRenderRevision(
		strconv.Itoa(summary.TotalItems),
		strconv.Itoa(summary.ReadyItems),
		strconv.Itoa(summary.FallbackItems),
		strconv.Itoa(summary.MissingItems),
		strconv.Itoa(summary.QueuedItems),
		strconv.Itoa(summary.RunningItems),
		strconv.Itoa(summary.CompletedItems),
		strconv.Itoa(summary.FailedItems),
		strconv.Itoa(summary.StubbedItems),
		strconv.Itoa(summary.RetryableItems),
		strconv.Itoa(summary.PreviewableItems),
		strconv.Itoa(summary.ApprovedSections),
		strconv.Itoa(summary.DeferredSections),
		strconv.Itoa(summary.ReviewPendingSections),
	)
}
