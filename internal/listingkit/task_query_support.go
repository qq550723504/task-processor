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
	if !queryHasSheinDerivedFilters(query) {
		return true
	}
	item := buildTaskListItem(task)
	if query.SheinWorkflowStatus != "" && item.SheinWorkflowStatus != query.SheinWorkflowStatus {
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

func queryHasSheinDerivedFilters(query *TaskListQuery) bool {
	if query == nil {
		return false
	}
	return query.SheinWorkflowStatus != "" ||
		query.SheinBlockerKey != "" ||
		query.SheinWarningKey != "" ||
		query.SheinWorkQueue != "" ||
		query.SheinActionQueue != ""
}

func taskQueryContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
