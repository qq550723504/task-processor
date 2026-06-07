package listingkit

import "testing"

func TestTaskGenerationActionExecuteHandoffModePairingNormalizationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("mode_pairing_owner_routes_branch_results_through_local_normalization_owner", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_mode_pairing.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runRetryable")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runRetryable")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runQueue")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runQueue")

		assertSourceContainsAll(t, fileSource, []string{
			"normalization *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase",
			"normalization: buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase(),",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"return p.normalization.fromRetryPage(retryPage), nil",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"fromRetryPage",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"return p.normalization.fromQueuePage(queuePage), nil",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"fromQueuePage",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("mode_pairing_normalization_owner_defers_branch_result_work_to_branch_local_result_seams", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_mode_pairing.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "fromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "fromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "fromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "fromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase) fromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffModePairingNormalizationPhase) fromQueuePage(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"retryResult: buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(),",
			"queueResult: buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(),",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"return p.retryResult.run(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{
			"run",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"return p.queueResult.run(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{
			"run",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase",
		})
	})
}
