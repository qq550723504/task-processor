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

	workbench := &TaskWorkbench{
		TaskID:      task.ID,
		Status:      task.Status,
		NeedsReview: task.Status == TaskStatusNeedsReview,
	}
	if task.Result == nil {
		return workbench, nil
	}

	workbench.Ready = task.Result.Compliance != nil && task.Result.Compliance.Ready
	workbench.ActionBuckets = buildWorkbenchBuckets(task.Result)
	for _, bucket := range workbench.ActionBuckets {
		workbench.TotalItems += bucket.Count
	}
	if len(workbench.ActionBuckets) > 0 {
		workbench.TopAction = workbench.ActionBuckets[0].Action
	}
	return workbench, nil
}

func buildWorkbenchBuckets(draft *AmazonListingDraft) []WorkbenchActionBox {
	if draft == nil || draft.Submission == nil || draft.Submission.IssueSummary == nil {
		return nil
	}

	grouped := map[string]*WorkbenchActionBox{}
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
