package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultAdaptationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("result_routing_seams_defer_to_phase30_result_shape_home", func(t *testing.T) {
		t.Parallel()

		retryFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry.go")
		retryBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry.go", "buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase")
		retrySource := readExactMethodSource(t, "task_generation_action_execute_request_handoff_retry.go", "func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(")
		queueFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		queueBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase")
		queueSource := readExactMethodSource(t, "task_generation_action_execute_request_handoff_queue.go", "func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(")

		assertSourceContainsAll(t, retryFileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase()",
			"dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase",
		})
		assertSourceContainsAll(t, retryBuildSource, []string{
			"dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),",
		})
		assertSourceContainsAll(t, retrySource, []string{
			"return p.dispatch.fromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"fromRetryNormalization(",
			"fromRetryPage(retryPage))",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertSourceContainsAll(t, queueFileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase()",
			"dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase",
		})
		assertSourceContainsAll(t, queueBuildSource, []string{
			"dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),",
		})
		assertSourceContainsAll(t, queueSource, []string{
			"return p.dispatch.fromQueuePage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"fromQueueNormalization(",
			"fromQueuePage(queuePage))",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
	})

	t.Run("result_adaptation_phase_owns_page_to_persistence_queue_mapping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_adaptation.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "persistenceQueueFromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "persistenceQueueFromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "persistenceQueueFromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "persistenceQueueFromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) persistenceQueueFromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) persistenceQueueFromQueuePage(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"return generationWorkQueueFromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"retryPage:",
			"queuePage:",
			"persistenceQueue:",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{"generationWorkQueueFromRetryPage"})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"return generationWorkQueueFromPage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"retryPage:",
			"queuePage:",
			"persistenceQueue:",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{"generationWorkQueueFromPage"})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})
	})
}
