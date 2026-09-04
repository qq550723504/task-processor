package listingkit

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit/core"
	listingworkflow "task-processor/internal/listingkit/workflow"
)

const sdsDesignSyncTimeout = listingworkflow.SDSDesignSyncTimeout
const sdsDesignSyncExtraPollCap = listingworkflow.SDSDesignSyncExtraPollCap

func sdsDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	return listingworkflow.SDSDesignSyncTimeoutForVariantCount(targetCount)
}

func (s *service) syncSDSDesignFromApprovedAssets(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
	if resolveSDSSyncService(s) == nil || task == nil || task.Request == nil || !shouldRunSDSDesignSync(task.Request) {
		return
	}
	result = normalizeListingKitResultSemanticFields(result)
	defer normalizeListingKitResultSemanticFields(result)
	recorder = normalizeSDSSyncRecorder(result, recorder)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/sds_sync_approved_assets",
		"task_id":   task.ID,
	})

	options := task.Request.Options.SDS
	if len(options.Variants) > 0 {
		log.WithField("variant_count", len(options.Variants)).Info("starting approved-asset SDS variant design sync")
		s.syncSDSDesignVariantsFromApprovedAssets(ctx, task, result, recorder)
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
		}).Info("finished approved-asset SDS variant design sync")
		return
	}
	s.runSingleSDSDesignFromApprovedAssets(ctx, task, result, recorder, log)
}

func (s *service) syncSDSDesignVariantsFromApprovedAssets(ctx context.Context, task *Task, result *ListingKitResult, recorder *workflowRecorder) {
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
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

	summaries := s.collectSDSVariantApprovedAssetSummaries(ctx, task, options, representatives, recorder, syncService)
	finalizeSDSVariantApprovedAssetSummaries(result, task.Request, recorder, options, summaries)
}
