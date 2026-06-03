package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("mode_pairing_seam_routes_branch_results_through_local_result_seams", func(t *testing.T) {
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
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
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
		assertFunctionCallsContainAll(t, retryCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
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
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
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
		assertFunctionCallsContainAll(t, queueCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
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
	})

	t.Run("retry_result_routing_stays_local_and_defers_result_shape", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_result.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"normalization: buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(),",
			"resultShape:   buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.resultShape.fromRetryNormalization(p.normalization.fromRetryPage(retryPage))",
		})
		assertSourceExcludesAll(t, source, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"fromRetryNormalization",
			"fromRetryPage",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("queue_result_routing_stays_local_and_defers_result_shape", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue_result.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue_result.go", "buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"normalization: buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(),",
			"resultShape:   buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.resultShape.fromQueueNormalization(p.normalization.fromQueuePage(queuePage))",
		})
		assertSourceExcludesAll(t, source, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"fromQueueNormalization",
			"fromQueuePage",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})
}
