package listingkit

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	sdsclient "task-processor/internal/sds/client"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

func (s *service) runSingleSDSDesignFromRemote(ctx context.Context, task *Task, result *ListingKitResult, imageURL string, recorder *workflowRecorder, log *logrus.Entry) {
	options := task.Request.Options.SDS
	recorder, stage := beginSDSSyncStage(result, task.Request, recorder)
	log.WithFields(logrus.Fields{
		"variant_id":         options.VariantID,
		"parent_product_id":  options.ParentProductID,
		"prototype_group_id": options.PrototypeGroupID,
	}).Info("starting remote SDS design sync")

	syncResult, err := s.performSingleSDSRemoteSync(ctx, task, imageURL, options)
	if err != nil {
		if reasonCode, retryable := sdsclient.RetryableUploadFailure(err); retryable {
			if scheduleErr := s.ScheduleSDSChildRetry(ctx, task, reasonCode, err); scheduleErr != nil {
				log.WithError(scheduleErr).Warn("schedule transient SDS upload retry")
			}
		}
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

func (s *service) performSingleSDSRemoteSync(ctx context.Context, task *Task, imageURL string, options *SDSSyncOptions) (*sdsworkflow.SyncResult, error) {
	syncInput := sdsusecase.SyncInput{
		VariantID:        options.VariantID,
		ParentProductID:  options.ParentProductID,
		PrototypeGroupID: options.PrototypeGroupID,
		DesignType:       options.DesignType,
		LayerID:          options.LayerID,
		FitLevel:         options.FitLevel,
		ResizeMode:       options.ResizeMode,
		BlankDesignURL:   options.BlankDesignURL,
	}
	if syncResult, handled, err := s.syncSDSDesignFromUploadedImagePath(ctx, task, imageURL, syncInput); handled {
		return syncResult, err
	}

	syncService := resolveSDSSyncService(s)
	syncCtx, cancel := context.WithTimeout(ctx, sdsDesignSyncTimeout)
	defer cancel()

	return syncService.SyncFromRemoteImage(syncCtx, sdsusecase.RemoteImageInput{
		Sync: syncInput,
		Image: sdsusecase.ImageSource{
			URL:      imageURL,
			FileName: studioSDSMaterialFileName(task),
		},
	})
}

func (s *service) collectSDSVariantRemoteSummaries(
	ctx context.Context,
	task *Task,
	imageURL string,
	options *SDSSyncOptions,
	representatives []SDSSyncVariantOption,
	recorder *workflowRecorder,
	syncService sdsusecase.Service,
) []SDSSyncSummary {
	summaries := make([]SDSSyncSummary, 0, len(representatives))
	for _, variant := range representatives {
		stage := recorder.Start("sds_design_sync", "")
		stage.SetTaskID(strings.TrimSpace(variant.VariantSKU))
		syncResult, err := s.performVariantSDSRemoteSync(ctx, task, imageURL, options, variant, syncService)
		if err != nil {
			if reasonCode, retryable := sdsclient.RetryableUploadFailure(err); retryable {
				if scheduleErr := s.ScheduleSDSChildRetry(ctx, task, reasonCode, err); scheduleErr != nil {
					logrus.WithError(scheduleErr).Warn("schedule transient SDS upload retry")
				}
			}
			finishSDSStageWithError(stage, recorder, "sds_variant_render_failed", "SDS variant render failed", err)
			failedSummary := failedSDSVariantSyncSummary(variant, err.Error())
			if strings.TrimSpace(imageURL) != "" {
				failedSummary.Diagnostics = &SDSSyncDiagnostics{
					MaterialImageURL: strings.TrimSpace(imageURL),
				}
			}
			summaries = append(summaries, failedSummary)
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

func (s *service) performVariantSDSRemoteSync(
	ctx context.Context,
	task *Task,
	imageURL string,
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
}

func finalizeSDSVariantRemoteSummaries(result *ListingKitResult, req *GenerateRequest, recorder *workflowRecorder, options *SDSSyncOptions, summaries []SDSSyncSummary) {
	result.SDSDesignResult = mergeSDSVariantSyncSummaries(options, summaries)
	if result.SDSDesignResult.Status == "failed" {
		appendWarning(result, result.SDSDesignResult.Error)
		markChildTask(result, "sds_design_sync", "", string(TaskStatusFailed), result.SDSDesignResult.Error)
		recorder.AddIssue(WorkflowIssueSeverityWarning, "sds_design_sync", "sds_variant_render_failed", result.SDSDesignResult.Error, "")
		ensureResultPodExecution(result, req)
		return
	}
	markChildTask(result, "sds_design_sync", "", string(TaskStatusCompleted), "")
	ensureResultPodExecution(result, req)
}
