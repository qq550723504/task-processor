package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinPreviewPayload(
	pkg *sheinpub.Package,
	pod *PodExecutionSummary,
	canonical *canonical.Product,
) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	input := buildSheinPreviewPayloadInput(
		pkg,
		pod,
		canonical,
	)
	return buildSheinPreviewPayloadFromInput(input)
}

func buildSheinPreviewPayloadInput(
	pkg *sheinpub.Package,
	pod *PodExecutionSummary,
	canonical *canonical.Product,
) sheinPreviewPayloadInput {
	sheinpub.NormalizePackageSemanticFields(pkg)
	needsReview, summary := sheinworkspace.BuildPreviewReviewSummary(pkg)
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	readiness := projection.Readiness
	checklist := projection.Checklist
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	submitState := projection.SubmitState
	statusOverview := projection.StatusOverview
	repairState := sheinworkspace.BuildRepairStateInput(repairCenter)
	return sheinPreviewPayloadInput{
		pkg:               pkg,
		canonical:         canonical,
		needsReview:       needsReview,
		summary:           summary,
		readiness:         readiness,
		checklist:         checklist,
		repairCenter:      repairCenter,
		statusOverview:    statusOverview,
		workspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState),
	}
}
