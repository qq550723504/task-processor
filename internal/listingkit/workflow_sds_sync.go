package listingkit

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	listingworkflow "task-processor/internal/listingkit/workflow"
	"task-processor/internal/productimage"
	sdsusecase "task-processor/internal/sds/usecase"
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
	if resolveSDSSyncService(s) == nil || task == nil || task.Request == nil || !shouldRunRemoteSDSDesignSync(task.Request) {
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
	s.runSingleSDSDesignFromRemote(ctx, task, result, imageURL, recorder, log)
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

	summaries := s.collectSDSVariantRemoteSummaries(ctx, task, imageURL, options, representatives, recorder, syncService)
	finalizeSDSVariantRemoteSummaries(result, task.Request, recorder, options, summaries)
}
