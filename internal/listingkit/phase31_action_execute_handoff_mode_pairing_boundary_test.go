package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffModePairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("mode_routing_seam_ends_at_local_mode_pairing_owner", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")

		assertSourceContainsAll(t, source, []string{
			"pairing := buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(p.service)",
			"return pairing.runRetryable(ctx, taskID, target)",
			"return pairing.runQueue(ctx, taskID, target)",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage)",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage)",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffModePairingPhase",
			"runRetryable",
			"runQueue",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
		})
	})

	t.Run("mode_pairing_owner_pairs_branch_invocation_and_result_without_absorbing_branch_local_work", func(t *testing.T) {
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
		assertSourceExcludesAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"return buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage), nil",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"fromRetryPage(",
			"fromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"fromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"return buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage), nil",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"fromRetryPage(",
			"fromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"fromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})
}
