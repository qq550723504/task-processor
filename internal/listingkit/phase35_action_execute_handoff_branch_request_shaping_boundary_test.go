package listingkit

import "testing"

func TestTaskGenerationActionExecuteRequestHandoffRequestShapingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_request_shaping_owner_keeps_only_retry_request_clone_handoff", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_retry.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_retry.go", "buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase")
		source := readExactMethodSource(t, "task_generation_action_execute_request_handoff_retry.go", "func (p *taskGenerationActionExecuteRequestHandoffRetryRequestPhase) run(")
		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffRetryRequestPhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &taskGenerationActionExecuteRequestHandoffRetryRequestPhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil {",
			"return cloneRetryGenerationTasksRequest(target.RetryRequest)",
		})
		assertSourceExcludesAll(t, source, []string{
			"RetryTaskGenerationTasks(",
			"GetTaskGenerationQueue(",
			"cloneGenerationQueueQuery(",
			"buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(",
			"fromRetryPage(",
			"fromQueuePage(",
		})
	})

	t.Run("queue_request_shaping_owner_keeps_only_queue_query_clone_handoff", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_execute_request_handoff_queue.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_execute_request_handoff_queue.go", "buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase")
		source := readExactMethodSource(t, "task_generation_action_execute_request_handoff_queue.go", "func (p *taskGenerationActionExecuteRequestHandoffQueueRequestPhase) run(")

		assertSourceContainsAll(t, fileSource, []string{
			"func buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase()",
			"func (p *taskGenerationActionExecuteRequestHandoffQueueRequestPhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &taskGenerationActionExecuteRequestHandoffQueueRequestPhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil {",
			"return cloneGenerationQueueQuery(target.QueueQuery)",
		})
		assertSourceExcludesAll(t, source, []string{
			"GetTaskGenerationQueue(",
			"RetryTaskGenerationTasks(",
			"cloneRetryGenerationTasksRequest(",
			"buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(",
			"fromRetryPage(",
			"fromQueuePage(",
		})
	})
}
