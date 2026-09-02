package listingkit

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit/core"
	listingworkflow "task-processor/internal/listingkit/workflow"
	productasset "task-processor/internal/product/asset"
)

const sdsDesignSyncTimeout = listingworkflow.SDSDesignSyncTimeout
const sdsDesignSyncExtraPollCap = listingworkflow.SDSDesignSyncExtraPollCap

func sdsDesignSyncTimeoutForVariantCount(targetCount int) time.Duration {
	return listingworkflow.SDSDesignSyncTimeoutForVariantCount(targetCount)
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
	imageURL := approvedMainImageURL(result)
	if imageURL == "" {
		log.Warn("skipping remote SDS design sync because approved main asset is unavailable")
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

func approvedMainImageURL(result *ListingKitResult) string {
	if result == nil {
		return ""
	}
	inventory := result.ApprovedAssetInventory
	if inventory == nil && result.StandardProductSnapshot != nil {
		inventory = result.StandardProductSnapshot.ApprovedAssetInventory
	}
	if inventory == nil {
		return ""
	}
	for _, approved := range inventory.Assets {
		if approved.Role == productasset.RoleMain {
			return strings.TrimSpace(approved.URL)
		}
	}
	return ""
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
	markChildTask(result, "sds_design_sync", "", string(core.TaskStatusProcessing), "")
	ensureResultPodExecution(result, task.Request)
	markPodExecutionStatus(result, podStatusProcessing, time.Now())

	summaries := s.collectSDSVariantRemoteSummaries(ctx, task, imageURL, options, representatives, recorder, syncService)
	finalizeSDSVariantRemoteSummaries(result, task.Request, recorder, options, summaries)
}
