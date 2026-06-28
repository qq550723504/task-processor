package listingkit

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
)

type standardWorkflowMediaPhase struct {
	service *service
}

func buildStandardWorkflowMediaPhase(s *service) *standardWorkflowMediaPhase {
	return &standardWorkflowMediaPhase{service: s}
}

func (p *standardWorkflowMediaPhase) run(
	ctx context.Context,
	task *Task,
	result *ListingKitResult,
	canonicalProduct *canonical.Product,
	recorder *workflowRecorder,
	log *logrus.Entry,
) (*productimage.ImageProcessResult, *SDSSyncOptions) {
	var imageResult *productimage.ImageProcessResult
	imageSvc := resolveWorkflowImageService(p.service)
	if shouldProcessImages(task.Request) && imageSvc != nil {
		stage := recorder.Start("product_image", "")
		imageTask, imageErr := imageSvc.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), toImageProcessRequest(task))
		if imageErr != nil {
			markChildTask(result, "product_image", "", string(TaskStatusFailed), imageErr.Error())
			appendWarning(result, "image processing skipped: "+imageErr.Error())
			stage.Degrade("image_processing_skipped", "Image processing skipped", imageErr.Error())
		} else {
			stage.SetTaskID(imageTask.ID)
			markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusPending), "")
			imageResult, imageErr = imageSvc.ProcessImages(ctx, imageTask)
			if imageErr != nil {
				markChildTask(result, "product_image", imageTask.ID, string(TaskStatusFailed), imageErr.Error())
				appendWarning(result, "image processing failed: "+imageErr.Error())
				stage.Degrade("image_processing_failed", "Image processing failed", imageErr.Error())
			} else {
				markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusCompleted), "")
				stage.Complete()
				result.ImageAssets = imageResult
				result.AssetBundle = asset.BuildBundle(canonicalProduct, imageResult)
				result.AssetInventorySummary = asset.InventorySummaryFromBundle(result.AssetBundle)
				p.service.syncSDSDesign(ctx, task, result, imageResult, recorder)
			}
		}
	}
	if imageResult == nil && shouldRunRemoteSDSDesignSync(task.Request) {
		log.Info("starting remote SDS design sync for listing kit workflow")
		p.service.syncSDSDesignFromRemote(ctx, task, result, recorder)
		log.WithFields(logrus.Fields{
			"sds_status": func() string {
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Status
			}(),
			"sds_error": func() string {
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Error
			}(),
		}).Info("finished remote SDS design sync for listing kit workflow")
	}
	var sdsOptions *SDSSyncOptions
	if task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if applySDSSyncMetadataToCanonical(canonicalProduct, result.SDSDesignResult, sdsOptions) {
		result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
		result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
		result.AssetInventorySummary = asset.InventorySummaryFromBundle(result.AssetBundle)
		log.Info("applied SDS sync metadata to canonical product")
	}
	return imageResult, sdsOptions
}
