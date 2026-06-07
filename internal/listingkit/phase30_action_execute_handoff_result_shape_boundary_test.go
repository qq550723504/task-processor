package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultShapeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("unified_handoff_result_shape_stays_in_local_result_shape_home", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_dispatch.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromRetryNormalization")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromRetryNormalization")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromQueueNormalization")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromQueueNormalization")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultShapePhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromRetryNormalization(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromQueueNormalization(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"retryPage:        normalized.retryPage",
			"persistenceQueue: normalized.persistenceQueue",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"queuePage:",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
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

		assertSourceContainsAll(t, queueSource, []string{
			"queuePage:        normalized.queuePage",
			"persistenceQueue: normalized.persistenceQueue",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"retryPage:",
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
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

	t.Run("branch_specific_result_seams_stay_outside_unified_result_shape_owner", func(t *testing.T) {
		t.Parallel()

		shapeSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_dispatch.go")

		assertSourceExcludesAll(t, shapeSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
	})
}
