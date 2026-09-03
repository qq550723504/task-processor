package listingkit

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit/core"
	productasset "task-processor/internal/product/asset"
	sdsclient "task-processor/internal/sds/client"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

func (s *service) runSingleSDSDesignFromApprovedAssets(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder, log *logrus.Entry) {
	options := task.Request.Options.SDS
	recorder, stage := beginSDSSyncStage(result, task.Request, recorder)
	log.WithFields(logrus.Fields{
		"variant_id":         options.VariantID,
		"parent_product_id":  options.ParentProductID,
		"prototype_group_id": options.PrototypeGroupID,
	}).Info("starting approved-asset SDS design sync")

	syncResult, err := s.performSingleSDSApprovedAssetSync(ctx, task, options)
	if err != nil {
		if reasonCode, retryable := sdsclient.RetryableUploadFailure(err); retryable {
			if scheduleErr := s.ScheduleSDSChildRetry(ctx, task, reasonCode, err); scheduleErr != nil {
				log.WithError(scheduleErr).Warn("schedule transient SDS upload retry")
			}
		}
		failSDSSyncStage(result, task.Request, recorder, stage, options.VariantID, "sds template render failed: ", "sds_template_render_failed", "SDS template render failed", err)
		log.WithError(err).Error("approved-asset SDS design sync failed")
		return
	}

	summary := buildSDSSyncSummary(options, syncResult.DesignResult)
	finalizeSDSSyncSummary(result, task.Request, recorder, stage, summary, options)
	log.WithFields(logrus.Fields{
		"status":        result.SDSDesignResult.Status,
		"mockup_count":  len(result.SDSDesignResult.MockupImageURLs),
		"variant_count": len(result.SDSDesignResult.VariantResults),
	}).Info("approved-asset SDS design sync completed")
}

func (s *service) performSingleSDSApprovedAssetSync(ctx context.Context, task *Task, options *SDSSyncOptions) (*sdsworkflow.SyncResult, error) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil {
		return nil, fmt.Errorf("sds sync service is not configured")
	}
	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	result, err := syncService.SyncFromApprovedAssets(syncCtx, sdsusecase.ApprovedAssetsInput{
		Sync:  singleSDSApprovedAssetSyncInput(options),
		Scope: approvedAssetScopeFromTask(task),
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.DesignSync == nil {
		return nil, fmt.Errorf("sds approved-asset sync returned no design result")
	}
	return result.DesignSync, nil
}

func singleSDSApprovedAssetSyncInput(options *SDSSyncOptions) sdsusecase.SyncInput {
	if options == nil {
		return sdsusecase.SyncInput{}
	}
	return sdsusecase.SyncInput{
		VariantID:        options.VariantID,
		ParentProductID:  options.ParentProductID,
		PrototypeGroupID: options.PrototypeGroupID,
		DesignType:       options.DesignType,
		LayerID:          options.LayerID,
		FitLevel:         options.FitLevel,
		ResizeMode:       options.ResizeMode,
		BlankDesignURL:   options.BlankDesignURL,
	}
}

func approvedAssetScopeFromTask(task *Task) productasset.InventoryScope {
	if task == nil || task.Request == nil {
		return productasset.InventoryScope{}
	}
	return productasset.InventoryScope{TenantID: task.TenantID, ProductKey: task.Request.ProductKey, SourceSnapshotVersion: task.SourceSnapshotVersion}
}

func (s *service) collectSDSVariantApprovedAssetSummaries(
	ctx context.Context,
	task *Task,
	options *SDSSyncOptions,
	representatives []SDSSyncVariantOption,
	recorder *workflowRecorder,
	syncService sdsusecase.Service,
) []SDSSyncSummary {
	summaries := make([]SDSSyncSummary, 0, len(representatives))
	for _, variant := range representatives {
		stage := recorder.Start("sds_design_sync", "")
		stage.SetTaskID(strings.TrimSpace(variant.VariantSKU))
		syncResult, err := s.performVariantSDSApprovedAssetSync(ctx, task, options, variant, syncService)
		if err != nil {
			if reasonCode, retryable := sdsclient.RetryableUploadFailure(err); retryable {
				if scheduleErr := s.ScheduleSDSChildRetry(ctx, task, reasonCode, err); scheduleErr != nil {
					logrus.WithError(scheduleErr).Warn("schedule transient SDS upload retry")
				}
			}
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
	return summaries
}

func (s *service) performVariantSDSApprovedAssetSync(
	ctx context.Context,
	task *Task,
	options *SDSSyncOptions,
	variant SDSSyncVariantOption,
	syncService sdsusecase.Service,
) (*sdsworkflow.SyncResult, error) {
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
	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeoutForVariantCount(1))
	defer cancel()
	result, err := syncService.SyncFromApprovedAssets(syncCtx, sdsusecase.ApprovedAssetsInput{
		Sync:  syncInput,
		Scope: approvedAssetScopeFromTask(task),
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.DesignSync == nil {
		return nil, fmt.Errorf("sds approved-asset sync returned no design result")
	}
	return result.DesignSync, nil
}

func finalizeSDSVariantApprovedAssetSummaries(result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder, options *SDSSyncOptions, summaries []SDSSyncSummary) {
	result.SDSDesignResult = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSDesignResult.Status == "failed" {
		appendWarning(result, result.SDSDesignResult.Error)
		markChildTask(result, "sds_design_sync", "", string(core.TaskStatusFailed), result.SDSDesignResult.Error)
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_variant_render_failed", result.SDSDesignResult.Error, "")
		ensureResultPodExecution(result, req)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusCompleted), "")
	ensureResultPodExecution(result, req)
}
