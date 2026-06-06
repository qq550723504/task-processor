package shein

import (
	sheinpub "task-processor/internal/publishing/shein"
	sheinworkspace "task-processor/internal/workspace/shein"
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

type EditorContext = sheinworkspace.EditorContext
type EditorBasicsContext = sheinworkspace.EditorBasicsContext
type EditorCategoryContext = sheinworkspace.EditorCategoryContext
type EditorAttributeContext = sheinworkspace.EditorAttributeContext
type EditorSaleAttributeContext = sheinworkspace.EditorSaleAttributeContext
type RevisionInput = sheinworkspace.RevisionInput
type CategoryResolutionPatch = sheinworkspace.CategoryResolutionPatch
type AttributeResolutionPatch = sheinworkspace.AttributeResolutionPatch
type SaleAttributeResolutionPatch = sheinworkspace.SaleAttributeResolutionPatch
type SKCRevisionPatch = sheinworkspace.SKCRevisionPatch
type SKURevisionPatch = sheinworkspace.SKURevisionPatch
type EditorRevisionSkeleton = sheinworkspace.EditorRevisionSkeleton
type EditorRecommendationMeta = sheinworkspace.EditorRecommendationMeta
type EditorAttributeSuggestion = sheinworkspace.EditorAttributeSuggestion
type EditorSaleCandidateSuggestion = sheinworkspace.EditorSaleCandidateSuggestion
type EditorEffect = sheinworkspace.EditorEffect
type EditorProgress = sheinworkspace.EditorProgress
type EditorProgressSection = sheinworkspace.EditorProgressSection
type EditorDirtyHints = sheinworkspace.EditorDirtyHints
type EditorDirtyHintSection = sheinworkspace.EditorDirtyHintSection
type RepairCenter[R any, P any, S any, Q any, V any] = sheinworkspace.RepairCenter[R, P, S, Q, V]
type RepairCenterStats = sheinworkspace.RepairCenterStats
type RepairCenterSection = sheinworkspace.RepairCenterSection
type RepairCenterAction[R any, P any, S any, Q any, V any] = sheinworkspace.RepairCenterAction[R, P, S, Q, V]
type RepairPlan = sheinworkspace.RepairPlan
type RepairPlanStep = sheinworkspace.RepairPlanStep
type RepairApplyQueue[Q any, V any] = sheinworkspace.RepairApplyQueue[Q, V]
type RepairApplyQueueItem[Q any, V any] = sheinworkspace.RepairApplyQueueItem[Q, V]
type RepairSession = sheinworkspace.RepairSession
type RepairResumeState = sheinworkspace.RepairResumeState
type RepairCompletionSnapshot = sheinworkspace.RepairCompletionSnapshot
type RepairRunbookStep = sheinworkspace.RepairRunbookStep
type SubmitReadiness[R any, H any] = sheinworkspace.SubmitReadiness[R, H]
type ReadinessItem[R any, H any] = sheinworkspace.ReadinessItem[R, H]
type ReadinessCheck[R any, H any] = sheinworkspace.ReadinessCheck[R, H]
type SubmitChecklist[R any, H any] = sheinworkspace.SubmitChecklist[R, H]
type ChecklistGroupItem[R any, H any] = sheinworkspace.ChecklistGroupItem[R, H]

func BuildEditorContext(pkg *Package) *EditorContext {
	return sheinworkspace.BuildEditorContext(pkg)
}

func BuildCategoryResolutionPatch(pkg *Package) *CategoryResolutionPatch {
	return sheinworkspace.BuildCategoryResolutionPatch(pkg)
}

func BuildAttributeResolutionPatch(pkg *Package) *AttributeResolutionPatch {
	return sheinworkspace.BuildAttributeResolutionPatch(pkg)
}

func BuildSaleAttributeResolutionPatch(pkg *Package) *SaleAttributeResolutionPatch {
	return sheinworkspace.BuildSaleAttributeResolutionPatch(pkg)
}

func BuildEditorSKCPatches(pkg *Package) []SKCRevisionPatch {
	return sheinworkspace.BuildEditorSKCPatches(pkg)
}
