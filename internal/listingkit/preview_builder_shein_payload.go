package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinPreviewPayloadBodyInput struct {
	pkg               *sheinpub.Package
	canonical         *canonical.Product
	assetBundle       *asset.Bundle
	renderPreviews    *PlatformAssetRenderPreviews
	needsReview       bool
	summary           []string
	readiness         *SheinSubmitReadiness
	checklist         *SheinSubmitChecklist
	repairCenter      *SheinRepairCenter
	statusOverview    *sheinworkspace.StatusOverview
	workspaceOverview *sheinworkspace.WorkspaceOverview
}

func buildSheinPreviewPayloadBody(input sheinPreviewPayloadBodyInput) *SheinPreviewPayload {
	pkg := input.pkg
	if pkg == nil {
		return nil
	}
	visualBase := buildPlatformVisualPreviewPayloadBase(
		buildPlatformVisualPreviewBase(pkg.ImageBundle, input.assetBundle, input.renderPreviews),
	)
	return &SheinPreviewPayload{
		Headline:          firstNonEmpty(pkg.SpuName, pkg.ProductNameEn),
		BrandName:         pkg.BrandName,
		CategoryPath:      append([]string(nil), pkg.CategoryPath...),
		CategoryID:        pkg.CategoryID,
		SourceProduct:     buildSheinSourceProductSummary(input.canonical),
		NeedsReview:       input.needsReview,
		Summary:           input.summary,
		ReviewNotes:       append([]string(nil), pkg.ReviewNotes...),
		Inspection:        pkg.Inspection,
		SubmitReadiness:   input.readiness,
		SubmitChecklist:   input.checklist,
		ImageUpload:       buildSheinImageUploadPreflight(pkg),
		ResolutionCache:   buildSheinResolutionCacheSummary(pkg),
		RepairCenter:      input.repairCenter,
		StatusOverview:    input.statusOverview,
		WorkspaceOverview: input.workspaceOverview,
		EditorContext:     buildSheinEditorContext(pkg),
		ImageBundle:       visualBase.imageBundle,
		RenderPreviews:    visualBase.renderPreviews,
		ScenePresets:      visualBase.scenePresets,
		DraftPayload:      pkg.DraftPayload,
		PreviewPayload:    pkg.PreviewPayload,
		SubmissionState:   pkg.SubmissionState,
		Pricing:           pkg.Pricing,
		FinalReview:       buildSheinFinalReviewPayload(pkg, input.canonical, input.readiness),
		SubmissionEvents:  append([]sheinpub.SubmissionEvent(nil), pkg.SubmissionEvents...),
		InspectionData:    pkg.Inspection,
	}
}
