// Adapter-only bridge. Keep domain rules in internal/marketplace/shein/*.
package listingkit

import (
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

type SheinPackage = sheinpub.Package
type SheinRequestDraft = sheinpub.RequestDraft
type SheinImageDraft = sheinpub.ImageDraft
type LocalizedText = sheinpub.LocalizedText
type SheinSKCRequestDraft = sheinpub.SKCRequestDraft
type SheinSKUDraft = sheinpub.SKUDraft
type SheinSitePrice = sheinpub.SitePrice
type SheinStockInfo = sheinpub.StockInfo
type SheinCategoryResolution = sheinpub.CategoryResolution
type SheinResolvedAttribute = sheinpub.ResolvedAttribute
type SheinAttributeResolution = sheinpub.AttributeResolution
type SheinPendingAttributeCandidate = sheinpub.PendingAttributeCandidate
type SheinResolvedSaleAttribute = sheinpub.ResolvedSaleAttribute
type SheinSaleAttributeResolution = sheinpub.SaleAttributeResolution
type SheinSaleAttributeCandidateInfo = sheinpub.SaleAttributeCandidateInfo
type SheinSubmissionReport = sheinpub.SubmissionReport
type SheinSubmissionRecord = sheinpub.SubmissionRecord
type SheinSubmissionResponse = sheinpub.SubmissionResponse
type SheinInspection = sheinpub.Inspection
type SheinInspectionSection = sheinpub.InspectionSection
type SheinInspectionAction = sheinpub.InspectionAction
type SheinInspectionCategoryPayload = sheinpub.InspectionCategoryPayload
type SheinInspectionAttributePayload = sheinpub.InspectionAttributePayload
type SheinInspectionSaleAttributePayload = sheinpub.InspectionSaleAttributePayload
type SheinInspectionSKCPatchPayload = sheinpub.InspectionSKCPatchPayload
type SheinInspectionSKUPatchPayload = sheinpub.InspectionSKUPatchPayload

type SheinEditorContext = sheinworkspace.EditorContext
type SheinEditorBasicsContext = sheinworkspace.EditorBasicsContext
type SheinEditorCategoryContext = sheinworkspace.EditorCategoryContext
type SheinEditorAttributeContext = sheinworkspace.EditorAttributeContext
type SheinEditorSaleAttributeContext = sheinworkspace.EditorSaleAttributeContext

func buildSheinEditorContext(pkg *SheinPackage) *SheinEditorContext {
	return sheinworkspace.BuildEditorContext(pkg)
}

func buildSheinCategoryResolutionPatch(pkg *SheinPackage) *SheinCategoryResolutionPatch {
	return sheinworkspace.BuildCategoryResolutionPatch(pkg)
}

func buildSheinAttributeResolutionPatch(pkg *SheinPackage) *SheinAttributeResolutionPatch {
	return sheinworkspace.BuildAttributeResolutionPatch(pkg)
}

func buildSheinSaleAttributeResolutionPatch(pkg *SheinPackage) *SheinSaleAttributeResolutionPatch {
	return sheinworkspace.BuildSaleAttributeResolutionPatch(pkg)
}

func buildSheinEditorSKCPatches(pkg *SheinPackage) []SheinSKCRevisionPatch {
	return sheinworkspace.BuildEditorSKCPatches(pkg)
}
