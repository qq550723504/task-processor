package listingkit

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	productasset "task-processor/internal/product/asset"
)

type standardWorkflowState struct {
	result   *ListingKitResult
	snapshot *StandardProductSnapshot
	blocked  bool
}

const (
	standardProductReadinessBlockReason  = "standard_product_readiness_pending"
	standardProductReadinessBlockMessage = "standard product inputs are not ready"
)

func (s *service) runStandardProductWorkflow(ctx context.Context, task *Task) (*standardWorkflowState, error) {
	result := initResult(task)
	recorder := newWorkflowRecorder(result)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/workflow_standard",
		"task_id":   task.ID,
	})

	stage := recorder.Start(productSnapshotStageKind, "")
	productSnapshot, err := buildStandardWorkflowCanonicalPhase(s).run(ctx, productSnapshotQueryForTask(task))
	if err != nil {
		if errors.Is(err, ErrProductSnapshotNotReady) {
			stage.Fail(productSnapshotNotReadyIssueCode, productSnapshotNotReadyMessage, err.Error())
			recorder.FinalizeSummary()
			snapshot := buildStandardProductSnapshot(result)
			result.StandardProductSnapshot = snapshot
			return &standardWorkflowState{result: result, snapshot: snapshot, blocked: true}, nil
		}
		stage.Fail("product_snapshot_read_failed", "Product snapshot could not be read", err.Error())
		recorder.FinalizeSummary()
		return &standardWorkflowState{result: result}, err
	}
	stage.Complete()

	result.CatalogProduct = &productSnapshot
	canonicalProduct := canonicalProductFromSnapshot(productSnapshot)
	result.CanonicalProduct = canonicalProduct
	if productSnapshot.Review != nil {
		result.ReviewReasons = append(result.ReviewReasons, productSnapshot.Review.Reasons...)
	}
	log.WithFields(logrus.Fields{
		"has_canonical": canonicalProduct != nil,
		"image_count": func() int {
			if canonicalProduct == nil {
				return 0
			}
			return len(canonicalProduct.Images)
		}(),
		"variant_count": func() int {
			if canonicalProduct == nil {
				return 0
			}
			return len(canonicalProduct.Variants)
		}(),
	}).Info("canonical product prepared for listing kit workflow")
	if persistErr := s.persistSDSBaselineIfEligible(ctx, task); persistErr != nil {
		log.WithError(persistErr).Warn("sds baseline persistence failed")
	} else if validationErr := s.persistSDSBaselineValidation(ctx, task); validationErr != nil {
		log.WithError(validationErr).Warn("sds baseline validation persistence failed")
	}

	assetStage := recorder.Start("approved_assets", "")
	approvedInventory, assetErr := buildStandardWorkflowAssetPhase(s).run(ctx, productasset.InventoryScope{
		TenantID: task.TenantID, ProductKey: task.Request.ProductKey, SourceSnapshotVersion: task.SourceSnapshotVersion,
	})
	if assetErr != nil {
		if errors.Is(assetErr, productasset.ErrApprovedAssetsNotReady) {
			assetStage.Fail("approved_assets_not_ready", "Approved product assets are not ready", assetErr.Error())
			recorder.FinalizeSummary()
			snapshot := buildStandardProductSnapshot(result)
			result.StandardProductSnapshot = snapshot
			return &standardWorkflowState{result: result, snapshot: snapshot, blocked: true}, nil
		}
		assetStage.Fail("approved_assets_read_failed", "Approved product assets could not be read", assetErr.Error())
		recorder.FinalizeSummary()
		return &standardWorkflowState{result: result}, assetErr
	}
	assetStage.Complete()
	result.ApprovedAssetInventory = &approvedInventory

	recorder.FinalizeSummary()
	snapshot := buildStandardProductSnapshot(result)
	result.StandardProductSnapshot = snapshot
	return &standardWorkflowState{
		result:   result,
		snapshot: snapshot,
	}, nil
}
