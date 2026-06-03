package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultAdaptationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("handoff_phase_routes_result_adaptation_through_local_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff.go", "run")

		assertSourceContainsAll(t, source, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"return adaptation.fromRetryPage(retryPage), nil",
			"return adaptation.fromQueuePage(queuePage), nil",
		})
		assertSourceExcludesAll(t, source, []string{
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"fromQueuePage",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
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
