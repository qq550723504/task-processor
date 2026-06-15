package listingkit

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	listingworkflow "task-processor/internal/listingkit/workflow"
	"task-processor/internal/productimage"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

const sdsDesignSyncTimeout = listingworkflow.SDSDesignSyncTimeout
const sdsDesignSyncExtraPollCap = listingworkflow.SDSDesignSyncExtraPollCap

func sdsDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	return listingworkflow.SDSDesignSyncTimeoutForVariantCount(targetCount)
}

func (s *service) syncSDSDesign(ctx context.Context, task *Task, result *ListingKitResult, imageResult *productimage.ImageProcessResult, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil || !shouldSyncSDS(task.Request) || imageResult == nil {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	options := task.Request.Options.SDS
	recorder, stage := beginSDSSyncStage(result, task.Request, recorder)

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := syncService.SyncFromImageResult(syncCtx, sdsusecase.ImageResultInput{
		Sync: sdsusecase.SyncInput{
			VariantID:        options.VariantID,
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: options.PrototypeGroupID,
			DesignType:       options.DesignType,
			LayerID:          options.LayerID,
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   options.BlankDesignURL,
		},
		ImageResult: imageResult,
	})
	if err != nil {
		failSDSSyncStage(result, task.Request, recorder, stage, options.VariantID, "sds design sync failed: ", "sds_design_sync_failed", "SDS design sync failed", err)
		return
	}

	var summary *SDSSyncSummary
	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		summary = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		summary = buildSDSSyncSummary(options, nil)
	}
	if !finalizeSDSSyncSummary(ctx, result, task.Request, recorder, stage, summary, options) {
		return
	}
}

func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil || task == nil || task.Request == nil || !shouldRunRemoteSDSDesignSync(task.Request) {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	recorder = normalizeSDSSyncRecorder(result, recorder)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/sds_sync_remote",
		"task_id":   task.ID,
	})

	options := task.Request.Options.SDS
	imageURL := strings.TrimSpace(task.Request.ImageURLs[0])
	if imageURL == "" {
		log.Warn("skipping remote SDS design sync because source image URL is empty")
		return
	}
	if len(options.Variants) > 0 {
		log.WithField("variant_count", len(options.Variants)).Info("starting remote SDS variant design sync")
		s.syncSDSDesignVariantsFromRemote(ctx, task, result, imageURL, recorder)
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
		}).Info("finished remote SDS variant design sync")
		return
	}
	recorder, stage := beginSDSSyncStage(result, task.Request, recorder)
	log.WithFields(logrus.Fields{
		"variant_id":         options.VariantID,
		"parent_product_id":  options.ParentProductID,
		"prototype_group_id": options.PrototypeGroupID,
	}).Info("starting remote SDS design sync")

	if syncResult, handled, err := s.syncSDSDesignFromUploadedImagePath(ctx, task, imageURL, sdsusecase.SyncInput{
		VariantID:        options.VariantID,
		ParentProductID:  options.ParentProductID,
		PrototypeGroupID: options.PrototypeGroupID,
		DesignType:       options.DesignType,
		LayerID:          options.LayerID,
		FitLevel:         options.FitLevel,
		ResizeMode:       options.ResizeMode,
		BlankDesignURL:   options.BlankDesignURL,
	}); handled {
		if err != nil {
			failSDSSyncStage(result, task.Request, recorder, stage, options.VariantID, "sds template render failed: ", "sds_template_render_failed", "SDS template render failed", err)
			log.WithError(err).Error("remote SDS design sync failed")
			return
		}
		summary := buildSDSSyncSummary(options, syncResult.DesignResult)
		if !finalizeSDSSyncSummary(ctx, result, task.Request, recorder, stage, summary, options) {
			return
		}
		log.WithFields(logrus.Fields{
			"status":        result.SDSDesignResult.Status,
			"mockup_count":  len(result.SDSDesignResult.MockupImageURLs),
			"variant_count": len(result.SDSDesignResult.VariantResults),
		}).Info("remote SDS design sync completed")
		return
	}

	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	syncResult, err := syncService.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
		Sync: sdsusecase.SyncInput{
			VariantID:        options.VariantID,
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: options.PrototypeGroupID,
			DesignType:       options.DesignType,
			LayerID:          options.LayerID,
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   options.BlankDesignURL,
		},
		Image: sdsusecase.ImageSource{
			URL:      imageURL,
			FileName: studioSDSMaterialFileName(task),
		},
	})
	if err != nil {
		failSDSSyncStage(result, task.Request, recorder, stage, options.VariantID, "sds template render failed: ", "sds_template_render_failed", "SDS template render failed", err)
		log.WithError(err).Error("remote SDS design sync failed")
		return
	}

	summary := buildSDSSyncSummary(options, syncResult.DesignResult)
	if !finalizeSDSSyncSummary(ctx, result, task.Request, recorder, stage, summary, options) {
		return
	}
	log.WithFields(logrus.Fields{
		"status":        result.SDSDesignResult.Status,
		"mockup_count":  len(result.SDSDesignResult.MockupImageURLs),
		"variant_count": len(result.SDSDesignResult.VariantResults),
	}).Info("remote SDS design sync completed")
}

func (s *service) syncSDSDesignVariantsFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil {
		return
	}
	options := task.Request.Options.SDS
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	recorder = normalizeSDSSyncRecorder(result, recorder)
	representatives := representativeSDSVariantsByColor(options.Variants)
	if len(representatives) == 0 {
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

	summaries := make([]SDSSyncSummary, 0, len(representatives))
	for _, variant := range representatives {
		stage := recorder.Start("sds_design_sync", "")
		stage.SetTaskID(strings.TrimSpace(variant.VariantSKU))
		syncInput := sdsusecase.SyncInput{
			VariantID:        firstNonZeroInt64(variant.VariantID, options.VariantID),
			ParentProductID:  options.ParentProductID,
			PrototypeGroupID: firstNonZeroInt64(variant.PrototypeGroupID, options.PrototypeGroupID),
			DesignType:       options.DesignType,
			LayerID:          firstNonEmptyString(variant.LayerID, options.LayerID),
			FitLevel:         options.FitLevel,
			ResizeMode:       options.ResizeMode,
			BlankDesignURL:   firstNonEmptyString(variant.BlankDesignURL, options.BlankDesignURL),
		}
		syncResult, err := func() (*sdsworkflow.SyncResult, error) {
			if key, ok := uploadedListingKitImageKeyFromURL(imageURL); ok {
				result, handled, err := s.syncSDSDesignFromUploadedImageKey(ctx, task, key, syncInput, sdsDesignSyncTimeoutForVariantCount(1))
				if handled {
					return result, err
				}
			}
			syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeoutForVariantCount(1))
			defer cancel()
			return syncService.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
				Sync: syncInput,
				Image: sdsusecase.ImageSource{
					URL:      imageURL,
					FileName: studioSDSMaterialFileName(task),
				},
			})
		}()
		if err != nil {
			finishSDSStageWithError(stage, recorder, "sds_variant_render_failed", "SDS variant render failed", err)
			summaries = append(summaries, failedSDSVariantSyncSummary(variant, err.Error()))
			continue
		}
		if syncResult == nil {
			stage.Degrade("sds_variant_render_empty", "SDS variant render returned empty result", "")
			summaries = append(summaries, emptySDSVariantSyncSummary(variant))
			continue
		}
		summaries = append(summaries, buildSDSVariantSyncSummaries(options, []SDSSyncVariantOption{variant}, syncResult.DesignResult)...)
		stage.Complete()
	}

	result.SDSDesignResult = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSDesignResult.Status == "failed" {
		appendWarning(result, result.SDSDesignResult.Error)
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), result.SDSDesignResult.Error)
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_variant_render_failed", result.SDSDesignResult.Error, "")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	ensureResultPodExecution(result, task.Request)
}
