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
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}

	options := task.Request.Options.SDS
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

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
		result.SDSDesignResult = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds design sync failed: "+err.Error())
		finishSDSStageWithError(stage, recorder, "sds_design_sync_failed", "SDS design sync failed", err)
		ensureResultPodExecution(result, task.Request)
		return
	}

	if syncResult != nil && syncResult.DesignSync != nil && syncResult.DesignSync.DesignResult != nil {
		result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignSync.DesignResult)
	} else {
		result.SDSDesignResult = buildSDSSyncSummary(options, nil)
	}
	if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
		result.SDSDesignResult.Status = "failed"
		result.SDSDesignResult.Error = "SDS render returned blank template"
		result.SDSDesignResult.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, task.Request)
}

func (s *service) syncSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil || task == nil || task.Request == nil || !shouldRunRemoteSDSDesignSync(task.Request) {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}
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
	stage := recorder.Start("sds_design_sync", "")
	markChildTask(result, "sds_design_sync", "", string(TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())
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
			result.SDSDesignResult = &SDSSyncSummary{
				VariantID: options.VariantID,
				Status:    "failed",
				Error:     err.Error(),
			}
			markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
			appendWarning(result, "sds template render failed: "+err.Error())
			finishSDSStageWithError(stage, recorder, "sds_template_render_failed", "SDS template render failed", err)
			log.WithError(err).Error("remote SDS design sync failed")
			ensureResultPodExecution(result, task.Request)
			return
		}
		result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignResult)
		if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
			appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
			recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
		}
		if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
			result.SDSDesignResult.Status = "failed"
			result.SDSDesignResult.Error = "SDS render returned blank template"
			result.SDSDesignResult.MockupImageURLs = nil
			appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
			stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
			ensureResultPodExecution(result, task.Request)
			return
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
		stage.Complete()
		ensureResultPodExecution(result, task.Request)
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
		result.SDSDesignResult = &SDSSyncSummary{
			VariantID: options.VariantID,
			Status:    "failed",
			Error:     err.Error(),
		}
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), err.Error())
		appendWarning(result, "sds template render failed: "+err.Error())
		finishSDSStageWithError(stage, recorder, "sds_template_render_failed", "SDS template render failed", err)
		log.WithError(err).Error("remote SDS design sync failed")
		ensureResultPodExecution(result, task.Request)
		return
	}

	result.SDSDesignResult = buildSDSSyncSummary(options, syncResult.DesignResult)
	if needsLocalSDSMockupFallback(result.SDSDesignResult, options) {
		appendWarning(result, "SDS render returned fewer images than expected; local fallback disabled")
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_render_incomplete", "SDS render returned fewer images than expected", "local fallback disabled")
	}
	if sdsRenderedLooksBlank(ctx, result.SDSDesignResult, options) {
		result.SDSDesignResult.Status = "failed"
		result.SDSDesignResult.Error = "SDS render returned blank template"
		result.SDSDesignResult.MockupImageURLs = nil
		appendWarning(result, "SDS render returned blank template; official SDS render needs investigation")
		stage.Degrade("sds_render_blank", "SDS render returned blank template", "official SDS render needs investigation")
		ensureResultPodExecution(result, task.Request)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	stage.Complete()
	ensureResultPodExecution(result, task.Request)
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
	if recorder == nil {
		recorder = newWorkflowRecorder(result)
	}
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
			summaries = append(summaries, SDSSyncSummary{
				VariantID:    variant.VariantID,
				ProductID:    variant.VariantID,
				VariantSKU:   strings.TrimSpace(variant.VariantSKU),
				VariantSize:  strings.TrimSpace(variant.Size),
				VariantColor: strings.TrimSpace(variant.Color),
				Status:       "failed",
				Error:        err.Error(),
			})
			continue
		}
		if syncResult == nil {
			stage.Degrade("sds_variant_render_empty", "SDS variant render returned empty result", "")
			summaries = append(summaries, SDSSyncSummary{
				VariantID:    variant.VariantID,
				ProductID:    variant.VariantID,
				VariantSKU:   strings.TrimSpace(variant.VariantSKU),
				VariantSize:  strings.TrimSpace(variant.Size),
				VariantColor: strings.TrimSpace(variant.Color),
				Status:       "failed",
				Error:        "SDS template render returned empty result",
			})
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
