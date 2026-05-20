package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func applySheinSubmissionRemoteSummary(item *TaskListItem, pkg *SheinPackage) {
	if item == nil || pkg == nil || pkg.Submission == nil {
		return
	}
	submission := pkg.Submission
	item.SheinSubmissionRemoteStatus = submission.RemoteStatus
	item.SheinSubmissionRemoteCheckedAt = submission.RemoteCheckedAt
	record := sheinPrimarySubmissionRecord(submission)
	if record == nil {
		return
	}
	item.SheinSubmissionRemoteRecordID = record.RemoteRecordID
	if item.SheinSubmissionRemoteCheckedAt == nil {
		item.SheinSubmissionRemoteCheckedAt = record.RemoteCheckedAt
	}
}

func deriveSheinWorkflowStatus(pkg *SheinPackage) string {
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
	if pkg.Submission != nil {
		if pkg.Submission.Publish != nil && pkg.Submission.Publish.Status == "success" {
			return SheinWorkflowStatusPublished
		}
		if pkg.Submission.SaveDraft != nil && pkg.Submission.SaveDraft.Status == "success" {
			return SheinWorkflowStatusDraftSaved
		}
		if pkg.Submission.LastStatus == "failed" {
			return SheinWorkflowStatusPublishFailed
		}
	}
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness != nil && readiness.Ready {
		return SheinWorkflowStatusReadyToSubmit
	}
	return SheinWorkflowStatusPendingConfirmation
}

func latestSheinSubmissionEvent(pkg *SheinPackage) *sheinpub.SubmissionEvent {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	return &pkg.SubmissionEvents[0]
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
