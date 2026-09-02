package listingkit

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func (s *service) runPlatformAdaptation(
	ctx context.Context,
	task *Task,
	snapshot *StandardProductSnapshot,
) *ListingKitResult {
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/platform_adaptation",
		"task_id":   task.ID,
	})

	if shouldSkipPlatformAdaptationAfterBlockedRemoteSDS(task, snapshot) {
		log.WithField("pod_status", func() string {
			if snapshot == nil || snapshot.PodExecution == nil {
				return ""
			}
			return snapshot.PodExecution.Status
		}()).Warn("skipping platform adaptation because required remote SDS render failed")
		final := initResult(task)
		applyStandardProductSnapshot(final, snapshot)
		final = normalizeListingKitResultSemanticFields(final)
		final.Summary = ensureGenerationSummary(final.Summary)
		if final.PodExecution != nil && strings.TrimSpace(final.PodExecution.FailureReason) != "" {
			reason := strings.TrimSpace(final.PodExecution.FailureReason)
			final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, reason))
			final.ReviewReasons = uniqueStrings(append(final.ReviewReasons, reason))
		}
		final.Summary.NeedsReview = true
		newWorkflowRecorder(final).FinalizeSummary()
		return final
	}

	var productSnapshot *catalog.ProductSnapshot
	var approvedAssets *productasset.ApprovedAssetInventory
	if snapshot != nil {
		productSnapshot = snapshot.CatalogProduct
		approvedAssets = snapshot.ApprovedAssetInventory
	}

	log.Info("starting listing kit platform adaptation")
	assembler := resolveAssembler(s)
	var final *ListingKitResult
	if targetAware, ok := assembler.(TargetAwareAssembler); ok {
		final = targetAware.AssembleForTargets(task, productSnapshot, approvedAssets)
	} else {
		final = assembler.Assemble(task, productSnapshot, approvedAssets)
	}
	if final == nil {
		final = initResult(task)
	}
	applyStandardProductSnapshot(final, snapshot)
	final = buildPlatformFinalizePhase(s).run(ctx, task, final, snapshot)
	log.WithFields(logrus.Fields{
		"needs_review": final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": func() int {
			if final.Summary == nil {
				return 0
			}
			return final.Summary.WarningCount
		}(),
	}).Info("listing kit platform adaptation finished")
	return final
}

func shouldSkipPlatformAdaptationAfterBlockedRemoteSDS(task *Task, snapshot *StandardProductSnapshot) bool {
	if task == nil || !shouldRunRemoteSDSDesignSync(task.Request) || snapshot == nil {
		return false
	}
	return podSubmissionBlocked(snapshot.PodExecution)
}
