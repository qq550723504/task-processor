package listingkit

func buildTaskListSummary(tasks []Task) *TaskListSummary {
	if len(tasks) == 0 {
		return nil
	}
	summary := &TaskListSummary{
		StatusCounts:              make(map[string]int),
		SheinWorkflowStatusCounts: make(map[string]int),
		SheinWorkQueueCounts:      make(map[string]int),
		SheinActionQueueCounts:    make(map[string]int),
		SheinBlockerCounts:        make(map[string]int),
		SheinWarningCounts:        make(map[string]int),
	}
	for i := range tasks {
		item := buildTaskListItem(&tasks[i])
		incrementTaskListSummary(summary, item)
	}
	return pruneEmptyTaskListSummary(summary)
}

func incrementTaskListSummary(summary *TaskListSummary, item TaskListItem) {
	if summary == nil {
		return
	}
	if item.Status != "" {
		summary.StatusCounts[string(item.Status)]++
	}
	if item.SheinWorkflowStatus != "" {
		summary.SheinWorkflowStatusCounts[item.SheinWorkflowStatus]++
	}
	if item.SheinWorkQueue != "" {
		summary.SheinWorkQueueCounts[item.SheinWorkQueue]++
	}
	if item.SheinActionQueue != "" {
		summary.SheinActionQueueCounts[item.SheinActionQueue]++
	}
	for _, key := range item.SheinBlockingKeys {
		if key != "" {
			summary.SheinBlockerCounts[key]++
		}
	}
	for _, key := range item.SheinWarningKeys {
		if key != "" {
			summary.SheinWarningCounts[key]++
		}
	}
}

func pruneEmptyTaskListSummary(summary *TaskListSummary) *TaskListSummary {
	if summary == nil {
		return nil
	}
	if len(summary.StatusCounts) == 0 {
		summary.StatusCounts = nil
	}
	if len(summary.SheinWorkflowStatusCounts) == 0 {
		summary.SheinWorkflowStatusCounts = nil
	}
	if len(summary.SheinWorkQueueCounts) == 0 {
		summary.SheinWorkQueueCounts = nil
	}
	if len(summary.SheinActionQueueCounts) == 0 {
		summary.SheinActionQueueCounts = nil
	}
	if len(summary.SheinBlockerCounts) == 0 {
		summary.SheinBlockerCounts = nil
	}
	if len(summary.SheinWarningCounts) == 0 {
		summary.SheinWarningCounts = nil
	}
	if summary.StatusCounts == nil &&
		summary.SheinWorkflowStatusCounts == nil &&
		summary.SheinWorkQueueCounts == nil &&
		summary.SheinActionQueueCounts == nil &&
		summary.SheinBlockerCounts == nil &&
		summary.SheinWarningCounts == nil {
		return nil
	}
	return summary
}
