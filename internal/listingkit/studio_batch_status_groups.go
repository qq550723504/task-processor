package listingkit

import "strings"

type StudioBatchStatusGroups struct {
	Items []StudioBatchStatusGroup          `json:"items,omitempty"`
	ByKey map[string]StudioBatchStatusGroup `json:"by_key,omitempty"`
}

type StudioBatchStatusGroup struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Count int      `json:"count"`
	IDs   []string `json:"ids,omitempty"`
}

func BuildStudioBatchStatusGroups(detail *StudioBatchDetail) StudioBatchStatusGroups {
	builder := newStudioBatchStatusGroupBuilder()
	if detail == nil {
		return builder.result()
	}

	for _, item := range detail.Items {
		switch item.Item.Status {
		case StudioBatchItemStatusReviewReady:
			if studioBatchItemNeedsFix(item) {
				builder.add("needs_fix", "需修复", item.Item.ID)
			} else {
				builder.add("submittable", "可提交", item.Item.ID)
			}
		case StudioBatchItemStatusGenerating, StudioBatchItemStatusAwaitingMaterialization, StudioBatchItemStatusPending:
			builder.add("processing", "处理中", item.Item.ID)
		case StudioBatchItemStatusFailed:
			builder.add("generation_failed", "生成失败", item.Item.ID)
		}
	}

	for _, task := range detail.FailedTasks {
		builder.add("submission_failed", "提交失败", firstNonEmptyString(task.DesignID, task.Title))
	}
	for _, task := range detail.CreatedTasks {
		key, label := studioBatchCreatedTaskGroup(task)
		builder.add(key, label, firstNonEmptyString(task.ID, task.DesignID, task.Title))
	}
	return builder.result()
}

type studioBatchStatusGroupBuilder struct {
	order []string
	byKey map[string]StudioBatchStatusGroup
}

func newStudioBatchStatusGroupBuilder() *studioBatchStatusGroupBuilder {
	return &studioBatchStatusGroupBuilder{byKey: map[string]StudioBatchStatusGroup{}}
}

func (b *studioBatchStatusGroupBuilder) add(key string, label string, id string) {
	if b == nil || key == "" {
		return
	}
	group, ok := b.byKey[key]
	if !ok {
		b.order = append(b.order, key)
		group = StudioBatchStatusGroup{Key: key, Label: label}
	}
	group.Count++
	if id != "" {
		group.IDs = append(group.IDs, id)
	}
	b.byKey[key] = group
}

func (b *studioBatchStatusGroupBuilder) result() StudioBatchStatusGroups {
	if b == nil {
		return StudioBatchStatusGroups{}
	}
	items := make([]StudioBatchStatusGroup, 0, len(b.order))
	for _, key := range b.order {
		items = append(items, b.byKey[key])
	}
	return StudioBatchStatusGroups{Items: items, ByKey: b.byKey}
}

func studioBatchItemNeedsFix(item StudioBatchItemDetail) bool {
	for _, design := range item.Designs {
		if design.ReviewStatus == StudioMaterializedDesignReviewStatusRejected {
			return true
		}
	}
	return false
}

func studioBatchCreatedTaskGroup(task SheinStudioCreatedTask) (string, string) {
	switch strings.TrimSpace(task.Status) {
	case "":
		if taskPublishedHint(task) {
			return "published", "已发布"
		}
		return "draft_saved", "已保存草稿"
	case "task_created":
		return "task_created", "任务已创建"
	case "needs_review":
		return "needs_review", "待审核"
	case "ready_to_submit":
		return "ready_to_submit", "待提交"
	case "draft_saved":
		return "draft_saved", "草稿已保存"
	case "published":
		return "published", "已发布"
	case "submit_failed":
		return "submission_failed", "提交失败"
	default:
		return "task_created", "任务已创建"
	}
}

func taskPublishedHint(task SheinStudioCreatedTask) bool {
	return containsFold(task.Title, "published") || containsFold(task.Title, "已发布")
}

func containsFold(value string, needle string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}
