package listingkit

import "context"

func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {
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
	if effectiveReq.Platform == "shein" {
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
		if effectiveReq.Platform == "shein" {
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
