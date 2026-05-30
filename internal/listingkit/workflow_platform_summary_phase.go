package listingkit

import "github.com/sirupsen/logrus"

type platformSummaryPhase struct{}

func buildPlatformSummaryPhase() *platformSummaryPhase {
	return &platformSummaryPhase{}
}

func (p *platformSummaryPhase) prepareReview(
	final *ListingKitResult,
	snapshot *StandardProductSnapshot,
) {
	buildPlatformReviewPhase().run(final, snapshot)
}

func (p *platformSummaryPhase) complete(task *Task, final *ListingKitResult) *ListingKitResult {
	newWorkflowRecorder(final).FinalizeSummary()
	syncAssetRenderPreviews(final)
	logrus.WithFields(logrus.Fields{
		"component":     "listingkit/platform_adaptation_finalize",
		"task_id":       task.ID,
		"needs_review":  final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": processWarningCount(final),
	}).Info("listing kit platform adaptation finalized")
	return final
}

func (p *platformSummaryPhase) run(
	task *Task,
	final *ListingKitResult,
	snapshot *StandardProductSnapshot,
) *ListingKitResult {
	p.prepareReview(final, snapshot)
	return p.complete(task, final)
}
