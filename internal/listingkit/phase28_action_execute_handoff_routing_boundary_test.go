package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("mode_pairing_seam_routes_branch_results_through_local_mode_pairing_normalization", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_mode_pairing.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runRetryable")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runRetryable")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runQueue")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_pairing.go", "runQueue")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runRetryable(",
			"func (p *taskGenerationActionExecuteRequestHandoffModePairingPhase) runQueue(",
		})
		assertSourceContainsAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"return p.normalization.fromRetryPage(retryPage), nil",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase()",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"resultShape.fromRetryPage(",
			"resultShape.fromQueuePage(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
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
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromQueuePage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
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
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase()",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"resultShape.fromRetryPage(",
			"resultShape.fromQueuePage(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
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
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("retry_and_queue_result_owners_defer_to_local_result_dispatch_home", func(t *testing.T) {
		t.Parallel()

		retryFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_result.go")
		retryBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		queueFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		queueBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase")
		queueSource := readExactMethodSource(t, "task_generation_action_execute_request_handoff_queue.go", "func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(")

		assertSourceContainsAll(t, retryFileSource, []string{
			"dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase",
		})
		assertSourceContainsAll(t, retryBuildSource, []string{
			"dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),",
		})
		assertSourceContainsAll(t, retrySource, []string{
			"return p.dispatch.fromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"fromRetryNormalization(",
			"fromQueueNormalization(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{"fromRetryPage"})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"fromQueuePage",
			"fromRetryNormalization",
			"fromQueueNormalization",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
		})

		assertSourceContainsAll(t, queueFileSource, []string{
			"dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase",
		})
		assertSourceContainsAll(t, queueBuildSource, []string{
			"dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),",
		})
		assertSourceContainsAll(t, queueSource, []string{
			"return p.dispatch.fromQueuePage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"fromRetryNormalization(",
			"fromQueueNormalization(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
		})
	})
}
