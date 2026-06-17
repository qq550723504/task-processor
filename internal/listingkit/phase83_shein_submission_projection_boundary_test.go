package listingkit

import "testing"

func TestSheinSubmissionProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("status_fields_adapter_delegates_to_shared_submission_projection", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submission_projection_shein.go", "applySheinSubmissionStatusFields")
		callNames := readNamedFunctionCallNames(t, "submission_projection_shein.go", "applySheinSubmissionStatusFields")

		assertSourceContainsAll(t, source, []string{
			"projection := buildSheinSubmissionProjection(pkg)",
			"*fields = projection.StatusFields",
		})
		assertSourceExcludesAll(t, source, []string{
			"fields.SheinWorkflowStatus = deriveSheinWorkflowStatus(pkg)",
			"fields.SheinLatestSubmissionStatus = latest.Status",
			"fields.SheinSubmissionRemoteStatus = pkg.SubmissionState.RemoteStatus",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmissionProjection",
		})
	})

	t.Run("task_list_remote_summary_adapter_delegates_to_shared_submission_projection", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submission_projection_shein.go", "applySheinSubmissionRemoteSummary")
		callNames := readNamedFunctionCallNames(t, "submission_projection_shein.go", "applySheinSubmissionRemoteSummary")

		assertSourceContainsAll(t, source, []string{
			"projection := buildSheinSubmissionProjection(pkg)",
			"*fields = projection.TaskList",
		})
		assertSourceExcludesAll(t, source, []string{
			"record := sheinPrimarySubmissionRecord(submission)",
			"fields.SheinSubmissionRemoteRecordID = record.RemoteRecordID",
			"fields.SheinSubmissionRemoteCheckedAt = submission.RemoteCheckedAt",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmissionProjection",
		})
	})

	t.Run("shared_submission_projection_seam_owns_status_and_remote_summary_contract", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submission_projection_shein.go", "buildSheinSubmissionProjection")
		callNames := readNamedFunctionCallNames(t, "submission_projection_shein.go", "buildSheinSubmissionProjection")

		assertSourceContainsAll(t, source, []string{
			"state := sheinpub.ResolveSubmissionProjection(pkg, readyToSubmit)",
			"projection.StatusFields.SheinWorkflowStatus = state.WorkflowStatus",
			"projection.TaskList.SheinSubmissionRemoteRecordID = state.RemoteRecordID",
		})
		assertSourceExcludesAll(t, source, []string{
			"LatestSubmissionOutcomeEvent(pkg)",
			"PrimarySubmissionRecord(",
			"pkg.SubmissionState.LastStatus",
			"pkg.SubmissionState.RemoteStatus",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"NormalizePackageSemanticFields",
			"buildSheinSubmitReadiness",
			"ResolveSubmissionProjection",
		})
	})

	t.Run("task_list_fields_reuses_one_shared_submission_projection", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_list_item_support.go", "applySheinTaskListFields")
		callNames := readNamedFunctionCallNames(t, "task_list_item_support.go", "applySheinTaskListFields")

		assertSourceContainsAll(t, source, []string{
			"submissionProjection := buildSheinSubmissionProjection(pkg)",
			"item.SheinSubmissionStatusFields = submissionProjection.StatusFields",
			"item.SheinTaskListSubmissionFields = submissionProjection.TaskList",
		})
		assertSourceExcludesAll(t, source, []string{
			"applySheinSubmissionStatusFields(&item.SheinSubmissionStatusFields, pkg)",
			"applySheinSubmissionRemoteSummary(&item.SheinTaskListSubmissionFields, pkg)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmissionProjection",
		})
	})
}
