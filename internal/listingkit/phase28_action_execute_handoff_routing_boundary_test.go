package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("mode_routing_seam_routes_branch_results_through_local_result_seams", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_mode_routing.go")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffModeRoutingPhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			`case "retryable":`,
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage)",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage)",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase()",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"resultShape.fromRetryPage(",
			"resultShape.fromQueuePage(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultShapePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"fromQueuePage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("retry_result_routing_stays_local_and_defers_result_shape", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_result.go")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.resultShape.fromRetryPage(retryPage)",
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
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.resultShape.fromQueuePage(queuePage)",
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
