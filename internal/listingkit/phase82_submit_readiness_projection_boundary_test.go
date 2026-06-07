package listingkit

import "testing"

func TestSheinSubmitReadinessProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("preview_payload_delegates_readiness_derived_projection_bundle_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein.go", "buildSheinPreviewPayload")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein.go", "buildSheinPreviewPayload")

		assertSourceContainsAll(t, source, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)",
			"readiness := projection.Readiness",
			"checklist := projection.Checklist",
			"submitState := projection.SubmitState",
			"statusOverview := projection.StatusOverview",
		})
		assertSourceExcludesAll(t, source, []string{
			"readiness := buildSheinSubmitReadinessWithPod(pkg, pod)",
			"checklist := buildSheinSubmitChecklist(readiness)",
			"submitState := sheinworkspace.BuildSubmitStateInput(readiness)",
			"statusOverview := sheinworkspace.BuildStatusOverview(pkg.Inspection, submitState)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})
	})

	t.Run("revision_success_helpers_delegate_readiness_projection_bundle_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		statusSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessStatusSummary")
		statusCalls := readNamedFunctionCallNames(t, "revision_success_shared.go", "buildRevisionSuccessStatusSummary")
		checklistSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessFollowUpChecklist")
		checklistCalls := readNamedFunctionCallNames(t, "revision_success_shared.go", "buildRevisionSuccessFollowUpChecklist")

		assertSourceContainsAll(t, statusSource, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)",
			"return sheinworkspace.BuildSuccessStatusSummary(result.Shein, projection.Readiness)",
		})
		assertSourceExcludesAll(t, statusSource, []string{
			"buildSheinSubmitReadinessWithPod(result.Shein, result.PodExecution)",
		})
		assertFunctionCallsContainAll(t, statusCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})

		assertSourceContainsAll(t, checklistSource, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)",
			"return sheinworkspace.BuildSuccessFollowUpChecklist(projection.Checklist)",
		})
		assertSourceExcludesAll(t, checklistSource, []string{
			"buildSheinSubmitChecklist(buildSheinSubmitReadinessWithPod(result.Shein, result.PodExecution))",
		})
		assertFunctionCallsContainAll(t, checklistCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})
	})

	t.Run("task_state_helpers_delegate_readiness_projection_bundle_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		overviewSource := readNamedFunctionSource(t, "task_list_item_support.go", "buildSheinTaskStatusOverviewWithPod")
		overviewCalls := readNamedFunctionCallNames(t, "task_list_item_support.go", "buildSheinTaskStatusOverviewWithPod")
		blockingSource := readNamedFunctionSource(t, "task_list_item_support.go", "sheinBlockingKeysWithPod")
		blockingCalls := readNamedFunctionCallNames(t, "task_list_item_support.go", "sheinBlockingKeysWithPod")
		warningSource := readNamedFunctionSource(t, "task_list_item_support.go", "sheinWarningKeysWithPod")
		warningCalls := readNamedFunctionCallNames(t, "task_list_item_support.go", "sheinWarningKeysWithPod")

		assertSourceContainsAll(t, overviewSource, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)",
			"return projection.StatusOverview",
		})
		assertSourceExcludesAll(t, overviewSource, []string{
			"readiness := buildSheinSubmitReadinessWithPod(pkg, pod)",
			"sheinworkspace.BuildStatusOverview(pkg.Inspection, sheinworkspace.BuildSubmitStateInput(readiness))",
		})
		assertFunctionCallsContainAll(t, overviewCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})

		assertSourceContainsAll(t, blockingSource, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)",
			"readiness := projection.Readiness",
		})
		assertSourceExcludesAll(t, blockingSource, []string{
			"readiness := buildSheinSubmitReadinessWithPod(pkg, pod)",
		})
		assertFunctionCallsContainAll(t, blockingCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})

		assertSourceContainsAll(t, warningSource, []string{
			"projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)",
			"readiness := projection.Readiness",
		})
		assertSourceExcludesAll(t, warningSource, []string{
			"readiness := buildSheinSubmitReadinessWithPod(pkg, pod)",
		})
		assertFunctionCallsContainAll(t, warningCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})
	})

	t.Run("shared_projection_seam_owns_readiness_checklist_and_status_projection_contract", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_readiness_projection_shein.go", "buildSheinSubmitReadinessProjectionWithPod")
		callNames := readNamedFunctionCallNames(t, "submit_readiness_projection_shein.go", "buildSheinSubmitReadinessProjectionWithPod")

		assertSourceContainsAll(t, source, []string{
			"readiness := buildSheinSubmitReadinessWithPod(pkg, pod)",
			"submitState := sheinworkspace.BuildSubmitStateInput(readiness)",
			"Checklist:      buildSheinSubmitChecklist(readiness)",
			"StatusOverview: sheinworkspace.BuildStatusOverview(pkg.Inspection, submitState)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmitReadinessWithPod",
			"buildSheinSubmitChecklist",
			"BuildSubmitStateInput",
			"BuildStatusOverview",
		})
	})
}
