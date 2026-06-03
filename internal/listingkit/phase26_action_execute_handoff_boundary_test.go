package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultAdaptationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("result_routing_seams_defer_to_phase26_adaptation_home", func(t *testing.T) {
		t.Parallel()

		retryFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_result.go")
		retryBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry_result.go", "run")
		queueFileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue_result.go")
		queueBuildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue_result.go", "buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_queue_result.go", "run")

		assertSourceContainsAll(t, retryFileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase()",
			"adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),",
		})
		assertSourceContainsAll(t, retryBuildSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
		})
		assertSourceContainsAll(t, retrySource, []string{
			"return p.adaptation.fromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{
			"fromRetryPage",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})

		assertSourceContainsAll(t, queueFileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase()",
			"adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),",
		})
		assertSourceContainsAll(t, queueBuildSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
		})
		assertSourceContainsAll(t, queueSource, []string{
			"return p.adaptation.fromQueuePage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{
			"fromQueuePage",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("result_adaptation_phase_owns_page_to_persistence_queue_mapping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_adaptation.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "fromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "fromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "fromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_adaptation.go", "fromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) fromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) fromQueuePage(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"retryPage:        retryPage",
			"persistenceQueue: generationWorkQueueFromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
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
			"queuePage:        queuePage",
			"persistenceQueue: generationWorkQueueFromPage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
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
