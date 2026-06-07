package listingkit

import (
	"context"
	"strings"
)

type taskRevisionServiceConfig struct {
	repo                                    Repository
	resolveManualSheinSaleAttributeValueIDs func(context.Context, *Task, *ApplyRevisionRequest) error
	mutateTaskResult                        func(context.Context, string, TaskResultMutation) (*Task, error)
	refreshSheinDerivedState                func(*Task, *ApplyRevisionRequest)
	buildTaskPreview                        func(context.Context, *Task, string) (*ListingKitPreview, error)
}

type taskRevisionService struct {
	repo                                    Repository
	resolveManualSheinSaleAttributeValueIDs func(context.Context, *Task, *ApplyRevisionRequest) error
	mutateTaskResult                        func(context.Context, string, TaskResultMutation) (*Task, error)
	refreshSheinDerivedState                func(*Task, *ApplyRevisionRequest)
	buildTaskPreview                        func(context.Context, *Task, string) (*ListingKitPreview, error)
}

func newTaskRevisionService(config taskRevisionServiceConfig) *taskRevisionService {
	return &taskRevisionService{
		repo:                                    config.repo,
		resolveManualSheinSaleAttributeValueIDs: config.resolveManualSheinSaleAttributeValueIDs,
		mutateTaskResult:                        config.mutateTaskResult,
		refreshSheinDerivedState:                config.refreshSheinDerivedState,
		buildTaskPreview:                        config.buildTaskPreview,
	}
}

func (s *taskRevisionService) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	effectiveReq, _, err := resolveRevisionValidationRequest(task.Result, req)
	if err != nil {
		return nil, err
	}
	if effectiveReq.Platform == "shein" && s.resolveManualSheinSaleAttributeValueIDs != nil {
		if err := s.resolveManualSheinSaleAttributeValueIDs(ctx, task, effectiveReq); err != nil {
			return nil, err
		}
	}
	var appliedChanges *RevisionDiffPreview
	task, err = s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		if task.Result == nil {
			return ErrTaskResultUnavailable
		}
		before, err := cloneListingKitResult(task.Result)
		if err != nil {
			return err
		}
		if err := applyListingKitRevision(task.Result, effectiveReq); err != nil {
			return err
		}
		if effectiveReq.Platform == "shein" && s.refreshSheinDerivedState != nil {
			s.refreshSheinDerivedState(task, effectiveReq)
		}
		appliedChanges = buildAppliedChangesPreview(effectiveReq.Platform, before, task.Result)
		task.Result.Revision = &ListingKitRevisionSummary{
			UpdatedAt:              task.Result.Revision.UpdatedAt,
			UpdatedBy:              task.Result.Revision.UpdatedBy,
			Reason:                 task.Result.Revision.Reason,
			Platform:               task.Result.Revision.Platform,
			ActionType:             revisionActionType(req),
			RestoredFromRevisionID: revisionRestoreSourceID(req),
			Timeline: buildRevisionTimelineSummary(ListingKitRevisionRecord{
				Platform:               task.Result.Revision.Platform,
				ActionType:             revisionActionType(req),
				RestoredFromRevisionID: revisionRestoreSourceID(req),
				AppliedChanges:         appliedChanges,
			}),
		}
		appendRevisionHistory(task.Result, ListingKitRevisionRecord{
			UpdatedAt:              task.Result.Revision.UpdatedAt,
			UpdatedBy:              task.Result.Revision.UpdatedBy,
			Reason:                 task.Result.Revision.Reason,
			Platform:               task.Result.Revision.Platform,
			ActionType:             revisionActionType(req),
			RestoredFromRevisionID: revisionRestoreSourceID(req),
			AppliedChanges:         appliedChanges,
			EditorContext:          buildRevisionHistorySnapshot(effectiveReq.Platform, task.Result),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	preview, err := s.buildTaskPreview(ctx, task, effectiveReq.Platform)
	if err != nil {
		return nil, err
	}
	preview.ApplyResult = buildRevisionApplyResult(req, task.Result, appliedChanges)
	preview.AppliedChanges = appliedChanges
	preview.RestoreResult = buildRevisionRestoreResult(req, task.Result, appliedChanges)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(task.Result.RevisionHistory)
	return preview, nil
}

func appendRevisionHistory(result *ListingKitResult, record ListingKitRevisionRecord) {
	if result == nil {
		return
	}
	if record.RevisionID == "" {
		record.RevisionID = newRevisionHistoryRecordID()
	}
	result.RevisionHistoryTotal++
	result.RevisionHistory = append(result.RevisionHistory, record)
	result.RevisionHistory = applyRevisionHistoryRetention(result.RevisionHistory)
}

func (s *taskRevisionService) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	page, err := buildRevisionHistoryPage(task.Result, query)
	if err != nil {
		return nil, err
	}
	storeResolution := sheinStoreResolutionSummaryFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
	if page != nil && storeResolution != nil {
		for idx := range page.Items {
			page.Items[idx].StoreResolution = storeResolution
		}
	}
	return page, nil
}

func (s *taskRevisionService) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	detail, err := buildRevisionHistoryDetail(task.Result, revisionID, query)
	if err != nil {
		return nil, err
	}
	if detail != nil && detail.Record != nil {
		detail.Record.StoreResolution = sheinStoreResolutionSummaryFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
	}
	return detail, nil
}

func (s *taskRevisionService) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	platform := ""
	if req != nil {
		platform = strings.ToLower(strings.TrimSpace(req.Platform))
	}
	effectiveReq, restorePreview, err := resolveRevisionValidationRequest(task.Result, req)
	if err != nil {
		return nil, err
	}
	if effectiveReq != nil {
		platform = strings.ToLower(strings.TrimSpace(effectiveReq.Platform))
	}
	validationErr := validateApplyRevisionRequest(effectiveReq)
	return buildRevisionValidationResult(taskID, platform, task.Result, validationErr, restorePreview), nil
}
