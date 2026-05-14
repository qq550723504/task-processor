package listingkit

const (
	SheinWorkflowStatusPublished           = "published"
	SheinWorkflowStatusDraftSaved          = "draft_saved"
	SheinWorkflowStatusPublishFailed       = "publish_failed"
	SheinWorkflowStatusReadyToSubmit       = "ready_to_submit"
	SheinWorkflowStatusPendingConfirmation = "pending_confirmation"
)

const (
	SheinWorkQueueGeneration       = "generation_queue"
	SheinWorkQueueGenerationFailed = "generation_failed_queue"
	SheinWorkQueueRepair           = "repair_queue"
	SheinWorkQueueReview           = "review_queue"
	SheinWorkQueueSubmitReady      = "submit_ready_queue"
	SheinWorkQueueDraft            = "draft_queue"
	SheinWorkQueueSubmitFailed     = "submit_failed_queue"
	SheinWorkQueuePublished        = "published_queue"
)

const (
	SheinActionQueueStoreAuth      = "store_auth_queue"
	SheinActionQueueClassification = "classification_queue"
	SheinActionQueueAttributes     = "attributes_queue"
	SheinActionQueueVariant        = "variant_queue"
	SheinActionQueueMedia          = "media_queue"
	SheinActionQueuePricing        = "pricing_queue"
	SheinActionQueueFinalReview    = "final_review_queue"
	SheinActionQueueSourceReview   = "source_review_queue"
	SheinActionQueuePayloadRebuild = "payload_rebuild_queue"
	SheinActionQueueManualReview   = "manual_review_queue"
	SheinActionQueueSubmitReady    = "submit_ready_action_queue"
)

var sheinWorkflowStatusDescriptors = []TaskFacetDescriptor{
	{Key: SheinWorkflowStatusPendingConfirmation, Label: "待确认", Description: "资料包已生成，但还未满足正式提交或草稿提交后的终态。", Severity: "neutral"},
	{Key: SheinWorkflowStatusReadyToSubmit, Label: "可提交", Description: "资料包已具备提交前关键骨架，可进入正式发布操作。", Severity: "positive"},
	{Key: SheinWorkflowStatusDraftSaved, Label: "已存草稿", Description: "资料包已经保存为 SHEIN 草稿，可继续人工补充或复核。", Severity: "neutral"},
	{Key: SheinWorkflowStatusPublished, Label: "已发布", Description: "资料包已经完成 SHEIN 发布。", Severity: "positive"},
	{Key: SheinWorkflowStatusPublishFailed, Label: "提交失败", Description: "最近一次保存草稿或正式发布失败，需要人工排查。", Severity: "negative"},
}

var sheinWorkQueueDescriptors = []TaskFacetDescriptor{
	{Key: SheinWorkQueueGeneration, Label: "生成队列", Description: "任务仍在生成或等待生成。", Severity: "neutral"},
	{Key: SheinWorkQueueGenerationFailed, Label: "生成失败队列", Description: "生成流程失败，需要回看上游数据或任务执行。", Severity: "negative"},
	{Key: SheinWorkQueueRepair, Label: "修复队列", Description: "存在阻断项，暂时不能进入提交态。", Severity: "negative"},
	{Key: SheinWorkQueueReview, Label: "复核队列", Description: "可提交但仍有 warning，建议人工复核。", Severity: "warning"},
	{Key: SheinWorkQueueSubmitReady, Label: "待提交队列", Description: "资料包已准备好，可直接进入提交。", Severity: "positive"},
	{Key: SheinWorkQueueDraft, Label: "草稿队列", Description: "已经保存草稿，等待后续处理。", Severity: "neutral"},
	{Key: SheinWorkQueueSubmitFailed, Label: "提交失败队列", Description: "远端提交流程失败，需要人工重试或修复。", Severity: "negative"},
	{Key: SheinWorkQueuePublished, Label: "已发布队列", Description: "已经完成发布。", Severity: "positive"},
}

var sheinActionQueueDescriptors = []TaskFacetDescriptor{
	{Key: SheinActionQueueStoreAuth, Label: "店铺授权处理", Description: "SHEIN 店铺登录或 cookie 异常，需先恢复店铺授权。", Severity: "negative"},
	{Key: SheinActionQueueClassification, Label: "类目处理", Description: "优先处理类目骨架或类目复核问题。", Severity: "negative"},
	{Key: SheinActionQueueAttributes, Label: "属性处理", Description: "优先处理普通属性映射与属性补齐。", Severity: "negative"},
	{Key: SheinActionQueueVariant, Label: "规格处理", Description: "优先处理销售属性、SKC/SKU 结构和变体问题。", Severity: "negative"},
	{Key: SheinActionQueueMedia, Label: "图片处理", Description: "优先处理主图、最终图片和图片覆盖问题。", Severity: "negative"},
	{Key: SheinActionQueuePricing, Label: "价格处理", Description: "优先处理价格生成与价格确认。", Severity: "negative"},
	{Key: SheinActionQueueFinalReview, Label: "最终确认", Description: "进入正式提交前的最终人工核对。", Severity: "warning"},
	{Key: SheinActionQueueSourceReview, Label: "来源复核", Description: "优先复核缺少来源依据的字段。", Severity: "negative"},
	{Key: SheinActionQueuePayloadRebuild, Label: "载荷重建", Description: "需要重建 request draft 或 preview payload。", Severity: "negative"},
	{Key: SheinActionQueueManualReview, Label: "人工备注复核", Description: "处理非阻断的人工备注与人工确认项。", Severity: "warning"},
	{Key: SheinActionQueueSubmitReady, Label: "直接提交", Description: "当前已无优先修复项，可以直接进入提交。", Severity: "positive"},
}

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
		SheinWorkflowStatuses: cloneTaskFacetDescriptors(sheinWorkflowStatusDescriptors),
		SheinWorkQueues:       cloneTaskFacetDescriptors(sheinWorkQueueDescriptors),
		SheinActionQueues:     cloneTaskFacetDescriptors(sheinActionQueueDescriptors),
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
