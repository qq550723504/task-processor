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
	payload := &SheinPreviewPayload{
		Headline:          firstNonEmpty(pkg.SpuName, pkg.ProductNameEn),
		BrandName:         pkg.BrandName,
		CategoryPath:      append([]string(nil), pkg.CategoryPath...),
		CategoryID:        pkg.CategoryID,
		SourceProduct:     buildSheinSourceProductSummary(canonical),
		NeedsReview:       needsReview,
		Summary:           summary,
		ReviewNotes:       append([]string(nil), pkg.ReviewNotes...),
		Inspection:        pkg.Inspection,
		SubmitReadiness:   readiness,
		SubmitChecklist:   checklist,
		ImageUpload:       buildSheinImageUploadPreflight(pkg),
		ResolutionCache:   buildSheinResolutionCacheSummary(pkg),
		RepairCenter:      repairCenter,
		StatusOverview:    statusOverview,
		WorkspaceOverview: sheinworkspace.BuildWorkspaceOverview(statusOverview, submitState, repairState),
		EditorContext:     buildSheinEditorContext(pkg),
		ImageBundle:       pkg.ImageBundle,
		RenderPreviews:    renderPreviews,
		ScenePresets:      buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		DraftPayload:      pkg.DraftPayload,
		PreviewPayload:    pkg.PreviewPayload,
		SubmissionState:   pkg.SubmissionState,
		Pricing:           pkg.Pricing,
		FinalReview:       buildSheinFinalReviewPayload(pkg, canonical, readiness),
		SubmissionEvents:  append([]sheinpub.SubmissionEvent(nil), pkg.SubmissionEvents...),
		InspectionData:    pkg.Inspection,
	}
	return normalizeSheinPreviewPayloadSemanticFields(payload)
}
