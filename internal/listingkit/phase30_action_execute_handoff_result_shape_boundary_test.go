package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultShapeBoundary(t *testing.T) {
	t.Parallel()

	t.Run("unified_handoff_result_shape_stays_in_local_result_shape_home", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_shape.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_shape.go", "fromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_shape.go", "fromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_shape.go", "fromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_shape.go", "fromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultShapePhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromQueuePage(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"retryPage:        retryPage",
			"persistenceQueue: p.adaptation.persistenceQueueFromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"queuePage:",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{"persistenceQueueFromRetryPage"})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"queuePage:        queuePage",
			"persistenceQueue: p.adaptation.persistenceQueueFromQueuePage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"retryPage:",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{"persistenceQueueFromQueuePage"})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
		})
	})

	t.Run("branch_specific_result_seams_stay_outside_unified_result_shape_owner", func(t *testing.T) {
		t.Parallel()

		shapeSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_shape.go")

		assertSourceExcludesAll(t, shapeSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase(",
			"func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
	})
}
