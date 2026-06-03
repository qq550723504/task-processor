package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffBranchBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_branch_invocation_lives_in_local_retry_seam", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry.go")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffRetryPhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.service.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))",
		})
		assertSourceExcludesAll(t, source, []string{
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("queue_branch_invocation_lives_in_local_queue_seam", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_queue.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffQueuePhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.service.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))",
		})
		assertSourceExcludesAll(t, source, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("shared_clone_helpers_remain_outside_branch_local_owners", func(t *testing.T) {
		t.Parallel()

		retrySource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry.go")
		queueSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		serviceSource := readTaskGenerationSourceFile(t, "service_generation_actions.go")

		assertSourceExcludesAll(t, retrySource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceContainsAll(t, serviceSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
	})
}
