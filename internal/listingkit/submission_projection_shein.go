package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinSubmissionProjection struct {
	StatusFields SheinSubmissionStatusFields
	TaskList     SheinTaskListSubmissionFields
}

func applySheinSubmissionStatusFields(fields *SheinSubmissionStatusFields, pkg *SheinPackage) {
	projection := buildSheinSubmissionProjection(pkg)
	if fields == nil || projection == nil {
		return
	}
	*fields = projection.StatusFields
}

func applySheinSubmissionRemoteSummary(fields *SheinTaskListSubmissionFields, pkg *SheinPackage) {
	projection := buildSheinSubmissionProjection(pkg)
	if fields == nil || projection == nil {
		return
	}
	*fields = projection.TaskList
}

func buildSheinSubmissionProjection(pkg *SheinPackage) *sheinSubmissionProjection {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}

	projection := &sheinSubmissionProjection{}
	projection.StatusFields.SheinWorkflowStatus = deriveSheinWorkflowStatus(pkg)

	if latest := latestSheinSubmissionOutcomeEvent(pkg); latest != nil {
		projection.StatusFields.SheinLatestSubmissionStatus = latest.Status
		projection.StatusFields.SheinLatestSubmissionError = latest.ErrorMessage
	} else if pkg.SubmissionState != nil {
		projection.StatusFields.SheinLatestSubmissionStatus = pkg.SubmissionState.LastStatus
		projection.StatusFields.SheinLatestSubmissionError = pkg.SubmissionState.LastError
	}

	if pkg.SubmissionState == nil {
		return projection
	}

	submission := pkg.SubmissionState
	projection.StatusFields.SheinSubmissionRemoteStatus = submission.RemoteStatus
	projection.TaskList.SheinSubmissionRemoteCheckedAt = submission.RemoteCheckedAt

	record := sheinPrimarySubmissionRecord(submission)
	if record == nil {
		return projection
	}
	projection.TaskList.SheinSubmissionRemoteRecordID = record.RemoteRecordID
	if projection.TaskList.SheinSubmissionRemoteCheckedAt == nil {
		projection.TaskList.SheinSubmissionRemoteCheckedAt = record.RemoteCheckedAt
	}

	return projection
}

func deriveSheinWorkflowStatus(pkg *SheinPackage) string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return ""
	}
	if latest := latestSheinSubmissionOutcomeEvent(pkg); latest != nil {
		if latest.Action == "publish" && latest.Status == "success" {
			return SheinWorkflowStatusPublished
		}
		if latest.Action == "save_draft" && latest.Status == "success" {
			return SheinWorkflowStatusDraftSaved
		}
		if latest.Status == "failed" {
			return SheinWorkflowStatusPublishFailed
		}
	}
	if pkg.SubmissionState != nil {
		if pkg.SubmissionState.Publish != nil && pkg.SubmissionState.Publish.Status == "success" {
			return SheinWorkflowStatusPublished
		}
		if pkg.SubmissionState.SaveDraft != nil && pkg.SubmissionState.SaveDraft.Status == "success" {
			return SheinWorkflowStatusDraftSaved
		}
		if pkg.SubmissionState.LastStatus == "failed" {
			return SheinWorkflowStatusPublishFailed
		}
	}
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness != nil && readiness.Ready {
		return SheinWorkflowStatusReadyToSubmit
	}
	return SheinWorkflowStatusPendingConfirmation
}

func latestSheinSubmissionOutcomeEvent(pkg *SheinPackage) *sheinpub.SubmissionEvent {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	for i := range pkg.SubmissionEvents {
		event := &pkg.SubmissionEvents[i]
		if event.Action != "submit_phase" {
			return event
		}
	}
	return nil
}

func sheinPrimarySubmissionRecord(submission *sheinpub.SubmissionReport) *sheinpub.SubmissionRecord {
	if submission == nil {
		return nil
	}
	record := sheinSubmissionRecordForAction(submission, submission.LastAction)
	if record != nil {
		return record
	}
	if submission.Publish != nil {
		return submission.Publish
	}
	return submission.SaveDraft
}
