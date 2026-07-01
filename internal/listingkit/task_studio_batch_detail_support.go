package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"

	"gorm.io/gorm"
)

func resolveStudioBatchDetailWithoutGraph(ctx context.Context, studioSessionRepo studioBatchSeedSessionRepository, batchID string) (*StudioBatchDetail, bool, error) {
	if studioSessionRepo == nil {
		return nil, false, gorm.ErrRecordNotFound
	}
	session, err := studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	if session == nil || !session.SavedAsBatch {
		return nil, false, ErrStudioSessionNotFound
	}
	if shouldSyncStudioBatchGraphOnRead(session) {
		return nil, true, nil
	}
	return buildStudioBatchDraftOnlyDetail(session), false, nil
}

func projectStudioBatchDetail(
	detail *StudioBatchDetailGraph,
	draftUpdatedAt *time.Time,
	createdTasks []SheinStudioCreatedTask,
	rejectedTasks []SheinStudioRejectedTask,
	failedTasks []SheinStudioFailedTask,
) *StudioBatchDetail {
	if detail == nil {
		return &StudioBatchDetail{}
	}

	batch := projectStudioBatchRecord(detail.Batch, detail.Items, draftUpdatedAt)
	items := make([]StudioBatchItemDetail, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, StudioBatchItemDetail{
			Item:     item,
			Attempts: append([]StudioGenerationAttemptRecord(nil), detail.AttemptsByItem[item.ID]...),
			Designs:  append([]StudioMaterializedDesignRecord(nil), detail.DesignsByItem[item.ID]...),
		})
	}

	projected := &StudioBatchDetail{
		Batch:         batch,
		Items:         items,
		CreatedTasks:  append([]SheinStudioCreatedTask(nil), createdTasks...),
		RejectedTasks: append([]SheinStudioRejectedTask(nil), rejectedTasks...),
		FailedTasks:   append([]SheinStudioFailedTask(nil), failedTasks...),
	}
	projected.StatusGroups = BuildStudioBatchStatusGroups(projected)
	return projected
}

func projectStudioBatchRecord(batch *StudioBatchRecord, items []StudioBatchItemRecord, draftUpdatedAt *time.Time) *StudioBatchRecord {
	if batch == nil {
		return nil
	}
	cloned := *batch
	cloned.Status = resolveProjectedStudioBatchStatus(cloned.Status, items)
	cloned.DraftUpdatedAt = draftUpdatedAt
	return &cloned
}

