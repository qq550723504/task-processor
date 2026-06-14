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
	readyToSubmit := false
	if readiness := buildSheinSubmitReadiness(pkg); readiness != nil {
		readyToSubmit = readiness.Ready
	}
	state := sheinpub.ResolveSubmissionProjection(pkg, readyToSubmit)
	projection.StatusFields.SheinWorkflowStatus = state.WorkflowStatus
	projection.StatusFields.SheinLatestSubmissionStatus = state.LatestStatus
	projection.StatusFields.SheinLatestSubmissionError = state.LatestError
	projection.StatusFields.SheinSubmissionRemoteStatus = state.RemoteStatus
	projection.TaskList.SheinSubmissionRemoteCheckedAt = state.RemoteCheckedAt
	projection.TaskList.SheinSubmissionRemoteRecordID = state.RemoteRecordID

	return projection
}
