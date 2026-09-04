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
) (*ListingKitResult, error) {
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/platform_adaptation",
		"task_id":   task.ID,
	})

	if shouldSkipPlatformAdaptationAfterBlockedSDS(task, snapshot) {
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
		return final, nil
	}

	var productSnapshot *catalog.ProductSnapshot
	var approvedAssets *productasset.ApprovedAssetInventory
	var approvedInventories map[string]productasset.ApprovedAssetInventory
	if snapshot != nil {
		productSnapshot = snapshot.CatalogProduct
		approvedAssets = snapshot.ApprovedAssetInventory
		approvedInventories = snapshot.ApprovedAssetInventories
	}

	log.Info("starting listing kit platform adaptation")
	assembler := resolveAssembler(s)
	var final *ListingKitResult
	var err error
	selectedPlatforms := selectedInventoryPlatforms(task)
	if len(selectedPlatforms) == 1 && len(approvedInventories) > 0 {
		inventory, ok := approvedInventories[selectedPlatforms[0]]
		if !ok {
			return nil, productasset.ErrApprovedAssetsNotReady
		}
		approvedAssets = &inventory
		if targetAware, ok := assembler.(TargetAwareAssembler); ok {
			final, err = targetAware.AssembleForTargets(task, productSnapshot, approvedAssets)
		} else {
			final, err = assembler.Assemble(task, productSnapshot, approvedAssets)
		}
	} else if len(approvedInventories) > 1 && len(selectedPlatforms) > 1 {
		final, err = assembleTargetSpecificPlatforms(assembler, task, productSnapshot, approvedInventories)
	} else if targetAware, ok := assembler.(TargetAwareAssembler); ok {
		final, err = targetAware.AssembleForTargets(task, productSnapshot, approvedAssets)
	} else {
		final, err = assembler.Assemble(task, productSnapshot, approvedAssets)
	}
	if err != nil {
		return nil, err
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
	return final, nil
}

func assembleTargetSpecificPlatforms(assembler Assembler, task *Task, product *catalog.ProductSnapshot, inventories map[string]productasset.ApprovedAssetInventory) (*ListingKitResult, error) {
	if assembler == nil || task == nil || task.Request == nil {
		return nil, ErrTaskResultUnavailable
	}
	platforms := selectedInventoryPlatforms(task)
	combined := initResult(task)
	combined.ApprovedAssetInventories = cloneApprovedAssetInventories(inventories)
	for index, platform := range platforms {
		inventory, ok := inventories[platform]
		if !ok {
			return nil, productasset.ErrApprovedAssetsNotReady
		}
		targetTask := *task
		targetRequest := *task.Request
		targetRequest.Platforms = []string{platform}
		targetTask.Request = &targetRequest
		var assembled *ListingKitResult
		var err error
		if targetAware, ok := assembler.(TargetAwareAssembler); ok {
			assembled, err = targetAware.AssembleForTargets(&targetTask, product, &inventory)
		} else {
			assembled, err = assembler.Assemble(&targetTask, product, &inventory)
		}
		if err != nil {
			return nil, err
		}
		if assembled == nil {
			return nil, ErrTaskResultUnavailable
		}
		if index == 0 {
			combined.CatalogProduct = assembled.CatalogProduct
			combined.CanonicalProduct = assembled.CanonicalProduct
			combined.Summary = assembled.Summary
		}
		combined.ReviewReasons = append(combined.ReviewReasons, assembled.ReviewReasons...)
		switch platform {
		case "amazon":
			combined.Amazon = assembled.Amazon
		case "shein":
			combined.Shein = assembled.Shein
		case "temu":
			combined.Temu = assembled.Temu
		case "walmart":
			combined.Walmart = assembled.Walmart
		}
	}
	if len(platforms) == 1 {
		inventory := inventories[platforms[0]]
		combined.ApprovedAssetInventory = &inventory
	}
	return combined, nil
}

func shouldSkipPlatformAdaptationAfterBlockedSDS(task *Task, snapshot *StandardProductSnapshot) bool {
	if task == nil || !shouldRunSDSDesignSync(task.Request) || snapshot == nil {
		return false
	}
	return podSubmissionBlocked(snapshot.PodExecution)
}
