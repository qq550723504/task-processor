package amazonlisting

import (
	"context"
	"sort"
	"strings"
)

func (s *service) GetTaskWorkbench(ctx context.Context, taskID string) (*TaskWorkbench, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskWorkbench(task), nil
}

func (s *service) ListTaskQueue(ctx context.Context, query TaskQueueQuery) (*TaskQueueResult, error) {
	statuses := normalizedStatuses(query.Status)
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	tasks, err := s.repo.ListTasks(ctx, statuses, limit*3)
	if err != nil {
		return nil, err
	}

	items := make([]TaskWorkbench, 0, len(tasks))
	for _, task := range tasks {
		workbench := buildTaskWorkbench(task)
		if !matchesTaskQueueQuery(workbench, query) {
			continue
		}
		items = append(items, *workbench)
		if len(items) >= limit {
			break
		}
	}
	return &TaskQueueResult{
		Items: items,
		Count: len(items),
		Query: TaskQueueQuery{
			Status:      statuses,
			Action:      strings.TrimSpace(query.Action),
			Field:       strings.TrimSpace(query.Field),
			Severity:    strings.TrimSpace(query.Severity),
			Source:      strings.TrimSpace(query.Source),
			ChildStatus: strings.TrimSpace(query.ChildStatus),
			NeedsHuman:  query.NeedsHuman,
			Limit:       limit,
		},
	}, nil
}

func buildTaskWorkbench(task *Task) *TaskWorkbench {
	if task == nil {
		return &TaskWorkbench{}
	}
	workbench := &TaskWorkbench{
		TaskID:      task.ID,
		Status:      task.Status,
		NeedsReview: task.Status == TaskStatusNeedsReview,
	}
	if task.Result == nil {
		return workbench
	}
	workbench.Ready = task.Result.Compliance != nil && task.Result.Compliance.Ready
	workbench.ChildTasks = cloneChildTasks(task.Result.ChildTasks)
	workbench.ReviewItems = append([]AmazonReviewItem(nil), task.Result.ReviewItems...)
	workbench.ReviewSummary = buildReviewItemSummary(task.Result.ReviewItems)
	workbench.ActionBuckets = buildWorkbenchBuckets(task.Result)
	for _, bucket := range workbench.ActionBuckets {
		workbench.TotalItems += bucket.Count
	}
	if len(workbench.ActionBuckets) > 0 {
		workbench.TopAction = workbench.ActionBuckets[0].Action
	}
	return workbench
}

