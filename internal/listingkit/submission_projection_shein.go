package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinSubmissionProjection struct {
	StatusFields SheinSubmissionStatusFields
	TaskList     SheinTaskListSubmissionFields
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
