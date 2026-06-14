package workspace

type FacetDescriptor struct {
	Key         string
	Label       string
	Description string
	Severity    string
}

var workflowStatusDescriptors = []FacetDescriptor{
	{Key: WorkflowStatusPendingConfirmation, Label: "待确认", Description: "资料包已生成，但还未满足正式提交或草稿提交后的终态。", Severity: "neutral"},
	{Key: WorkflowStatusReadyToSubmit, Label: "可提交", Description: "资料包已具备提交前关键骨架，可进入正式发布操作。", Severity: "positive"},
	{Key: WorkflowStatusDraftSaved, Label: "已存草稿", Description: "资料包已经保存为 SHEIN 草稿，可继续人工补充或复核。", Severity: "neutral"},
	{Key: WorkflowStatusPublished, Label: "已发布", Description: "资料包已经完成 SHEIN 发布。", Severity: "positive"},
	{Key: WorkflowStatusPublishFailed, Label: "提交失败", Description: "最近一次保存草稿或正式发布失败，需要人工排查。", Severity: "negative"},
}

var workQueueDescriptors = []FacetDescriptor{
	{Key: WorkQueueGeneration, Label: "生成队列", Description: "任务仍在生成或等待生成。", Severity: "neutral"},
	{Key: WorkQueueGenerationFailed, Label: "生成失败队列", Description: "生成流程失败，需要回看上游数据或任务执行。", Severity: "negative"},
	{Key: WorkQueueRepair, Label: "修复队列", Description: "存在阻断项，暂时不能进入提交态。", Severity: "negative"},
	{Key: WorkQueueReview, Label: "复核队列", Description: "可提交但仍有 warning，建议人工复核。", Severity: "warning"},
	{Key: WorkQueueSubmitReady, Label: "待提交队列", Description: "资料包已准备好，可直接进入提交。", Severity: "positive"},
	{Key: WorkQueueDraft, Label: "草稿队列", Description: "已经保存草稿，等待后续处理。", Severity: "neutral"},
	{Key: WorkQueueSubmitFailed, Label: "提交失败队列", Description: "远端提交流程失败，需要人工重试或修复。", Severity: "negative"},
	{Key: WorkQueuePublished, Label: "已发布队列", Description: "已经完成发布。", Severity: "positive"},
}

var actionQueueDescriptors = []FacetDescriptor{
	{Key: ActionQueueStoreAuth, Label: "店铺授权处理", Description: "SHEIN 店铺登录或 cookie 异常，需先恢复店铺授权。", Severity: "negative"},
	{Key: ActionQueueClassification, Label: "类目处理", Description: "优先处理类目骨架或类目复核问题。", Severity: "negative"},
	{Key: ActionQueueAttributes, Label: "属性处理", Description: "优先处理普通属性映射与属性补齐。", Severity: "negative"},
	{Key: ActionQueueVariant, Label: "规格处理", Description: "优先处理销售属性、SKC/SKU 结构和变体问题。", Severity: "negative"},
	{Key: ActionQueueMedia, Label: "图片处理", Description: "优先处理主图、最终图片和图片覆盖问题。", Severity: "negative"},
	{Key: ActionQueuePricing, Label: "价格处理", Description: "优先处理价格生成与价格确认。", Severity: "negative"},
	{Key: ActionQueueFinalReview, Label: "最终确认", Description: "进入正式提交前的最终人工核对。", Severity: "warning"},
	{Key: ActionQueueSourceReview, Label: "来源复核", Description: "优先复核缺少来源依据的字段。", Severity: "negative"},
	{Key: ActionQueuePayloadRebuild, Label: "载荷重建", Description: "需要重建 request draft 或 preview payload。", Severity: "negative"},
	{Key: ActionQueueManualReview, Label: "人工备注复核", Description: "处理非阻断的人工备注与人工确认项。", Severity: "warning"},
	{Key: ActionQueueSubmitReady, Label: "直接提交", Description: "当前已无优先修复项，可以直接进入提交。", Severity: "positive"},
}

func WorkflowStatusDescriptors() []FacetDescriptor {
	return cloneFacetDescriptors(workflowStatusDescriptors)
}

func WorkQueueDescriptors() []FacetDescriptor {
	return cloneFacetDescriptors(workQueueDescriptors)
}

func ActionQueueDescriptors() []FacetDescriptor {
	return cloneFacetDescriptors(actionQueueDescriptors)
}

func cloneFacetDescriptors(items []FacetDescriptor) []FacetDescriptor {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]FacetDescriptor, len(items))
	copy(cloned, items)
	return cloned
}
