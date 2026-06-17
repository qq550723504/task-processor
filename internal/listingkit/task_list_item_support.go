package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
)

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
		TenantID:        query.TenantID,
		Status:          query.Status,
		Platform:        query.Platform,
		SourceType:      query.SourceType,
		ReadinessStatus: query.ReadinessStatus,
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
		if task.Result.Summary != nil {
			item.SourceType = strings.TrimSpace(task.Result.Summary.SourceType)
		}
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
	submissionProjection := buildSheinSubmissionProjection(pkg)
	if submissionProjection != nil {
		item.SheinSubmissionStatusFields = submissionProjection.StatusFields
	}
	var pod *PodExecutionSummary
	if task != nil && task.Result != nil {
		pod = task.Result.PodExecution
	}
	item.SheinBlockingKeys = sheinBlockingKeysWithPod(pkg, pod)
	item.SheinWarningKeys = sheinWarningKeysWithPod(pkg, pod)
	item.SheinStatusOverview = buildSheinTaskStatusOverviewWithPod(pkg, pod)
	item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
	item.SheinActionQueue = deriveSheinActionQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview, item.SheinBlockingKeys, item.SheinWarningKeys)
	if submissionProjection != nil {
		item.SheinTaskListSubmissionFields = submissionProjection.TaskList
	}
}

func buildSheinTaskStatusOverview(pkg *SheinPackage) *sheinworkspace.StatusOverview {
	return buildSheinTaskStatusOverviewWithPod(pkg, nil)
}

func buildSheinTaskStatusOverviewWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *sheinworkspace.StatusOverview {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	return projection.StatusOverview
}

func sheinBlockingKeys(pkg *SheinPackage) []string {
	return sheinBlockingKeysWithPod(pkg, nil)
}

func sheinBlockingKeysWithPod(pkg *SheinPackage, pod *PodExecutionSummary) []string {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	readiness := projection.Readiness
	if readiness == nil || len(readiness.BlockingItems) == 0 {
		return nil
	}
	return uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.BlockingItems))
}

func sheinWarningKeys(pkg *SheinPackage) []string {
	return sheinWarningKeysWithPod(pkg, nil)
}

func sheinWarningKeysWithPod(pkg *SheinPackage, pod *PodExecutionSummary) []string {
	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, pod)
	if projection == nil {
		return nil
	}
	readiness := projection.Readiness
	if readiness == nil || len(readiness.WarningItems) == 0 {
		return nil
	}
	return uniqueNonEmptyStrings(sheinworkspace.FindKeys(readiness.WarningItems))
}

func deriveSheinWorkQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview) string {
	if task == nil {
		return ""
	}
	return sheinworkspace.BuildTaskWorkQueue(string(task.Status), workflowStatus, overview)
}

func deriveSheinActionQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview, blockingKeys []string, warningKeys []string) string {
	if task == nil {
		return ""
	}
	return sheinworkspace.BuildTaskActionQueue(string(task.Status), workflowStatus, overview, blockingKeys, warningKeys)
}

func taskListImageCount(task *Task) int {
	if task == nil {
		return 0
	}
	if count := listingKitResultImageCount(task.Result); count > 0 {
		return count
	}
	if task.Request != nil {
		return len(task.Request.ImageURLs)
	}
	return 0
}

func listingKitResultImageCount(result *ListingKitResult) int {
	result = normalizeListingKitResultSemanticFields(result)
	if result == nil {
		return 0
	}
	count := 0
	if result.SDSDesignResult != nil {
		count = max(count, sdsSyncImageCount(result.SDSDesignResult))
	}
	if result.Shein != nil {
		count = max(count, sheinPackageImageCount(result.Shein))
	}
	if result.Summary != nil {
		count = max(count, result.Summary.ImageCount)
	}
	return count
}

func sdsSyncImageCount(summary *SDSSyncSummary) int {
	if summary == nil {
		return 0
	}
	urls := append([]string(nil), summary.MockupImageURLs...)
	for _, item := range summary.VariantResults {
		urls = append(urls, item.MockupImageURLs...)
	}
	return len(uniqueNonEmptyStrings(urls))
}

func sheinPackageImageCount(pkg *SheinPackage) int {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return 0
	}
	urls := make([]string, 0)
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		urls = appendImageDraftURLs(urls, pkg.DraftPayload.ImageInfo)
		for _, skc := range pkg.DraftPayload.SKCList {
			urls = appendImageDraftURLs(urls, skc.ImageInfo)
			for _, sku := range skc.SKUList {
				urls = append(urls, sku.MainImage)
			}
		}
	}
	if pkg.Images != nil {
		urls = append(urls, pkg.Images.MainImage, pkg.Images.WhiteBgImage)
		urls = append(urls, pkg.Images.Gallery...)
	}
	if pkg.ImageBundle != nil {
		if pkg.ImageBundle.Main != nil {
			urls = append(urls, pkg.ImageBundle.Main.URL)
		}
		for _, slot := range pkg.ImageBundle.Gallery {
			urls = append(urls, slot.URL)
		}
		for _, slot := range pkg.ImageBundle.Auxiliary {
			urls = append(urls, slot.URL)
		}
	}
	return len(uniqueNonEmptyStrings(urls))
}

func appendImageDraftURLs(urls []string, info *sheinpub.ImageDraft) []string {
	if info == nil {
		return urls
	}
	urls = append(urls, info.MainImage, info.WhiteBg)
	urls = append(urls, info.Gallery...)
	return urls
}
