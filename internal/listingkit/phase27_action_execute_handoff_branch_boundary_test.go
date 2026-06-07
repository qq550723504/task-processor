package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffBranchBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_branch_invocation_lives_in_local_retry_seam", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry.go", "buildTaskGenerationActionExecuteRequestHandoffRetryPhase")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_retry.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffRetryPhase) run(",
			"request *taskGenerationActionExecuteRequestHandoffRetryRequestPhase",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"request: buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase(),",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.service.RetryTaskGenerationTasks(ctx, taskID, p.request.run(target))",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneRetryGenerationTasksRequest(target.RetryRequest)",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"run",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneRetryGenerationTasksRequest",
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
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "buildTaskGenerationActionExecuteRequestHandoffQueuePhase")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_queue.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueuePhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffQueuePhase) run(",
			"request *taskGenerationActionExecuteRequestHandoffQueueRequestPhase",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"request: buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase(),",
		})
		assertSourceContainsAll(t, source, []string{
			"return p.service.GetTaskGenerationQueue(ctx, taskID, p.request.run(target))",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationQueueQuery(target.QueueQuery)",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"GetTaskGenerationQueue",
			"run",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
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
		retryRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry_request.go")
		queueRequestSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		sharedCloneSource := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceExcludesAll(t, retrySource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, retryRequestSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, queueRequestSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceContainsAll(t, sharedCloneSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
	})
}