func normalizedStatuses(statuses []TaskStatus) []TaskStatus {
	if len(statuses) == 0 {
		return nil
	}
	seen := map[TaskStatus]struct{}{}
	result := make([]TaskStatus, 0, len(statuses))
	for _, status := range statuses {
		status = TaskStatus(strings.ToLower(strings.TrimSpace(string(status))))
		if status == "" {
			continue
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		result = append(result, status)
	}
	return result
}

func matchesTaskQueueQuery(workbench *TaskWorkbench, query TaskQueueQuery) bool {
	if workbench == nil {
		return false
	}
	action := strings.TrimSpace(query.Action)
	field := strings.TrimSpace(query.Field)
	severity := strings.TrimSpace(query.Severity)
	source := strings.TrimSpace(query.Source)
	childStatus := strings.TrimSpace(query.ChildStatus)

	if query.NeedsHuman != nil {
		matched := false
		for _, item := range workbench.ReviewItems {
			if item.NeedsHuman == *query.NeedsHuman {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if action != "" && !hasReviewItem(workbench.ReviewItems, func(item AmazonReviewItem) bool {
		return strings.EqualFold(strings.TrimSpace(item.Action), action)
	}) {
		return false
	}
	if field != "" && !hasReviewItem(workbench.ReviewItems, func(item AmazonReviewItem) bool {
		return strings.EqualFold(strings.TrimSpace(item.Field), field)
	}) {
		return false
	}
	if severity != "" && !hasReviewItem(workbench.ReviewItems, func(item AmazonReviewItem) bool {
		return strings.EqualFold(strings.TrimSpace(item.Severity), severity)
	}) {
		return false
	}
	if source != "" && !hasReviewItem(workbench.ReviewItems, func(item AmazonReviewItem) bool {
		return strings.Contains(strings.ToLower(strings.TrimSpace(item.Source)), strings.ToLower(source))
	}) {
		return false
	}
	if childStatus != "" {
		found := false
		for _, child := range workbench.ChildTasks {
			if strings.EqualFold(strings.TrimSpace(child.Status), childStatus) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func hasReviewItem(items []AmazonReviewItem, match func(AmazonReviewItem) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
	}
	return false
}

func buildReviewItemSummary(items []AmazonReviewItem) *ReviewItemSummary {
	if len(items) == 0 {
		return nil
	}
	summary := &ReviewItemSummary{
		ByAction:   map[string]int{},
		ByField:    map[string]int{},
		BySeverity: map[string]int{},
	}
	for _, item := range items {
		summary.TotalCount++
		if item.IsBlocking {
			summary.BlockingCount++
		}
		if item.NeedsHuman {
			summary.NeedsHumanCount++
		}
		if action := strings.TrimSpace(item.Action); action != "" {
			summary.ByAction[action]++
		}
		if field := strings.TrimSpace(item.Field); field != "" {
			summary.ByField[field]++
		}
		if severity := strings.TrimSpace(item.Severity); severity != "" {
			summary.BySeverity[severity]++
		}
	}
	return summary
}

func buildWorkbenchBuckets(draft *AmazonListingDraft) []WorkbenchActionBox {
	grouped := map[string]*WorkbenchActionBox{}
	if draft != nil && draft.Submission != nil && draft.Submission.IssueSummary != nil {
		for _, issue := range draft.Submission.IssueSummary.ManualIssues {
			action := strings.TrimSpace(issue.OperatorAction)
			if action == "" {
				action = OperatorActionManualReview
			}
			bucket, exists := grouped[action]
			if !exists {
				bucket = &WorkbenchActionBox{
					Action:   action,
					Label:    operatorActionLabel(action),
					Priority: operatorActionPriority(action),
				}
				grouped[action] = bucket
			}
			bucket.Items = append(bucket.Items, issue)
			bucket.Count++
			if issue.IsBlocking {
				bucket.BlockingCount++
			}
		}
	}
	if draft != nil {
		for _, item := range draft.ReviewItems {
			action := strings.TrimSpace(item.Action)
			if action == "" {
				action = OperatorActionManualReview
			}
			bucket, exists := grouped[action]
			if !exists {
				bucket = &WorkbenchActionBox{
					Action:   action,
					Label:    operatorActionLabel(action),
					Priority: operatorActionPriority(action),
				}
				grouped[action] = bucket
			}
			bucket.Items = append(bucket.Items, AmazonIssue{
				Message:        item.Reason,
				Severity:       item.Severity,
				IsBlocking:     item.IsBlocking,
				OperatorAction: action,
				OperatorAdvice: item.RecommendedFix,
				Target:         item.Field,
			})
			bucket.Count++
			if item.IsBlocking {
				bucket.BlockingCount++
			}
		}
	}

	if len(grouped) == 0 {
		return nil
	}
	result := make([]WorkbenchActionBox, 0, len(grouped))
	for _, bucket := range grouped {
		result = append(result, *bucket)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BlockingCount != result[j].BlockingCount {
			return result[i].BlockingCount > result[j].BlockingCount
		}
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Action < result[j].Action
	})
	for i := range result {
		result[i].Rank = i + 1
	}
	return result
}

func operatorActionLabel(action string) string {
	switch action {
	case OperatorActionFillBrand:
		return "待补品牌"
	case OperatorActionEditBrand:
		return "待改品牌"
	case OperatorActionFillBullets:
		return "待补卖点"
	case OperatorActionEditBullets:
		return "待改卖点"
	case OperatorActionEditTitle:
		return "待改标题"
	case OperatorActionFillMainImage:
		return "待补主图"
	case OperatorActionFillImages:
		return "待补图片"
	case OperatorActionFillPrice:
		return "待补价格"
	case OperatorActionEditPrice:
		return "待改价格"
	case OperatorActionFillSKU:
		return "待补SKU"
	case OperatorActionEditSKU:
		return "待改SKU"
	case OperatorActionCheckCompliance:
		return "待查合规"
	case OperatorActionCheckHazmat:
		return "待查危险品"
	case OperatorActionEditCategory:
		return "待改类目"
	case OperatorActionFillAttributes:
		return "待补属性"
	default:
		return "待人工处理"
	}
}

func operatorActionPriority(action string) int {
	switch action {
	case OperatorActionCheckCompliance:
		return 1
	case OperatorActionCheckHazmat:
		return 2
	case OperatorActionEditCategory:
		return 3
	case OperatorActionFillAttributes:
		return 4
	case OperatorActionFillMainImage:
		return 5
	case OperatorActionFillImages:
		return 6
	case OperatorActionFillBrand, OperatorActionEditBrand:
		return 7
	case OperatorActionEditTitle:
		return 8
	case OperatorActionFillBullets, OperatorActionEditBullets:
		return 9
	case OperatorActionFillPrice, OperatorActionEditPrice:
		return 10
	case OperatorActionFillSKU, OperatorActionEditSKU:
		return 11
	default:
		return 99
	}
}
