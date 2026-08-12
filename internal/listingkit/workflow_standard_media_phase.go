package listingkit

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit/core"
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
		imageRequests, requestErr := toImageProcessRequests(task)
		if requestErr != nil {
			stage := recorder.Start("product_image", "")
			markChildTask(result, "product_image", "", string(core.TaskStatusFailed), requestErr.Error())
			appendWarning(result, "image processing skipped: "+requestErr.Error())
			stage.Degrade("image_processing_skipped", "Image processing skipped", requestErr.Error())
		} else {
			for _, imageRequest := range imageRequests {
				target := imageRequest.TargetPlatform
				kind := fmt.Sprintf("product_image:%s", target)
				stage := recorder.Start(kind, "")
				imageTask, imageErr := imageSvc.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), imageRequest)
				if imageErr != nil {
					markChildTask(result, kind, "", string(core.TaskStatusFailed), imageErr.Error())
					appendWarning(result, "image processing skipped for "+target+": "+imageErr.Error())
					stage.Degrade("image_processing_skipped", "Image processing skipped", imageErr.Error())
					continue
				}
				stage.SetTaskID(imageTask.ID)
				markChildTask(result, kind, imageTask.ID, string(productimage.TaskStatusPending), "")
				targetResult, imageErr := imageSvc.ProcessImages(ctx, imageTask)
				if imageErr != nil {
					markChildTask(result, kind, imageTask.ID, string(core.TaskStatusFailed), imageErr.Error())
					appendWarning(result, "image processing failed for "+target+": "+imageErr.Error())
					stage.Degrade("image_processing_failed", "Image processing failed", imageErr.Error())
					continue
				}
				markChildTask(result, kind, imageTask.ID, string(productimage.TaskStatusCompleted), "")
				stage.Complete()
				bundle := asset.BuildBundle(canonicalProduct, targetResult)
				result.recordTargetImageAssets(target, targetResult, bundle, asset.InventorySummaryFromBundle(bundle), compatibilityTargetPlatform(task.Request))
				if imageResult == nil {
					imageResult = targetResult
				}
				p.service.syncSDSDesign(ctx, task, result, targetResult, recorder)
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
		for target, targetImageResult := range result.ImageAssetsByTarget {
			bundle := asset.BuildBundle(canonicalProduct, targetImageResult)
			result.recordTargetImageAssets(target, targetImageResult, bundle, asset.InventorySummaryFromBundle(bundle), compatibilityTargetPlatform(task.Request))
		}
		log.Info("applied SDS sync metadata to canonical product")
	}
	return imageResult, sdsOptions
}

func compatibilityTargetPlatform(req *GenerateRequest) string {
	if req == nil || req.Options == nil {
		return ""
	}
	return req.Options.CompatibilityTargetPlatform
}
