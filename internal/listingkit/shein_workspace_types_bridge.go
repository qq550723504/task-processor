// Adapter-only bridge. Keep domain rules in internal/workspace/shein or internal/publishing/shein.
package listingkit

import listingworkspace "task-processor/internal/listingkit/workspace/shein"

type SheinPackage = listingworkspace.Package
type SheinRequestDraft = listingworkspace.RequestDraft
type SheinImageDraft = listingworkspace.ImageDraft
type LocalizedText = listingworkspace.LocalizedText
type SheinSKCRequestDraft = listingworkspace.SKCRequestDraft
type SheinSKUDraft = listingworkspace.SKUDraft
type SheinSitePrice = listingworkspace.SitePrice
type SheinStockInfo = listingworkspace.StockInfo
type SheinCategoryResolution = listingworkspace.CategoryResolution
type SheinResolvedAttribute = listingworkspace.ResolvedAttribute
type SheinAttributeResolution = listingworkspace.AttributeResolution
type SheinPendingAttributeCandidate = listingworkspace.PendingAttributeCandidate
type SheinResolvedSaleAttribute = listingworkspace.ResolvedSaleAttribute
type SheinSaleAttributeResolution = listingworkspace.SaleAttributeResolution
type SheinSaleAttributeCandidateInfo = listingworkspace.SaleAttributeCandidateInfo
type SheinSubmissionReport = listingworkspace.SubmissionReport
type SheinSubmissionRecord = listingworkspace.SubmissionRecord
type SheinSubmissionResponse = listingworkspace.SubmissionResponse
type SheinInspection = listingworkspace.Inspection
type SheinInspectionSection = listingworkspace.InspectionSection
type SheinInspectionAction = listingworkspace.InspectionAction
type SheinInspectionCategoryPayload = listingworkspace.InspectionCategoryPayload
type SheinInspectionAttributePayload = listingworkspace.InspectionAttributePayload
type SheinInspectionSaleAttributePayload = listingworkspace.InspectionSaleAttributePayload
type SheinInspectionSKCPatchPayload = listingworkspace.InspectionSKCPatchPayload
type SheinInspectionSKUPatchPayload = listingworkspace.InspectionSKUPatchPayload

type SheinEditorContext = listingworkspace.EditorContext
type SheinEditorBasicsContext = listingworkspace.EditorBasicsContext
type SheinEditorCategoryContext = listingworkspace.EditorCategoryContext
type SheinEditorAttributeContext = listingworkspace.EditorAttributeContext
type SheinEditorSaleAttributeContext = listingworkspace.EditorSaleAttributeContext

func buildSheinEditorContext(pkg *SheinPackage) *SheinEditorContext {
	return listingworkspace.BuildEditorContext(pkg)
}

func buildSheinCategoryResolutionPatch(pkg *SheinPackage) *SheinCategoryResolutionPatch {
	return listingworkspace.BuildCategoryResolutionPatch(pkg)
}

func buildSheinAttributeResolutionPatch(pkg *SheinPackage) *SheinAttributeResolutionPatch {
	return listingworkspace.BuildAttributeResolutionPatch(pkg)
}

func buildSheinSaleAttributeResolutionPatch(pkg *SheinPackage) *SheinSaleAttributeResolutionPatch {
	return listingworkspace.BuildSaleAttributeResolutionPatch(pkg)
}

func buildSheinEditorSKCPatches(pkg *SheinPackage) []SheinSKCRevisionPatch {
	return listingworkspace.BuildEditorSKCPatches(pkg)
}
