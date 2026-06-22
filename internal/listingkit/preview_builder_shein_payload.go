package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
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
	visualBase := buildPlatformVisualPresentationBase(pkg.ImageBundle, input.assetBundle, input.renderPreviews)
	return &SheinPreviewPayload{
		Headline:          sheinDisplayTitle(pkg),
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
		EditorContext:     sheinworkspace.BuildEditorContext(pkg),
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

func sheinDisplayTitle(pkg *sheinpub.Package) string {
	if pkg == nil {
		return ""
	}
	title := firstNonEmpty(pkg.ProductNameEn, pkg.ProductNameMulti)
	if strings.TrimSpace(title) != "" {
		return title
	}
	if shouldSuppressSheinTitleFallback(pkg) {
		return ""
	}
	return strings.TrimSpace(pkg.SpuName)
}

func shouldSuppressSheinTitleFallback(pkg *sheinpub.Package) bool {
	if pkg == nil || pkg.TitleDiagnostics == nil {
		return false
	}
	if strings.TrimSpace(firstNonEmpty(pkg.ProductNameEn, pkg.ProductNameMulti)) != "" {
		return false
	}
	switch strings.TrimSpace(pkg.TitleDiagnostics.Source) {
	case "unresolved_prompt_title", "structured_fallback":
		return true
	default:
		return false
	}
}
