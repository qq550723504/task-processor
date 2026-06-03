package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffModeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("top_level_handoff_routes_through_local_mode_routing_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff.go", "run")

		assertSourceContainsAll(t, source, []string{
			"return buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(p.service).run(ctx, taskID, target)",
		})
		assertSourceExcludesAll(t, source, []string{
			`case "retryable":`,
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase().run(retryPage)",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase(p.service).run(ctx, taskID, target)",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase().run(queuePage)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildTaskGenerationActionExecuteRequestHandoffRetryPhase",
			"buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueuePhase",
			"buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase",
		})
	})

	t.Run("mode_routing_seam_routes_interaction_modes_through_local_mode_pairing_seam", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_mode_routing.go")
		source := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_mode_routing.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffModeRoutingPhase(",
			"func (p *taskGenerationActionExecuteRequestHandoffModeRoutingPhase) run(",
		})
		assertSourceContainsAll(t, source, []string{
			"pairing := buildTaskGenerationActionExecuteRequestHandoffModePairingPhase(p.service)",
			`case "retryable":`,
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
			"resultShape.fromRetryPage(",
			"resultShape.fromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"buildGenerationReviewSession(",
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
			"fromRetryPage",
			"fromQueuePage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"buildGenerationReviewSession",
		})
	})
}
