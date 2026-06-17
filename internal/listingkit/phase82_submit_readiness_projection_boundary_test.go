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

		projectionSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessReadinessProjection")
		projectionCalls := readNamedFunctionCallNames(t, "revision_success_shared.go", "buildRevisionSuccessReadinessProjection")
		statusSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessStatusSummary")
		statusCalls := readNamedFunctionCallNames(t, "revision_success_shared.go", "buildRevisionSuccessStatusSummary")
		statusFromProjectionSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessStatusSummaryFromProjection")
		checklistSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessFollowUpChecklist")
		checklistCalls := readNamedFunctionCallNames(t, "revision_success_shared.go", "buildRevisionSuccessFollowUpChecklist")
		checklistFromProjectionSource := readNamedFunctionSource(t, "revision_success_shared.go", "buildRevisionSuccessFollowUpChecklistFromProjection")

		assertSourceContainsAll(t, projectionSource, []string{
			"return buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)",
		})
		assertFunctionCallsContainAll(t, projectionCalls, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})
		assertSourceContainsAll(t, statusSource, []string{
			"return buildRevisionSuccessStatusSummaryFromProjection(result, buildRevisionSuccessReadinessProjection(result))",
		})
		assertSourceContainsAll(t, statusFromProjectionSource, []string{
			"return sheinworkspace.BuildSuccessStatusSummary(result.Shein, projection.Readiness)",
		})
		assertSourceExcludesAll(t, statusSource, []string{
			"buildSheinSubmitReadinessWithPod(result.Shein, result.PodExecution)",
			"buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)",
		})
		assertFunctionCallsContainAll(t, statusCalls, []string{
			"buildRevisionSuccessReadinessProjection",
			"buildRevisionSuccessStatusSummaryFromProjection",
		})

		assertSourceContainsAll(t, checklistSource, []string{
			"return buildRevisionSuccessFollowUpChecklistFromProjection(buildRevisionSuccessReadinessProjection(result))",
		})
		assertSourceContainsAll(t, checklistFromProjectionSource, []string{
			"return sheinworkspace.BuildSuccessFollowUpChecklist(projection.Checklist)",
		})
		assertSourceExcludesAll(t, checklistSource, []string{
			"buildSheinSubmitChecklist(buildSheinSubmitReadinessWithPod(result.Shein, result.PodExecution))",
			"buildSheinSubmitReadinessProjectionWithPod(result.Shein, result.PodExecution)",
		})
		assertFunctionCallsContainAll(t, checklistCalls, []string{
			"buildRevisionSuccessReadinessProjection",
			"buildRevisionSuccessFollowUpChecklistFromProjection",
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

	t.Run("task_state_queue_helpers_delegate_workspace_queue_rules", func(t *testing.T) {
		t.Parallel()

		workQueueSource := readNamedFunctionSource(t, "task_list_item_support.go", "deriveSheinWorkQueue")
		workQueueCalls := readNamedFunctionCallNames(t, "task_list_item_support.go", "deriveSheinWorkQueue")
		actionQueueSource := readNamedFunctionSource(t, "task_list_item_support.go", "deriveSheinActionQueue")
		actionQueueCalls := readNamedFunctionCallNames(t, "task_list_item_support.go", "deriveSheinActionQueue")

		assertSourceContainsAll(t, workQueueSource, []string{
			"return sheinworkspace.BuildTaskWorkQueue(string(task.Status), workflowStatus, overview)",
		})
		assertSourceExcludesAll(t, workQueueSource, []string{
			"switch task.Status",
			"switch workflowStatus",
			"case \"blocked\":",
		})
		assertFunctionCallsContainAll(t, workQueueCalls, []string{
			"BuildTaskWorkQueue",
		})

		assertSourceContainsAll(t, actionQueueSource, []string{
			"return sheinworkspace.BuildTaskActionQueue(string(task.Status), workflowStatus, overview, blockingKeys, warningKeys)",
		})
		assertSourceExcludesAll(t, actionQueueSource, []string{
			"sheinActionQueueForKey",
			"switch task.Status",
			"switch workflowStatus",
		})
		assertFunctionCallsContainAll(t, actionQueueCalls, []string{
			"BuildTaskActionQueue",
		})
	})

	t.Run("task_list_fields_reuses_one_readiness_projection_for_state_fields", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_list_item_support.go", "applySheinTaskListFields")
		callNames := readNamedFunctionCallNames(t, "task_list_item_support.go", "applySheinTaskListFields")

		assertSourceContainsAll(t, source, []string{
			"readinessProjection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)",
			"item.SheinStatusOverview = readinessProjection.StatusOverview",
			"item.SheinBlockingKeys = uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.BlockingItems))",
			"item.SheinWarningKeys = uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.WarningItems))",
		})
		assertSourceExcludesAll(t, source, []string{
			"item.SheinBlockingKeys = sheinBlockingKeysWithPod(pkg, pod)",
			"item.SheinWarningKeys = sheinWarningKeysWithPod(pkg, pod)",
			"item.SheinStatusOverview = buildSheinTaskStatusOverviewWithPod(pkg, pod)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmitReadinessProjectionWithPod",
		})
	})

	t.Run("revision_success_results_reuse_one_readiness_projection_for_status_and_followup", func(t *testing.T) {
		t.Parallel()

		applySource := readNamedFunctionSource(t, "revision_apply_result.go", "buildRevisionApplyResult")
		applyCalls := readNamedFunctionCallNames(t, "revision_apply_result.go", "buildRevisionApplyResult")
		restoreSource := readNamedFunctionSource(t, "revision_restore_result.go", "buildRevisionRestoreResult")
		restoreCalls := readNamedFunctionCallNames(t, "revision_restore_result.go", "buildRevisionRestoreResult")

		for _, source := range []string{applySource, restoreSource} {
			assertSourceContainsAll(t, source, []string{
				"readinessProjection := buildRevisionSuccessReadinessProjection(listingResult)",
				"statusSummary := buildRevisionSuccessStatusSummaryFromProjection(listingResult, readinessProjection)",
				"readinessProjection,",
			})
			assertSourceExcludesAll(t, source, []string{
				"statusSummary := buildRevisionSuccessStatusSummary(listingResult)",
			})
		}
		assertFunctionCallsContainAll(t, applyCalls, []string{
			"buildRevisionSuccessReadinessProjection",
			"buildRevisionSuccessStatusSummaryFromProjection",
		})
		assertFunctionCallsContainAll(t, restoreCalls, []string{
			"buildRevisionSuccessReadinessProjection",
			"buildRevisionSuccessStatusSummaryFromProjection",
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
