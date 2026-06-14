package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

type Package = sheinpub.Package
type RequestDraft = sheinpub.RequestDraft
type ImageDraft = sheinpub.ImageDraft
type LocalizedText = sheinpub.LocalizedText
type SKCRequestDraft = sheinpub.SKCRequestDraft
type SKUDraft = sheinpub.SKUDraft
type SitePrice = sheinpub.SitePrice
type StockInfo = sheinpub.StockInfo
type CategoryResolution = sheinpub.CategoryResolution
type ResolvedAttribute = sheinpub.ResolvedAttribute
type AttributeResolution = sheinpub.AttributeResolution
type PendingAttributeCandidate = sheinpub.PendingAttributeCandidate
type ResolvedSaleAttribute = sheinpub.ResolvedSaleAttribute
type SaleAttributeResolution = sheinpub.SaleAttributeResolution
type SaleAttributeCandidateInfo = sheinpub.SaleAttributeCandidateInfo
type SubmissionReport = sheinpub.SubmissionReport
type SubmissionRecord = sheinpub.SubmissionRecord
type SubmissionResponse = sheinpub.SubmissionResponse
type Inspection = sheinpub.Inspection
type InspectionSection = sheinpub.InspectionSection
type InspectionAction = sheinpub.InspectionAction
type InspectionCategoryPayload = sheinpub.InspectionCategoryPayload
type InspectionAttributePayload = sheinpub.InspectionAttributePayload
type InspectionSaleAttributePayload = sheinpub.InspectionSaleAttributePayload
type InspectionSKCPatchPayload = sheinpub.InspectionSKCPatchPayload
type InspectionSKUPatchPayload = sheinpub.InspectionSKUPatchPayload

type EditorContext = sheinmarketplace.EditorContext
type EditorBasicsContext = sheinmarketplace.EditorBasicsContext
type EditorCategoryContext = sheinmarketplace.EditorCategoryContext
type EditorAttributeContext = sheinmarketplace.EditorAttributeContext
type EditorSaleAttributeContext = sheinmarketplace.EditorSaleAttributeContext
type RevisionInput = sheinmarketplace.RevisionInput
type CategoryResolutionPatch = sheinmarketplace.CategoryResolutionPatch
type AttributeResolutionPatch = sheinmarketplace.AttributeResolutionPatch
type SaleAttributeResolutionPatch = sheinmarketplace.SaleAttributeResolutionPatch
type SKCRevisionPatch = sheinmarketplace.SKCRevisionPatch
type SKURevisionPatch = sheinmarketplace.SKURevisionPatch
type EditorRevisionSkeleton = sheinmarketplace.EditorRevisionSkeleton
type EditorRecommendationMeta = sheinmarketplace.EditorRecommendationMeta
type EditorAttributeSuggestion = sheinmarketplace.EditorAttributeSuggestion
type EditorSaleCandidateSuggestion = sheinmarketplace.EditorSaleCandidateSuggestion
type EditorEffect = sheinmarketplace.EditorEffect
type EditorProgress = sheinmarketplace.EditorProgress
type EditorProgressSection = sheinmarketplace.EditorProgressSection
type EditorDirtyHints = sheinmarketplace.EditorDirtyHints
type EditorDirtyHintSection = sheinmarketplace.EditorDirtyHintSection
type RepairCenter[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenter[R, P, S, Q, V]
type RepairCenterStats = sheinmarketplace.RepairCenterStats
type RepairCenterSection = sheinmarketplace.RepairCenterSection
type RepairCenterAction[R any, P any, S any, Q any, V any] = sheinmarketplace.RepairCenterAction[R, P, S, Q, V]
type RepairPlan = sheinmarketplace.RepairPlan
type RepairPlanStep = sheinmarketplace.RepairPlanStep
type RepairApplyQueue[Q any, V any] = sheinmarketplace.RepairApplyQueue[Q, V]
type RepairApplyQueueItem[Q any, V any] = sheinmarketplace.RepairApplyQueueItem[Q, V]
type RepairSession = sheinmarketplace.RepairSession
type RepairResumeState = sheinmarketplace.RepairResumeState
type RepairCompletionSnapshot = sheinmarketplace.RepairCompletionSnapshot
type RepairRunbookStep = sheinmarketplace.RepairRunbookStep
type SubmitReadiness[R any, H any] = sheinmarketplace.SubmitReadiness[R, H]
type ReadinessItem[R any, H any] = sheinmarketplace.ReadinessItem[R, H]
type ReadinessCheck[R any, H any] = sheinmarketplace.ReadinessCheck[R, H]
type SubmitChecklist[R any, H any] = sheinmarketplace.SubmitChecklist[R, H]
type ChecklistGroupItem[R any, H any] = sheinmarketplace.ChecklistGroupItem[R, H]

func BuildEditorContext(pkg *Package) *EditorContext {
	return sheinmarketplace.BuildEditorContext(pkg)
}

func BuildCategoryResolutionPatch(pkg *Package) *CategoryResolutionPatch {
	return sheinmarketplace.BuildCategoryResolutionPatch(pkg)
}

func BuildAttributeResolutionPatch(pkg *Package) *AttributeResolutionPatch {
	return sheinmarketplace.BuildAttributeResolutionPatch(pkg)
}

func BuildSaleAttributeResolutionPatch(pkg *Package) *SaleAttributeResolutionPatch {
	return sheinmarketplace.BuildSaleAttributeResolutionPatch(pkg)
}

func BuildEditorSKCPatches(pkg *Package) []SKCRevisionPatch {
	return sheinmarketplace.BuildEditorSKCPatches(pkg)
}
