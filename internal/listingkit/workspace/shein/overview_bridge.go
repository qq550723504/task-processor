package shein

import sheinmarketplace "task-processor/internal/marketplace/shein/workspace"

const (
	WorkflowStatusPublished           = sheinmarketplace.WorkflowStatusPublished
	WorkflowStatusDraftSaved          = sheinmarketplace.WorkflowStatusDraftSaved
	WorkflowStatusPublishFailed       = sheinmarketplace.WorkflowStatusPublishFailed
	WorkflowStatusReadyToSubmit       = sheinmarketplace.WorkflowStatusReadyToSubmit
	WorkflowStatusPendingConfirmation = sheinmarketplace.WorkflowStatusPendingConfirmation
)

const (
	WorkQueueGeneration       = sheinmarketplace.WorkQueueGeneration
	WorkQueueGenerationFailed = sheinmarketplace.WorkQueueGenerationFailed
	WorkQueueRepair           = sheinmarketplace.WorkQueueRepair
	WorkQueueReview           = sheinmarketplace.WorkQueueReview
	WorkQueueSubmitReady      = sheinmarketplace.WorkQueueSubmitReady
	WorkQueueDraft            = sheinmarketplace.WorkQueueDraft
	WorkQueueSubmitFailed     = sheinmarketplace.WorkQueueSubmitFailed
	WorkQueuePublished        = sheinmarketplace.WorkQueuePublished
)

const (
	ActionQueueStoreAuth      = sheinmarketplace.ActionQueueStoreAuth
	ActionQueueClassification = sheinmarketplace.ActionQueueClassification
	ActionQueueAttributes     = sheinmarketplace.ActionQueueAttributes
	ActionQueueVariant        = sheinmarketplace.ActionQueueVariant
	ActionQueueMedia          = sheinmarketplace.ActionQueueMedia
	ActionQueuePricing        = sheinmarketplace.ActionQueuePricing
	ActionQueueFinalReview    = sheinmarketplace.ActionQueueFinalReview
	ActionQueueSourceReview   = sheinmarketplace.ActionQueueSourceReview
	ActionQueuePayloadRebuild = sheinmarketplace.ActionQueuePayloadRebuild
	ActionQueueManualReview   = sheinmarketplace.ActionQueueManualReview
	ActionQueueSubmitReady    = sheinmarketplace.ActionQueueSubmitReady
)

type SubmitStateInput = sheinmarketplace.SubmitStateInput
type RepairStateInput = sheinmarketplace.RepairStateInput
type StatusOverview = sheinmarketplace.StatusOverview
type WorkspaceOverview = sheinmarketplace.WorkspaceOverview
type WorkspaceSessionEntry = sheinmarketplace.WorkspaceSessionEntry
type WorkspaceSubmitState = sheinmarketplace.WorkspaceSubmitState
type WorkspaceRepairState = sheinmarketplace.WorkspaceRepairState

func BuildSubmitStateInput[R any, H any](readiness *SubmitReadiness[R, H]) *SubmitStateInput {
	return sheinmarketplace.BuildSubmitStateInput(readiness)
}

func BuildRepairStateInput[R any, P any, S any, Q any, V any](center *RepairCenter[R, P, S, Q, V]) *RepairStateInput {
	return sheinmarketplace.BuildRepairStateInput(center)
}

func BuildStatusOverview(inspection *Inspection, readiness *SubmitStateInput) *StatusOverview {
	return sheinmarketplace.BuildStatusOverview(inspection, readiness)
}

func BuildWorkspaceOverview(status *StatusOverview, readiness *SubmitStateInput, repair *RepairStateInput) *WorkspaceOverview {
	return sheinmarketplace.BuildWorkspaceOverview(status, readiness, repair)
}

func BuildPreviewCardStatus(pkg *Package) string {
	return sheinmarketplace.BuildPreviewCardStatus(pkg)
}

func BuildPreviewCardSummary(pkg *Package) string {
	return sheinmarketplace.BuildPreviewCardSummary(pkg)
}

func PreviewCardNeedsReview(pkg *Package) bool {
	return sheinmarketplace.PreviewCardNeedsReview(pkg)
}

func BuildTaskWorkQueue(taskStatus, workflowStatus string, overview *StatusOverview) string {
	return sheinmarketplace.BuildTaskWorkQueue(taskStatus, workflowStatus, overview)
}

func BuildTaskActionQueue(taskStatus, workflowStatus string, overview *StatusOverview, blockingKeys []string, warningKeys []string) string {
	return sheinmarketplace.BuildTaskActionQueue(taskStatus, workflowStatus, overview, blockingKeys, warningKeys)
}

type FacetDescriptor = sheinmarketplace.FacetDescriptor

func WorkflowStatusDescriptors() []FacetDescriptor {
	return sheinmarketplace.WorkflowStatusDescriptors()
}

func WorkQueueDescriptors() []FacetDescriptor {
	return sheinmarketplace.WorkQueueDescriptors()
}

func ActionQueueDescriptors() []FacetDescriptor {
	return sheinmarketplace.ActionQueueDescriptors()
}
