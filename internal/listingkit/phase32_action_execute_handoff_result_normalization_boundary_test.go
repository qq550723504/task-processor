package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffResultNormalizationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("result_normalization_phase_owns_retry_and_queue_mirror_normalization", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_normalization.go")
		retrySource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_normalization.go", "fromRetryPage")
		retryCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_normalization.go", "fromRetryPage")
		queueSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_result_normalization.go", "fromQueuePage")
		queueCalls := readNamedFunctionCallNames(t, "task_generation_action_execute_request_handoff_result_normalization.go", "fromQueuePage")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase) fromRetryPage(",
			"func (p *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase) fromQueuePage(",
		})

		assertSourceContainsAll(t, retrySource, []string{
			"retryPage:        retryPage",
			"persistenceQueue: p.adaptation.persistenceQueueFromRetryPage(retryPage)",
		})
		assertSourceExcludesAll(t, retrySource, []string{
			"taskGenerationActionExecuteRequestHandoff{",
			"queuePage:",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, retryCalls, []string{"persistenceQueueFromRetryPage"})
		assertFunctionCallsExcludeAll(t, retryCalls, []string{
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})

		assertSourceContainsAll(t, queueSource, []string{
			"queuePage:        queuePage",
			"persistenceQueue: p.adaptation.persistenceQueueFromQueuePage(queuePage)",
		})
		assertSourceExcludesAll(t, queueSource, []string{
			"taskGenerationActionExecuteRequestHandoff{",
			"retryPage:",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, queueCalls, []string{"persistenceQueueFromQueuePage"})
		assertFunctionCallsExcludeAll(t, queueCalls, []string{
			"generationWorkQueueFromRetryPage",
			"generationWorkQueueFromPage",
			"RetryTaskGenerationTasks",
			"cloneRetryGenerationTasksRequest",
			"GetTaskGenerationQueue",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("persistence_queue_mapping_stays_outside_normalized_result_shape", func(t *testing.T) {
		t.Parallel()

		adaptationSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_adaptation.go")
		shapeSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_result_shape.go")

		assertSourceExcludesAll(t, adaptationSource, []string{
			"taskGenerationActionExecuteRequestHandoffResultNormalization{",
			"taskGenerationActionExecuteRequestHandoff{",
			"retryPage:",
			"queuePage:",
			"persistenceQueue:",
		})
		assertSourceExcludesAll(t, shapeSource, []string{
			"buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(",
			"persistenceQueueFromRetryPage(",
			"persistenceQueueFromQueuePage(",
			"generationWorkQueueFromRetryPage(",
			"generationWorkQueueFromPage(",
		})
	})
}
