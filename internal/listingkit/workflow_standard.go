package listingkit

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
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
}

func (s *service) runStandardProductWorkflow(ctx context.Context, task *Task) (*standardWorkflowState, error) {
	result := initResult(task)
	recorder := newWorkflowRecorder(result)
	enableAssetGeneration := shouldGenerateAssets(task.Request)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/workflow_standard",
		"task_id":   task.ID,
	})

	canonicalProduct, err := buildStandardWorkflowCanonicalPhase(s).run(ctx, task, result, recorder, log)
	if err != nil {
		return &standardWorkflowState{result: result}, err
	}

	result.CanonicalProduct = canonicalProduct
	result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
	result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
	result.AssetInventorySummary = asset.InventorySummaryFromBundle(result.AssetBundle)
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

	_, sdsOptions := buildStandardWorkflowMediaPhase(s).run(ctx, task, result, canonicalProduct, recorder, log)

	inventory, recipesByPlatform, generationPlan, persistedGenerationTasks := buildStandardWorkflowAssetPhase(s).run(
		ctx,
		task,
		result,
		canonicalProduct,
		recorder,
		enableAssetGeneration,
	)

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
