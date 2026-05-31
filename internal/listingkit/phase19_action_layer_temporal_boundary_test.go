package listingkit

import (
	"go/ast"
	"testing"
)

func TestTaskGenerationLayerTemporalActionBoundary(t *testing.T) {
	t.Parallel()

	source := readFunctionSourceMatching(t, "task_generation_service.go", "method executeLayerTemporalAction", func(decl *ast.FuncDecl) bool {
		if decl.Name == nil || decl.Name.Name != "executeLayerTemporalAction" || decl.Recv == nil || len(decl.Recv.List) != 1 {
			return false
		}
		star, ok := decl.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			return false
		}
		ident, ok := star.X.(*ast.Ident)
		return ok && ident.Name == "taskGenerationService"
	})
	callNames := readNamedFunctionCallNames(t, "task_generation_service.go", "executeLayerTemporalAction")

	assertSourceContainsAll(t, source, []string{
		"requestedAssetGenerationActionKey(req)",
		"buildTaskGenerationActionTemporalStandardPhase(s).run(ctx, taskID, req)",
		"buildTaskGenerationActionTemporalPlatformPhase(s).run(ctx, taskID, req)",
		"return false, nil, nil",
	})
	assertSourceExcludesAll(t, source, []string{
		"StandardProductWorkflowStartInput{",
		"PlatformAdaptWorkflowStartInput{",
		"client.StartStandardProduct(",
		"client.StartPlatformAdaptation(",
		"buildTaskGenerationActionTemporalResultPhase().run(",
		"resolveLayerTemporalPlatform(req)",
		`fmt.Errorf("standard product temporal workflow is not configured")`,
		`fmt.Errorf("platform adaptation temporal workflow is not configured")`,
		"GenerationActionExecutionResult{",
		"GenerationActionAudit{",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"requestedAssetGenerationActionKey",
		"buildTaskGenerationActionTemporalStandardPhase",
		"buildTaskGenerationActionTemporalPlatformPhase",
	})
	assertFunctionCallsAppearInOrder(t, callNames, []string{
		"requestedAssetGenerationActionKey",
		"buildTaskGenerationActionTemporalStandardPhase",
		"buildTaskGenerationActionTemporalPlatformPhase",
	})
	assertFunctionCallsExcludeAll(t, callNames, []string{
		"StartStandardProduct",
		"StartPlatformAdaptation",
		"resolveLayerTemporalPlatform",
	})
}

func TestTaskGenerationLayerTemporalPhaseOwnershipBoundary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		required  []string
		forbidden []string
	}{
		{
			name: "service_router_only",
			path: "task_generation_service.go",
			required: []string{
				"requestedAssetGenerationActionKey(req)",
				"buildTaskGenerationActionTemporalStandardPhase(s).run(ctx, taskID, req)",
				"buildTaskGenerationActionTemporalPlatformPhase(s).run(ctx, taskID, req)",
				"return false, nil, nil",
			},
			forbidden: []string{
				"StandardProductWorkflowStartInput{",
				"PlatformAdaptWorkflowStartInput{",
				"client.StartStandardProduct(",
				"client.StartPlatformAdaptation(",
				"buildTaskGenerationActionTemporalResultPhase().run(",
				"resolveLayerTemporalPlatform(req)",
				"GenerationActionExecutionResult{",
				"GenerationActionAudit{",
			},
		},
		{
			name: "standard_phase_owns_standard_temporal_start",
			path: "task_generation_action_temporal_standard.go",
			required: []string{
				"standardWorkflow()",
				"client.StartStandardProduct(",
				"StandardProductWorkflowStartInput{",
				"buildTaskGenerationActionTemporalResultPhase().run(",
			},
			forbidden: []string{
				"resolveLayerTemporalPlatform(req)",
				"client.StartPlatformAdaptation(",
				"PlatformAdaptWorkflowStartInput{",
				"GenerationActionExecutionResult{",
				"GenerationActionAudit{",
			},
		},
		{
			name: "platform_phase_owns_platform_temporal_start",
			path: "task_generation_action_temporal_platform.go",
			required: []string{
				"platformAdaptWorkflow()",
				"resolveLayerTemporalPlatform(req)",
				"client.StartPlatformAdaptation(",
				"PlatformAdaptWorkflowStartInput{",
				"buildTaskGenerationActionTemporalResultPhase().run(",
				"&GenerationQueueQuery{Platform: platform}",
			},
			forbidden: []string{
				"client.StartStandardProduct(",
				"StandardProductWorkflowStartInput{",
				"GenerationActionExecutionResult{",
				"GenerationActionAudit{",
			},
		},
		{
			name: "shared_result_phase_owns_temporal_outward_shape",
			path: "task_generation_action_temporal_result.go",
			required: []string{
				"cloneGenerationQueueQuery(queueQuery)",
				"GenerationActionExecutionResult{",
				"GenerationActionAudit{",
				`ResolutionSource:`,
				`"layer_temporal"`,
				`ExecutionPath:`,
				`"queue_only"`,
			},
			forbidden: []string{
				"client.StartStandardProduct(",
				"client.StartPlatformAdaptation(",
				"StandardProductWorkflowStartInput{",
				"PlatformAdaptWorkflowStartInput{",
				"resolveLayerTemporalPlatform(req)",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := readTaskGenerationSourceFile(t, tc.path)
			assertSourceContainsAll(t, source, tc.required)
			assertSourceExcludesAll(t, source, tc.forbidden)
		})
	}
}
