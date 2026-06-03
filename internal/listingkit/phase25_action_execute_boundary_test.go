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

	t.Run("request_handoff_phase_routes_branch_invocation_through_local_seams", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff.go", "run")

		assertSourceContainsAll(t, source, []string{
			`case "retryable":`,
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase()",
			"return adaptation.fromRetryPage(retryPage), nil",
			"return adaptation.fromQueuePage(queuePage), nil",
		})
		assertSourceExcludesAll(t, source, []string{
			"buildGenerationReviewSession(",
			"buildTaskGenerationActionRefreshPhase(",
			"buildTaskGenerationActionProjectionPhase(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"fromRetryPage",
			"fromQueuePage",
		})
		assertFunctionCallsAppearInOrder(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"fromRetryPage",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"buildGenerationReviewSession",
			"buildTaskGenerationActionRefreshPhase",
			"buildTaskGenerationActionProjectionPhase",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
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
