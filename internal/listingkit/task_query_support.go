package listingkit

func TaskMatchesListQuery(task *Task, query *TaskListQuery) bool {
	if task == nil {
		return false
	}
	if query == nil {
		return true
	}
	if query.Status != "" && string(task.Status) != query.Status {
		return false
	}
	if query.Platform != "" && !taskHasPlatform(task, query.Platform) {
		return false
	}
	if !queryHasDerivedTaskItemFilters(query) {
		return true
	}
	item := buildTaskListItem(task)
	if query.SourceType != "" && item.SourceType != query.SourceType {
		return false
	}
	if query.ReadinessStatus != "" && taskListReadinessStatus(item) != query.ReadinessStatus {
		return false
	}
	if query.SheinWorkflowStatus != "" && item.SheinWorkflowStatus != query.SheinWorkflowStatus {
		return false
	}
	if query.SheinSubmissionStatus != "" && item.SheinLatestSubmissionStatus != query.SheinSubmissionStatus {
		return false
	}
	if query.SheinBlockerKey != "" && !taskQueryContainsString(item.SheinBlockingKeys, query.SheinBlockerKey) {
		return false
	}
	if query.SheinWarningKey != "" && !taskQueryContainsString(item.SheinWarningKeys, query.SheinWarningKey) {
		return false
	}
	if query.SheinWorkQueue != "" && item.SheinWorkQueue != query.SheinWorkQueue {
		return false
	}
	if query.SheinActionQueue != "" && item.SheinActionQueue != query.SheinActionQueue {
		return false
	}
	return true
}

func queryHasDerivedTaskItemFilters(query *TaskListQuery) bool {
	if query == nil {
		return false
	}
	return query.SourceType != "" ||
		query.ReadinessStatus != "" ||
		queryHasSheinDerivedFilters(query)
}

func queryHasSheinDerivedFilters(query *TaskListQuery) bool {
	if query == nil {
		return false
	}
	return query.SheinWorkflowStatus != "" ||
		query.SheinSubmissionStatus != "" ||
		query.SheinBlockerKey != "" ||
		query.SheinWarningKey != "" ||
		query.SheinWorkQueue != "" ||
		query.SheinActionQueue != ""
}

func taskListReadinessStatus(item TaskListItem) string {
	if len(item.SheinBlockingKeys) > 0 {
		return "blocked"
	}
	if len(item.SheinWarningKeys) > 0 {
		return "warning"
	}
	if item.SheinStatusOverview != nil || item.SheinWorkflowStatus != "" {
		return "ready"
	}
	return ""
}

func taskQueryContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
