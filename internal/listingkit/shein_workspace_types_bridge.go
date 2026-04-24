package listingkit

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinworkspace "task-processor/internal/workspace/shein"
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

func buildSheinEditorContext(pkg *sheinpub.Package) *SheinEditorContext {
	return sheinworkspace.BuildEditorContext(pkg)
}

func buildSheinCategoryResolutionPatch(pkg *sheinpub.Package) *SheinCategoryResolutionPatch {
	return sheinworkspace.BuildCategoryResolutionPatch(pkg)
}

func buildSheinAttributeResolutionPatch(pkg *sheinpub.Package) *SheinAttributeResolutionPatch {
	return sheinworkspace.BuildAttributeResolutionPatch(pkg)
}

func buildSheinSaleAttributeResolutionPatch(pkg *sheinpub.Package) *SheinSaleAttributeResolutionPatch {
	return sheinworkspace.BuildSaleAttributeResolutionPatch(pkg)
}

func buildSheinEditorSKCPatches(pkg *sheinpub.Package) []SheinSKCRevisionPatch {
	return sheinworkspace.BuildEditorSKCPatches(pkg)
}
