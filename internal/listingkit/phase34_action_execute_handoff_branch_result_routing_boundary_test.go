package listingkit

import "testing"

func TestTaskGenerationActionExecuteHandoffBranchResultRoutingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("branch_result_dispatch_owner_pairs_unified_normalization_and_shape", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_dispatch.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_dispatch.go", "fromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromQueuePage(",
			"normalization *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase",
			"resultShape   *taskGenerationActionExecuteRequestHandoffResultShapePhase",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"normalization: buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(),",
			"resultShape:   buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"return p.resultShape.fromRetryNormalization(p.normalization.fromRetryPage(retryPage))",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{"fromRetryNormalization", "fromRetryPage"})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"fromQueueNormalization",
			"fromQueuePage",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"return p.resultShape.fromQueueNormalization(p.normalization.fromQueuePage(queuePage))",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{"fromQueueNormalization", "fromQueuePage"})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"fromRetryNormalization",
			"fromRetryPage",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"persistenceQueueFromRetryPage",
			"persistenceQueueFromQueuePage",
		})
	})
}
