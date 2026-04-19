package generation

import "task-processor/internal/asset"

const (
	ExecutionModePipelineBacked = "pipeline_backed"
	ExecutionModeNativeAlias    = "native_alias"
	ExecutionModeRendererBacked = "renderer_backed"
	ExecutionModeDeferredPlan   = "deferred_generation"
	ExecutionModeDeferredStub   = "deferred_stub"
	ExecutionModeGeneratedAsset = "generated_asset"
)

func PlannedExecutionMode(kind asset.Kind) string {
	switch kind {
	case asset.KindWhiteBgImage, asset.KindSubjectCutout:
		return ExecutionModePipelineBacked
	case asset.KindCleanImage:
		return ExecutionModeNativeAlias
	case asset.KindSceneImage, asset.KindSellingPointImage:
		return ExecutionModeRendererBacked
	default:
		return ExecutionModeDeferredPlan
	}
}
