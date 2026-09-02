package listingkit

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type standardWorkflowState struct {
	result                   *ListingKitResult
	snapshot                 *StandardProductSnapshot
	recipesByPlatform        map[string][]assetrecipe.AssetRecipe
	generationPlan           *assetgeneration.Result
	inventory                *asset.Inventory
	persistedGenerationTasks []assetgeneration.Task
	enableAssetGeneration    bool
	sdsOptions               *SDSSyncOptions
	blocked                  bool
}

func (s *service) runStandardProductWorkflow(ctx context.Context, task *Task) (*standardWorkflowState, error) {
	result := initResult(task)
	recorder := newWorkflowRecorder(result)
	enableAssetGeneration := shouldGenerateAssets(task.Request)
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
	if !shouldProcessImages(task.Request) {
		result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
		result.AssetInventorySummary = asset.InventorySummaryFromBundle(result.AssetBundle)
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

	_, sdsOptions, mediaErr := buildStandardWorkflowMediaPhase(s).run(ctx, task, result, canonicalProduct, recorder, log)
	if mediaErr != nil {
		return &standardWorkflowState{result: result}, mediaErr
	}

	inventory, recipesByPlatform, generationPlan, persistedGenerationTasks := buildStandardWorkflowAssetPhase(s).run(
		ctx,
		task,
		result,
		canonicalProduct,
		recorder,
		enableAssetGeneration,
	)

	recorder.FinalizeSummary()
	snapshot := buildStandardProductSnapshot(result)
	result.StandardProductSnapshot = snapshot
	return &standardWorkflowState{
		result:                   result,
		snapshot:                 snapshot,
		recipesByPlatform:        recipesByPlatform,
		generationPlan:           generationPlan,
		inventory:                inventory,
		persistedGenerationTasks: persistedGenerationTasks,
		enableAssetGeneration:    enableAssetGeneration,
		sdsOptions:               sdsOptions,
	}, nil
}
