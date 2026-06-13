package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinPreviewPayloadInput struct {
	pkg               *sheinpub.Package
	canonical         *canonical.Product
	visualAssetBundle *asset.Bundle
	renderPreviews    *PlatformAssetRenderPreviews
	needsReview       bool
	summary           []string
	readiness         *SheinSubmitReadiness
	checklist         *SheinSubmitChecklist
	repairCenter      *SheinRepairCenter
	statusOverview    *sheinworkspace.StatusOverview
	workspaceOverview *sheinworkspace.WorkspaceOverview
}

func buildAmazonPreviewPayloadFromInput(input amazonPreviewPayloadInput) *AmazonPreviewPayload {
	return buildAmazonPreviewPayloadBody(input)
}

func buildSheinPreviewPayloadFromInput(input sheinPreviewPayloadInput) *SheinPreviewPayload {
	return normalizeSheinPreviewPayloadSemanticFields(buildSheinPreviewPayloadBody(sheinPreviewPayloadBodyInput{
		pkg:               input.pkg,
		canonical:         input.canonical,
		assetBundle:       input.visualAssetBundle,
		renderPreviews:    input.renderPreviews,
		needsReview:       input.needsReview,
		summary:           input.summary,
		readiness:         input.readiness,
		checklist:         input.checklist,
		repairCenter:      input.repairCenter,
		statusOverview:    input.statusOverview,
		workspaceOverview: input.workspaceOverview,
	}))
}

func buildTemuPreviewPayloadFromInput(input reviewablePlatformPreviewPayloadInput, pkg *TemuPackage) *TemuPreviewPayload {
	return buildTemuPreviewPayloadBody(input, pkg)
}

func buildWalmartPreviewPayloadFromInput(input reviewablePlatformPreviewPayloadInput, pkg *WalmartPackage) *WalmartPreviewPayload {
	return buildWalmartPreviewPayloadBody(input, pkg)
}
