package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildSheinPreviewPayload(pkg *sheinpub.Package, pod *PodExecutionSummary, canonical *canonical.Product, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *SheinPreviewPayload {
	if pkg == nil {
		return nil
	}
	sheinpub.NormalizePackageSemanticFields(pkg)
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	readiness := projection.Readiness
	checklist := projection.Checklist
	repairCenter := buildSheinRepairCenter(readiness, checklist)
	submitState := projection.SubmitState
	repairState := sheinworkspace.BuildRepairStateInput(repairCenter)
	statusOverview := projection.StatusOverview
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		summary = uniqueStrings(append(summary, pkg.Inspection.Summary...))
	}
	return normalizeSheinPreviewPayloadSemanticFields(buildSheinPreviewPayloadBody(sheinPreviewPayloadBodyInput{
		pkg:               pkg,
		canonical:         canonical,
		assetBundle:       assetBundle,
		renderPreviews:    renderPreviews,
		needsReview:       needsReview,
		summary:           summary,
		readiness:         readiness,
		checklist:         checklist,
		repairCenter:      repairCenter,
		statusOverview:    statusOverview,
		workspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState),
	}))
}
