package listingkit

import (
	"go/ast"
	"testing"
)

func TestResolveAssetGenerationActionTargetImplementationBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_action_target_resolution.go")

	assertSourceContainsAll(t, source, []string{
		"type taskGenerationActionTargetResolutionPhase struct{}",
		"type taskGenerationActionTargetResolutionResult struct",
		"buildTaskGenerationActionTargetResolutionPhase()",
		"buildAssetGenerationOverview(queue)",
		"resolveAssetGenerationActionTarget(overview, req)",
		"func resolveAssetGenerationActionTarget(",
		"func collectAssetGenerationActionTargets(",
		"func requestedAssetGenerationActionKey(",
	})
	assertSourceExcludesAll(t, source, []string{
		"executeLayerTemporalAction(",
		"persistGenerationReviewDecision(",
		"buildActionPlatformRenderPreviews(",
		"buildGenerationReviewWorkflowResult(",
		"applyGenerationReviewWorkflow(",
		"buildGenerationReviewSessionPatch(",
		"applyGenerationConditionalStateToActionResult(",
	})
}

func TestTaskGenerationActionTargetResolutionEntryBoundary(t *testing.T) {
	t.Parallel()

	source := readFunctionSourceMatching(t, "task_generation_action_entry.go", "taskGenerationActionEntryPhase.run", func(decl *ast.FuncDecl) bool {
		if decl.Name == nil || decl.Name.Name != "run" || decl.Recv == nil || len(decl.Recv.List) != 1 {
			return false
		}
		star, ok := decl.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			return false
		}
		ident, ok := star.X.(*ast.Ident)
		return ok && ident.Name == "taskGenerationActionEntryPhase"
	})
	callNames := readNamedFunctionCallNames(t, "task_generation_action_entry.go", "run")

	assertSourceContainsAll(t, source, []string{
		"buildTaskGenerationActionTargetResolutionPhase().run(queue, req)",
		"target := resolution.target",
		"ResolutionSource:",
		"resolution.source",
		"requestedAssetGenerationActionKey(req)",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"buildTaskGenerationActionTargetResolutionPhase",
		"buildAssetGenerationActionImpact",
		"buildGenerationReviewSession",
		"requestedAssetGenerationActionKey",
	})
	assertFunctionCallsAppearInOrder(t, callNames, []string{
		"buildTaskGenerationActionTargetResolutionPhase",
		"buildAssetGenerationActionImpact",
		"buildGenerationReviewSession",
	})
	assertFunctionCallsExcludeAll(t, callNames, []string{
		"buildAssetGenerationOverview",
		"resolveAssetGenerationActionTarget",
		"collectAssetGenerationActionTargets",
		"cloneAssetGenerationActionTarget",
		"actionInteractionMode",
		"isAllowedAssetGenerationActionKey",
		"TrimSpace",
	})
}

func TestTaskGenerationActionTargetResolutionServiceHelperBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "service_generation_actions.go")

	assertSourceContainsAll(t, source, []string{
		"func cloneGenerationQueueQuery(",
		"func cloneRetryGenerationTasksRequest(",
	})
	assertSourceExcludesAll(t, source, []string{
		"func cloneAssetGenerationActionTarget(",
		"func cloneAssetGenerationActionImpact(",
		"type taskGenerationActionTargetResolutionPhase struct{}",
		"type taskGenerationActionTargetResolutionResult struct",
		"buildTaskGenerationActionTargetResolutionPhase()",
		"buildAssetGenerationOverview(queue)",
		"func resolveAssetGenerationActionTarget(",
		"func collectAssetGenerationActionTargets(",
		"func requestedAssetGenerationActionKey(",
	})
}
