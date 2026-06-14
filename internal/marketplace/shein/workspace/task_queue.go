package workspace

import "strings"

const (
	WorkflowStatusPublished           = "published"
	WorkflowStatusDraftSaved          = "draft_saved"
	WorkflowStatusPublishFailed       = "publish_failed"
	WorkflowStatusReadyToSubmit       = "ready_to_submit"
	WorkflowStatusPendingConfirmation = "pending_confirmation"
)

const (
	WorkQueueGeneration       = "generation_queue"
	WorkQueueGenerationFailed = "generation_failed_queue"
	WorkQueueRepair           = "repair_queue"
	WorkQueueReview           = "review_queue"
	WorkQueueSubmitReady      = "submit_ready_queue"
	WorkQueueDraft            = "draft_queue"
	WorkQueueSubmitFailed     = "submit_failed_queue"
	WorkQueuePublished        = "published_queue"
)

const (
	ActionQueueStoreAuth      = "store_auth_queue"
	ActionQueueClassification = "classification_queue"
	ActionQueueAttributes     = "attributes_queue"
	ActionQueueVariant        = "variant_queue"
	ActionQueueMedia          = "media_queue"
	ActionQueuePricing        = "pricing_queue"
	ActionQueueFinalReview    = "final_review_queue"
	ActionQueueSourceReview   = "source_review_queue"
	ActionQueuePayloadRebuild = "payload_rebuild_queue"
	ActionQueueManualReview   = "manual_review_queue"
	ActionQueueSubmitReady    = "submit_ready_action_queue"
)

func BuildTaskWorkQueue(taskStatus, workflowStatus string, overview *StatusOverview) string {
	switch strings.TrimSpace(taskStatus) {
	case "pending", "processing":
		return WorkQueueGeneration
	case "failed":
		return WorkQueueGenerationFailed
	}
	switch strings.TrimSpace(workflowStatus) {
	case WorkflowStatusPublished:
		return WorkQueuePublished
	case WorkflowStatusDraftSaved:
		return WorkQueueDraft
	case WorkflowStatusPublishFailed:
		return WorkQueueSubmitFailed
	}
	if overview == nil {
		return ""
	}
	switch overview.Status {
	case "blocked":
		return WorkQueueRepair
	case "ready_with_warnings":
		return WorkQueueReview
	case "ready":
		return WorkQueueSubmitReady
	default:
		return ""
	}
}

func BuildTaskActionQueue(taskStatus, workflowStatus string, overview *StatusOverview, blockingKeys []string, warningKeys []string) string {
	switch strings.TrimSpace(taskStatus) {
	case "pending", "processing", "failed":
		return ""
	}
	switch strings.TrimSpace(workflowStatus) {
	case WorkflowStatusPublished, WorkflowStatusDraftSaved, WorkflowStatusPublishFailed:
		return ""
	}
	for _, key := range blockingKeys {
		if queue := actionQueueForKey(key); queue != "" {
			return queue
		}
	}
	for _, key := range warningKeys {
		if queue := actionQueueForKey(key); queue != "" {
			return queue
		}
	}
	if overview != nil && overview.Status == "ready" {
		return ActionQueueSubmitReady
	}
	return ""
}

func actionQueueForKey(key string) string {
	switch strings.TrimSpace(key) {
	case "shein_cookie_unavailable":
		return ActionQueueStoreAuth
	case "category", "category_review":
		return ActionQueueClassification
	case "attributes", "attribute_review":
		return ActionQueueAttributes
	case "sale_attributes", "variants":
		return ActionQueueVariant
	case "images", "final_images", "variant_image_coverage", "pod_platform":
		return ActionQueueMedia
	case "pricing":
		return ActionQueuePricing
	case "final_review":
		return ActionQueueFinalReview
	case "source_facts":
		return ActionQueueSourceReview
	case "request_draft", "preview_product":
		return ActionQueuePayloadRebuild
	case "manual_notes":
		return ActionQueueManualReview
	default:
		return ""
	}
}
