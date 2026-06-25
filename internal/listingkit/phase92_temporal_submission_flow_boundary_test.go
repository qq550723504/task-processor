package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinTemporalSubmissionFlowBoundary(t *testing.T) {
	t.Parallel()

	payloadSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "PrepareSheinPublishPayload")
	assertSourceContainsAll(t, payloadSource, []string{
		"return flow.PrepareSheinPublishPayload(ctx, in)",
	})
	assertSourceExcludesAll(t, payloadSource, []string{
		"s.payloadStages.Prepare(",
		"s.normalizeSheinSubmitPackage(",
	})

	submitSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "SubmitSheinPublishRemote")
	assertSourceContainsAll(t, submitSource, []string{
		"return flow.SubmitSheinPublishRemote(ctx, in)",
	})
	assertSourceExcludesAll(t, submitSource, []string{
		"s.remoteSubmitter.Submit(",
		"s.persistSheinSubmitPhase(",
	})

	flowSource := readNamedFunctionSource(t, "task_temporal_submission_flow_service.go", "SubmitSheinPublishRemote")
	assertSourceContainsAll(t, flowSource, []string{
		"s.loadSheinPreparedPayloadState(",
		"buildSheinTemporalRemoteSubmitInput(",
		"s.remoteSubmitter.Submit(",
		"s.persistSheinSubmitPhase(",
	})
	flowServiceSrc, err := os.ReadFile("task_temporal_submission_flow_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_flow_service.go) error = %v", err)
	}
	assertSourceExcludesAll(t, string(flowServiceSrc), []string{
		"func (s *taskTemporalSubmissionFlowService) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {",
	})

	prepareSource := readNamedFunctionSource(t, "task_temporal_submission_flow_service.go", "PrepareSheinPublishPayload")
	assertSourceContainsAll(t, prepareSource, []string{
		"loadSheinTemporalPreparedPublishState(ctx, in, s.loadSheinPublishTask, s.normalizeSheinSubmitPackage)",
		"buildSheinTemporalSubmissionPayloadStageContext(state.execution)",
	})
	assertSourceExcludesAll(t, prepareSource, []string{
		"s.loadSheinPublishTaskState",
	})

	uploadSource := readNamedFunctionSource(t, "task_temporal_submission_flow_service.go", "UploadSheinPublishImages")
	assertSourceContainsAll(t, uploadSource, []string{
		"s.loadSheinPreparedPayloadState(",
		"s.payloadStages.UploadImages(",
	})

	preValidateSource := readNamedFunctionSource(t, "task_temporal_submission_flow_service.go", "PreValidateSheinPublish")
	assertSourceContainsAll(t, preValidateSource, []string{
		"s.loadSheinPreparedPayloadState(",
		"s.payloadStages.PreValidate(",
	})

	stateSupportSrc, err := os.ReadFile("task_temporal_submission_flow_state_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_flow_state_support.go) error = %v", err)
	}
	stateSupportContent := string(stateSupportSrc)
	for _, needle := range []string{
		"func (s *taskTemporalSubmissionFlowService) loadSheinPublishExecutionState(ctx context.Context, taskID, action, requestID string) (*sheinTemporalPublishExecutionState, error) {",
		"func (s *taskTemporalSubmissionLifecycleService) loadSheinPublishExecutionState(ctx context.Context, taskID, action, requestID string) (*sheinTemporalPublishExecutionState, error) {",
		"func (s *taskTemporalSubmissionFlowService) loadSheinPreparedPublishState(ctx context.Context, in SheinPublishAttemptInput) (*sheinTemporalPreparedPublishState, error) {",
		"func (s *taskTemporalSubmissionLifecycleService) loadSheinPreparedPublishState(ctx context.Context, in SheinPublishAttemptInput) (*sheinTemporalPreparedPublishState, error) {",
	} {
		if strings.Contains(stateSupportContent, needle) {
			t.Fatalf("task_temporal_submission_flow_state_support.go should not keep unused temporal state wrapper %q; call shared state loaders directly from entrypoints", needle)
		}
	}
}
