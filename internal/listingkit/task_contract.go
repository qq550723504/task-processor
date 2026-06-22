package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

const (
	SheinWorkflowStatusPublished           = sheinworkspace.WorkflowStatusPublished
	SheinWorkflowStatusDraftSaved          = sheinworkspace.WorkflowStatusDraftSaved
	SheinWorkflowStatusPublishFailed       = sheinworkspace.WorkflowStatusPublishFailed
	SheinWorkflowStatusReadyToSubmit       = sheinworkspace.WorkflowStatusReadyToSubmit
	SheinWorkflowStatusPendingConfirmation = sheinworkspace.WorkflowStatusPendingConfirmation
)

const (
	SheinWorkQueueGeneration       = sheinworkspace.WorkQueueGeneration
	SheinWorkQueueGenerationFailed = sheinworkspace.WorkQueueGenerationFailed
	SheinWorkQueueRepair           = sheinworkspace.WorkQueueRepair
	SheinWorkQueueReview           = sheinworkspace.WorkQueueReview
	SheinWorkQueueSubmitReady      = sheinworkspace.WorkQueueSubmitReady
	SheinWorkQueueDraft            = sheinworkspace.WorkQueueDraft
	SheinWorkQueueSubmitFailed     = sheinworkspace.WorkQueueSubmitFailed
	SheinWorkQueuePublished        = sheinworkspace.WorkQueuePublished
)

const (
	SheinActionQueueStoreAuth      = sheinworkspace.ActionQueueStoreAuth
	SheinActionQueueClassification = sheinworkspace.ActionQueueClassification
	SheinActionQueueAttributes     = sheinworkspace.ActionQueueAttributes
	SheinActionQueueVariant        = sheinworkspace.ActionQueueVariant
	SheinActionQueueMedia          = sheinworkspace.ActionQueueMedia
	SheinActionQueuePricing        = sheinworkspace.ActionQueuePricing
	SheinActionQueueFinalReview    = sheinworkspace.ActionQueueFinalReview
	SheinActionQueueSourceReview   = sheinworkspace.ActionQueueSourceReview
	SheinActionQueuePayloadRebuild = sheinworkspace.ActionQueuePayloadRebuild
	SheinActionQueueManualReview   = sheinworkspace.ActionQueueManualReview
	SheinActionQueueSubmitReady    = sheinworkspace.ActionQueueSubmitReady
)

var sheinBlockerDescriptors = []TaskFacetDescriptor{
	{Key: sheinCookieUnavailableIssueCode, Label: "店铺登录", Description: "店铺登录或 cookie 不可用。", Severity: "negative"},
	{Key: "category", Label: "类目骨架", Description: "类目主骨架尚未确认。", Severity: "negative"},
	{Key: "category_review", Label: "类目复核", Description: "类目命中后仍需人工复核。", Severity: "negative"},
	{Key: "attributes", Label: "普通属性", Description: "普通属性映射不完整。", Severity: "negative"},
	{Key: "attribute_review", Label: "属性复核", Description: "普通属性仍有必填或重要项未确认。", Severity: "negative"},
	{Key: "sale_attributes", Label: "销售属性", Description: "销售属性或规格结构未完成。", Severity: "negative"},
	{Key: "request_draft", Label: "请求草稿", Description: "提交请求草稿尚未生成。", Severity: "negative"},
	{Key: "preview_product", Label: "预览载荷", Description: "提交前预览载荷尚未生成。", Severity: "negative"},
	{Key: "images", Label: "主图资产", Description: "缺少基础可提交图片。", Severity: "negative"},
	{Key: "final_images", Label: "最终图片", Description: "最终提交图片仍不完整。", Severity: "negative"},
	{Key: "variant_image_coverage", Label: "变体图片覆盖", Description: "变体图片覆盖不完整。", Severity: "negative"},
	{Key: "variants", Label: "规格结构", Description: "SKC/SKU 结构不完整。", Severity: "negative"},
	{Key: "pricing", Label: "价格确认", Description: "价格尚未生成或确认。", Severity: "negative"},
	{Key: "final_review", Label: "最终确认", Description: "正式提交前必须完成最终确认。", Severity: "warning"},
	{Key: "source_facts", Label: "来源事实", Description: "存在缺少来源依据的字段。", Severity: "negative"},
}

var sheinWarningDescriptors = []TaskFacetDescriptor{
	{Key: "manual_notes", Label: "人工备注", Description: "存在建议人工复核的备注项。", Severity: "warning"},
}

func BuildTaskListTaxonomy() *TaskListTaxonomy {
	return &TaskListTaxonomy{
		SheinWorkflowStatuses: cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkflowStatusDescriptors()),
		SheinWorkQueues:       cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.WorkQueueDescriptors()),
		SheinActionQueues:     cloneTaskFacetDescriptorsFromWorkspace(sheinworkspace.ActionQueueDescriptors()),
		SheinBlockers:         cloneTaskFacetDescriptors(sheinBlockerDescriptors),
		SheinWarnings:         cloneTaskFacetDescriptors(sheinWarningDescriptors),
	}
}

func cloneTaskFacetDescriptors(items []TaskFacetDescriptor) []TaskFacetDescriptor {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]TaskFacetDescriptor, len(items))
	copy(cloned, items)
	return cloned
}

func cloneTaskFacetDescriptorsFromWorkspace(items []sheinworkspace.FacetDescriptor) []TaskFacetDescriptor {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]TaskFacetDescriptor, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, TaskFacetDescriptor{
			Key:         item.Key,
			Label:       item.Label,
			Description: item.Description,
			Severity:    item.Severity,
		})
	}
	return cloned
}