func loadStudioBatchDraftState(
	ctx context.Context,
	studioSessionRepo studioBatchSeedSessionRepository,
	taskLinkRepo StudioBatchTaskLinkRepository,
	getTask func(context.Context, string) (*Task, error),
	batchID string,
) (*time.Time, []SheinStudioCreatedTask, []SheinStudioRejectedTask, []SheinStudioFailedTask, error) {
	linkTasks, err := loadStudioBatchCreatedTasksFromLinks(ctx, taskLinkRepo, getTask, batchID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	linkRejectedTasks, err := loadStudioBatchRejectedTasksFromLinks(ctx, taskLinkRepo, batchID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if studioSessionRepo == nil {
		return nil, linkTasks, linkRejectedTasks, nil, nil
	}
	session, err := studioSessionRepo.GetSession(ctx, batchID)
	switch {
	case err == nil:
		if session == nil || !session.SavedAsBatch {
			return nil, linkTasks, linkRejectedTasks, nil, nil
		}
		updatedAt := session.UpdatedAt.UTC()
		createdTasks := mergeStudioCreatedTasks(linkTasks, session.CreatedTasks)
		return &updatedAt, createdTasks, linkRejectedTasks, append([]SheinStudioFailedTask(nil), session.FailedTasks...), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, linkTasks, linkRejectedTasks, nil, nil
	default:
		return nil, nil, nil, nil, err
	}
}

func loadStudioBatchCreatedTasksFromLinks(
	ctx context.Context,
	taskLinkRepo StudioBatchTaskLinkRepository,
	getTask func(context.Context, string) (*Task, error),
	batchID string,
) ([]SheinStudioCreatedTask, error) {
	if taskLinkRepo == nil {
		return nil, nil
	}
	links, err := taskLinkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	tasks := make([]SheinStudioCreatedTask, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		taskID := strings.TrimSpace(link.ListingKitTaskID)
		if taskID == "" || link.Status != studioBatchTaskLinkStatusCreated {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		var task *Task
		if getTask != nil {
			task, err = getTask(ctx, taskID)
			if err != nil || task == nil || task.Status == TaskStatusFailed {
				continue
			}
		}
		seen[taskID] = struct{}{}
		created := SheinStudioCreatedTask{
			ID:                       taskID,
			Title:                    strings.TrimSpace(link.DesignID),
			DesignID:                 strings.TrimSpace(link.DesignID),
			ItemID:                   strings.TrimSpace(link.ItemID),
			SelectionID:              strings.TrimSpace(link.SelectionID),
			CompatibilityFingerprint: strings.TrimSpace(link.CompatibilityFingerprint),
			Status:                   studioBatchCreatedTaskStatus,
			Source:                   resolveStudioBatchTaskLinkSource(&link, task),
			ReasonCode:               strings.TrimSpace(link.ReasonCode),
			Message:                  strings.TrimSpace(link.Message),
		}
		created = projectStudioBatchCreatedTaskFromListingTask(created, task)
		tasks = append(tasks, created)
	}
	return tasks, nil
}

func loadStudioBatchRejectedTasksFromLinks(
	ctx context.Context,
	taskLinkRepo StudioBatchTaskLinkRepository,
	batchID string,
) ([]SheinStudioRejectedTask, error) {
	if taskLinkRepo == nil {
		return nil, nil
	}
	links, err := taskLinkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	rejected := make([]SheinStudioRejectedTask, 0)
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		if link.Status != studioBatchTaskLinkStatusFailed ||
			strings.TrimSpace(link.ListingKitTaskID) != "" ||
			strings.TrimSpace(link.ReasonCode) == "task_create_failed" {
			continue
		}
		key := strings.Join([]string{
			strings.TrimSpace(link.DesignID),
			strings.TrimSpace(link.ItemID),
			strings.TrimSpace(link.SelectionID),
			strings.TrimSpace(link.ReasonCode),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		rejected = append(rejected, SheinStudioRejectedTask{
			DesignID:    strings.TrimSpace(link.DesignID),
			ItemID:      strings.TrimSpace(link.ItemID),
			SelectionID: strings.TrimSpace(link.SelectionID),
			Source:      firstNonEmptyString(strings.TrimSpace(link.Source), studioBatchTaskLinkSourceRejected),
			ReasonCode:  strings.TrimSpace(link.ReasonCode),
			Message:     strings.TrimSpace(link.Message),
		})
	}
	return rejected, nil
}

func projectStudioBatchCreatedTaskFromListingTask(created SheinStudioCreatedTask, task *Task) SheinStudioCreatedTask {
	if task == nil {
		return created
	}
	if strings.TrimSpace(created.Status) == "" {
		created.Status = studioBatchCreatedTaskStatus
	}
	switch task.Status {
	case TaskStatusNeedsReview:
		created.Status = "needs_review"
	case TaskStatusFailed:
		created.Status = "submit_failed"
		if strings.TrimSpace(created.Message) == "" {
			created.Message = strings.TrimSpace(task.Error)
		}
	}
	if task.Result == nil || task.Result.Shein == nil {
		return created
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		if task.Status == TaskStatusCompleted && sheinSubmitReadinessReady(buildSheinSubmitReadiness(pkg)) {
			created.Status = "ready_to_submit"
		}
		return created
	}
	latestStatus := strings.TrimSpace(pkg.SubmissionState.LastStatus)
	latestAction := strings.TrimSpace(pkg.SubmissionState.LastAction)
	if latestStatus != "" {
		created.SubmissionState = latestStatus
	}
	if latestAction != "" {
		created.LastSubmissionAction = latestAction
	}
	switch {
	case latestStatus == sheinpub.SubmissionStatusFailed:
		created.Status = "submit_failed"
		if strings.TrimSpace(created.Message) == "" {
			created.Message = strings.TrimSpace(pkg.SubmissionState.LastError)
		}
	case latestStatus == sheinpub.SubmissionStatusSuccess && latestAction == "publish":
		created.Status = "published"
	case latestStatus == sheinpub.SubmissionStatusSuccess && latestAction == "save_draft":
		created.Status = "draft_saved"
	case sheinSubmitReadinessReady(buildSheinSubmitReadiness(pkg)):
		created.Status = "ready_to_submit"
	}
	return created
}

func sheinSubmitReadinessReady(readiness *SheinSubmitReadiness) bool {
	return readiness != nil && readiness.Ready
}

func shouldSyncStudioBatchGraphOnRead(session *SheinStudioSession) bool {
	if session == nil {
		return false
	}
	if session.Status == SheinStudioSessionStatusGenerating {
		return true
	}
	if session.GenerationJobID != "" {
		return true
	}
	return len(session.GenerationJobs) > 0
}

func buildStudioBatchDraftOnlyDetail(session *SheinStudioSession) *StudioBatchDetail {
	if session == nil {
		return &StudioBatchDetail{}
	}
	batch := buildStudioBatchRecordFromSessionDraft(session, session.UpdatedAt.UTC())
	batch.Status = StudioBatchStatusDraft
	updatedAt := session.UpdatedAt.UTC()
	batch.DraftUpdatedAt = &updatedAt
	detail := &StudioBatchDetail{
		Batch:        batch,
		Items:        []StudioBatchItemDetail{},
		CreatedTasks: append([]SheinStudioCreatedTask(nil), session.CreatedTasks...),
		FailedTasks:  append([]SheinStudioFailedTask(nil), session.FailedTasks...),
	}
	detail.StatusGroups = BuildStudioBatchStatusGroups(detail)
	return detail
}
