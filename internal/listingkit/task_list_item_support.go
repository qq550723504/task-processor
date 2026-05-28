package listingkit

import "strings"

func taskHasPlatform(task *Task, platform string) bool {
	if task == nil || task.Request == nil {
		return false
	}
	for _, candidate := range task.Request.Platforms {
		if candidate == platform {
			return true
		}
	}
	return false
}

func normalizeTaskListQuery(query *TaskListQuery) *TaskListQuery {
	normalized := &TaskListQuery{Page: 1, PageSize: 20}
	if query != nil {
		*normalized = *query
	}
	if normalized.Page <= 0 {
		normalized.Page = 1
	}
	if normalized.PageSize <= 0 {
		normalized.PageSize = 20
	}
	if normalized.PageSize > 100 {
		normalized.PageSize = 100
	}
	return normalized
}

func summaryTaskListQuery(query *TaskListQuery) *TaskListQuery {
	if query == nil {
		return nil
	}
	return &TaskListQuery{
		TenantID: query.TenantID,
		Status:   query.Status,
		Platform: query.Platform,
	}
}

func buildTaskListItem(task *Task) TaskListItem {
	if task == nil {
		return TaskListItem{}
	}
	item := TaskListItem{
		TaskIdentityFields: TaskIdentityFields{
			TaskID:   task.ID,
			TenantID: task.TenantID,
		},
		TaskListLifecycleFields: TaskListLifecycleFields{
			Status:    task.Status,
			Error:     task.Error,
			CreatedAt: task.CreatedAt,
			UpdatedAt: task.UpdatedAt,
		},
		TaskListDisplayFields: TaskListDisplayFields{
			ImageCount: 0,
		},
	}
	applyTaskListRequestFields(&item, task)
	applyTaskListStoreSnapshot(&item, task)
	item.ImageCount = taskListImageCount(task)
	if task.Result != nil {
		ensureTaskPodExecution(task)
		task.Result = normalizeListingKitResultSemanticFields(task.Result)
		item.PodExecution = clonePodExecutionSummary(task.Result.PodExecution)
	}
	if task.Result != nil && task.Result.SDSDesignResult != nil {
		item.SDSSyncStatus = task.Result.SDSDesignResult.Status
	}
	if taskHasPlatform(task, "shein") || (task.Result != nil && task.Result.Shein != nil) {
		item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
	}
	if task.Result != nil && task.Result.Shein != nil {
		applySheinTaskListFields(&item, task, task.Result.Shein)
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		completedAt := task.UpdatedAt
		item.CompletedAt = &completedAt
	}
	return item
}

func applyTaskListRequestFields(item *TaskListItem, task *Task) {
	if item == nil || task == nil || task.Request == nil {
		return
	}
	item.Platforms = append([]string(nil), task.Request.Platforms...)
	item.Title = task.Request.Text
	if item.Title == "" {
		item.Title = task.Request.ProductURL
	}
	if task.Request.Options != nil && task.Request.Options.SDS != nil {
		item.ProductName = task.Request.Options.SDS.ProductName
		item.VariantLabel = strings.TrimSpace(strings.Join([]string{
			task.Request.Options.SDS.VariantColor,
			task.Request.Options.SDS.VariantSize,
			task.Request.Options.SDS.VariantSKU,
		}, " "))
		if item.Title == "" {
			item.Title = task.Request.Options.SDS.ProductName
		}
	}
	if task.Request.SheinStoreID > 0 {
		item.SheinStoreID = task.Request.SheinStoreID
	}
	if site := strings.TrimSpace(task.Request.Country); site != "" {
		item.SheinStoreSite = site
	}
}

func applyTaskListStoreSnapshot(item *TaskListItem, task *Task) {
	if item == nil || task == nil || task.SheinStoreResolutionSnapshot == nil {
		return
	}
	snapshot := task.SheinStoreResolutionSnapshot
	if snapshot.StoreID > 0 {
		item.SheinStoreID = snapshot.StoreID
	}
	if site := strings.TrimSpace(snapshot.Site); site != "" {
		item.SheinStoreSite = site
	}
	if snapshot.MatchedProfileID > 0 {
		item.SheinStoreProfileID = snapshot.MatchedProfileID
	}
	if !snapshot.ResolvedAt.IsZero() {
		resolvedAt := snapshot.ResolvedAt
		item.SheinStoreResolvedAt = &resolvedAt
	}
	item.SheinStoreStrategy = strings.TrimSpace(snapshot.Strategy)
	item.SheinStoreReason = strings.TrimSpace(snapshot.Reason)
	item.SheinStoreMatchedRuleKinds = append([]string(nil), snapshot.MatchedRuleKinds...)
	item.SheinStoreManualOverride = snapshot.ManualOverride
	item.SheinStoreFallback = snapshot.Fallback
}

func applySheinTaskListFields(item *TaskListItem, task *Task, pkg *SheinPackage) {
	if item == nil || pkg == nil {
		return
	}
	if len(pkg.SiteList) > 0 {
		if site := strings.TrimSpace(pkg.SiteList[0].MainSite); site != "" {
			item.SheinStoreSite = site
		}
	}
	applySheinSubmissionStatusFields(&item.SheinSubmissionStatusFields, pkg)
	var pod *PodExecutionSummary
	if task != nil && task.Result != nil {
		pod = task.Result.PodExecution
	}
	item.SheinBlockingKeys = sheinBlockingKeysWithPod(pkg, pod)
	item.SheinWarningKeys = sheinWarningKeysWithPod(pkg, pod)
	item.SheinStatusOverview = buildSheinTaskStatusOverviewWithPod(pkg, pod)
	item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
	item.SheinActionQueue = deriveSheinActionQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview, item.SheinBlockingKeys, item.SheinWarningKeys)
	applySheinSubmissionRemoteSummary(&item.SheinTaskListSubmissionFields, pkg)
}
