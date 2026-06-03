package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffBoundary(t *testing.T) {
	t.Parallel()

	t.Run("execute_phase_routes_request_handoff_through_local_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute.go", "run")

		assertSourceContainsAll(t, source, []string{
			"buildTaskGenerationActionExecuteRequestHandoffPhase(p.service).run(ctx, taskID, target)",
			"buildGenerationReviewSession(baseResult, handoff.persistenceQueue, target.QueueQuery)",
		})
		assertSourceExcludesAll(t, source, []string{
			"RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(",
			"switch target.InteractionMode {",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffPhase",
			"buildGenerationReviewSession",
		})
		assertFunctionCallsAppearInOrder(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffPhase",
			"buildGenerationReviewSession",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("request_handoff_phase_owns_branching_and_shared_clone_handoff", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff.go", "run")

		assertSourceContainsAll(t, source, []string{
			`case "retryable":`,
			"RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))",
			"GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))",
			"generationWorkQueueFromRetryPage(retryPage)",
			"generationWorkQueueFromPage(queuePage)",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildGenerationReviewSession(",
			"buildTaskGenerationActionRefreshPhase(",
			"buildTaskGenerationActionProjectionPhase(",
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"generationWorkQueueFromRetryPage",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromPage",
		})
		assertFunctionCallsAppearInOrder(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"generationWorkQueueFromRetryPage",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildGenerationReviewSession",
			"buildTaskGenerationActionRefreshPhase",
			"buildTaskGenerationActionProjectionPhase",
		})
	})

	t.Run("shared_clone_helpers_stay_outside_execute_local_handoff_home", func(t *testing.T) {
		t.Parallel()

		handoffSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff.go")
		serviceSource := readTaskGenerationSourceFile(t, "service_generation_actions.go")

		assertSourceExcludesAll(t, handoffSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceContainsAll(t, serviceSource, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, serviceSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffPhase) run(",
		})
	})
}
