package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinPreviewPayload(
	pkg *sheinpub.Package,
	pod *PodExecutionSummary,
	canonical *canonical.Product,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	input := buildSheinPreviewPayloadInput(
		pkg,
		pod,
		canonical,
		assetBundle,
		renderPreviews,
	)
	return buildSheinPreviewPayloadFromInput(input)
}

func buildSheinPreviewPayloadInput(
	pkg *sheinpub.Package,
	pod *PodExecutionSummary,
	canonical *canonical.Product,
	assetBundle *asset.Bundle,
	renderPreviews *PlatformAssetRenderPreviews,
) sheinPreviewPayloadInput {
	sheinpub.NormalizePackageSemanticFields(pkg)
	needsReview, summary := sheinworkspace.BuildPreviewReviewSummary(pkg)
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	readiness := projection.Readiness
	checklist := projection.Checklist
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	submitState := projection.SubmitState
	statusOverview := projection.StatusOverview
	return sheinPreviewPayloadInput{
		pkg:               pkg,
		canonical:         canonical,
		visualAssetBundle: assetBundle,
		renderPreviews:    renderPreviews,
		needsReview:       needsReview,
		summary:           summary,
		readiness:         readiness,
		checklist:         checklist,
		repairCenter:      repairCenter,
		statusOverview:    statusOverview,
		workspaceOverview: buildSheinPreviewWorkspaceOverview(statusOverview, submitState, repairCenter),
	}
}
