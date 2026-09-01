package listingkit

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
	"task-processor/internal/productimage"
	"task-processor/internal/shared/aiidentity"
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
) (*productimage.ImageProcessResult, *SDSSyncOptions, error) {
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
					if productimage.IsIdentityIntegrityError(imageErr) {
						stage.Fail("image_identity_integrity", "Image processing identity integrity failed", imageErr.Error())
						recorder.FinalizeSummary()
						return nil, nil, imageErr
					}
					appendWarning(result, "image processing skipped for "+target+": "+imageErr.Error())
					stage.Degrade("image_processing_skipped", "Image processing skipped", imageErr.Error())
					continue
				}
				stage.SetTaskID(imageTask.ID)
				markChildTask(result, kind, imageTask.ID, string(productimage.TaskStatusPending), "")
				imageCtx := ctx
				if envelope, envelopeErr := imageTask.ExecutionEnvelope(); envelopeErr != nil {
					markChildTask(result, kind, imageTask.ID, string(core.TaskStatusFailed), envelopeErr.Error())
					stage.Fail("image_identity_integrity", "Image processing identity integrity failed", envelopeErr.Error())
					recorder.FinalizeSummary()
					return nil, nil, envelopeErr
				} else if envelope.Version != 0 {
					imageCtx, envelopeErr = aiidentity.RestoreExecutionEnvelope(ctx, envelope, imageTask.ID)
					if envelopeErr != nil {
						markChildTask(result, kind, imageTask.ID, string(core.TaskStatusFailed), envelopeErr.Error())
						stage.Fail("image_identity_integrity", "Image processing identity integrity failed", envelopeErr.Error())
						recorder.FinalizeSummary()
						return nil, nil, envelopeErr
					}
				}
				targetResult, imageErr := imageSvc.ProcessImages(imageCtx, imageTask)
				if imageErr != nil {
					markChildTask(result, kind, imageTask.ID, string(core.TaskStatusFailed), imageErr.Error())
					if productimage.IsIdentityIntegrityError(imageErr) {
						stage.Fail("image_identity_integrity", "Image processing identity integrity failed", imageErr.Error())
						recorder.FinalizeSummary()
						return nil, nil, imageErr
					}
					appendWarning(result, "image processing failed for "+target+": "+imageErr.Error())
					stage.Degrade("image_processing_failed", "Image processing failed", imageErr.Error())
					continue
				}
				if targetResult != nil && targetResult.Review != nil && targetResult.Review.NeedsReview {
					reasons := normalizeReviewReasons(targetResult.Review.Reasons)
					detail := strings.Join(reasons, "; ")
					if detail == "" {
						detail = "image processing requires manual review"
					}
					message := detail
					if len(reasons) == 0 {
						message = "Image processing requires manual review"
					}
					markChildTask(result, kind, imageTask.ID, string(productimage.TaskStatusNeedsReview), detail)
					stage.Review("image_review_required", message, detail)
				} else {
					markChildTask(result, kind, imageTask.ID, string(productimage.TaskStatusCompleted), "")
					stage.Complete()
				}
				bundle := asset.BuildBundle(canonicalProduct, targetResult)
				result.recordTargetImageAssets(target, targetResult, bundle, asset.InventorySummaryFromBundle(bundle))
			}
			imageResult = deterministicSDSImageResult(result)
			if imageResult != nil {
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
		catalogProduct, err := catalog.Normalize(canonicalProduct)
		if err != nil {
			return imageResult, sdsOptions, err
		}
		result.CatalogProduct = catalogProduct
		for target, targetImageResult := range result.ImageAssetsByTarget {
			bundle := asset.BuildBundle(canonicalProduct, targetImageResult)
			result.recordTargetImageAssets(target, targetImageResult, bundle, asset.InventorySummaryFromBundle(bundle))
		}
		log.Info("applied SDS sync metadata to canonical product")
	}
	result.applyCompatibilityAssetProjectionForRequest(task.Request)
	return imageResult, sdsOptions, nil
}

func deterministicSDSImageResult(result *ListingKitResult) *productimage.ImageProcessResult {
	if result == nil {
		return nil
	}
	for _, target := range sortedImageAssetTargets(result.ImageAssetsByTarget) {
		if imageResult := result.ImageAssetsByTarget[target]; imageResult != nil {
			return imageResult
		}
	}
	return result.ImageAssets
}

func compatibilityTargetPlatform(req *GenerateRequest) string {
	if req == nil || req.Options == nil {
		return ""
	}
	return req.Options.CompatibilityTargetPlatform
}
