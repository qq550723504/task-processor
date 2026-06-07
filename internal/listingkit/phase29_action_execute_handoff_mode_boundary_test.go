package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffModeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("top_level_handoff_routes_through_local_mode_routing_seam", func(t *testing.T) {
		t.Parallel()

		source := readExactMethodSource(t, "task_generation_action_execute_request_handoff.go", "func (p *taskGenerationActionExecuteRequestHandoffPhase) run(")

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
	})

	t.Run("mode_routing_seam_routes_interaction_modes_through_local_mode_pairing_seam", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff.go")
		source := readExactMethodSource(t, "task_generation_action_execute_request_handoff.go", "func (p *taskGenerationActionExecuteRequestHandoffModeRoutingPhase) run(")
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
	})
}
