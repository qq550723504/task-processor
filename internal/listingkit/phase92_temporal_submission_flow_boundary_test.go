package listingkit

import "testing"

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

	prepareSource := readNamedFunctionSource(t, "task_temporal_submission_flow_service.go", "PrepareSheinPublishPayload")
	assertSourceContainsAll(t, prepareSource, []string{
		"s.loadSheinPreparedPublishState(",
		"buildSheinTemporalSubmissionPayloadStageContext(state.execution)",
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
}
