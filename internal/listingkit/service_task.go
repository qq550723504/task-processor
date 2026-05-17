package listingkit

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	sheinpub "task-processor/internal/publishing/shein"
	sheinworkspace "task-processor/internal/workspace/shein"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.TenantID == "" {
		req.TenantID = TenantIDFromContext(ctx)
	}
	ctx = WithTenantID(ctx, req.TenantID)
	applyGenerateRequestDefaults(req, s.requestDefaults)
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	task := &Task{
		ID:         uuid.New().String(),
		TenantID:   TenantIDFromContext(ctx),
		UserID:     strings.TrimSpace(req.UserID),
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	if s.taskSubmitter == nil {
		return s.runTaskInline(ctx, task)
	}
	if shouldRunStudioInline(req) {
		return s.enqueueOrRunStudioTask(ctx, task)
	}
	if err := s.enqueueTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	if s.taskSubmitter != nil {
		if err := s.enqueueTask(ctx, task); err != nil {
			return nil, err
		}
		return task, nil
	}

	return s.runTaskInline(ctx, task)
}

func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	if _, err := s.ProcessListingKit(context.WithoutCancel(ctx), task); err != nil {
		refreshed, getErr := s.repo.GetTask(context.WithoutCancel(ctx), task.ID)
		if getErr == nil {
			return refreshed, nil
		}
		return task, nil
	}
	refreshed, err := s.repo.GetTask(context.WithoutCancel(ctx), task.ID)
	if err == nil {
		return refreshed, nil
	}
	return task, nil
}

func (s *service) enqueueTask(ctx context.Context, task *Task) error {
	if s.taskSubmitter == nil {
		return nil
	}
	if err := s.taskSubmitter.Submit(task.ID); err != nil {
		_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
		return fmt.Errorf("failed to submit task: %w", err)
	}
	return nil
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	var resultPayload *ListingKitResult
	if task.Result != nil {
		copied := *task.Result
		tasks, listErr := s.listAssetGenerationTasks(ctx, task.ID)
		if listErr != nil {
			return nil, listErr
		}
		decorateListingKitResultGeneration(&copied, tasks)
		resultPayload = &copied
	}
	result := &TaskResult{
		TaskID:        task.ID,
		TenantID:      task.TenantID,
		Status:        task.Status,
		Result:        resultPayload,
		Error:         task.Error,
		ReviewReasons: reviewReasonsFromTask(task),
		CreatedAt:     task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {
	normalized := normalizeTaskListQuery(query)
	if normalized.TenantID != "" {
		ctx = WithTenantID(ctx, normalized.TenantID)
	}
	tasks, total, err := s.repo.ListTasks(ctx, normalized)
	if err != nil {
		return nil, err
	}

	items := make([]TaskListItem, 0, len(tasks))
	for i := range tasks {
		items = append(items, buildTaskListItem(&tasks[i]))
	}
	var summary *TaskListSummary
	if source, ok := s.repo.(TaskListSummarySource); ok {
		summaryTasks, summaryErr := source.ListTaskSummaryTasks(ctx, summaryTaskListQuery(normalized))
		if summaryErr != nil {
			return nil, summaryErr
		}
		summary = buildTaskListSummary(summaryTasks)
	}
	return &TaskListPage{
		Page:     normalized.Page,
		PageSize: normalized.PageSize,
		Total:    total,
		Summary:  summary,
		Taxonomy: BuildTaskListTaxonomy(),
		Items:    items,
	}, nil
}

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
	if query.SheinWorkflowStatus != "" {
		if buildTaskListItem(task).SheinWorkflowStatus != query.SheinWorkflowStatus {
			return false
		}
	}
	if query.SheinBlockerKey != "" {
		item := buildTaskListItem(task)
		matched := false
		for _, key := range item.SheinBlockingKeys {
			if key == query.SheinBlockerKey {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if query.SheinWarningKey != "" {
		item := buildTaskListItem(task)
		matched := false
		for _, key := range item.SheinWarningKeys {
			if key == query.SheinWarningKey {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if query.SheinWorkQueue != "" {
		if buildTaskListItem(task).SheinWorkQueue != query.SheinWorkQueue {
			return false
		}
	}
	if query.SheinActionQueue != "" {
		if buildTaskListItem(task).SheinActionQueue != query.SheinActionQueue {
			return false
		}
	}
	return true
}

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
		TaskID:     task.ID,
		TenantID:   task.TenantID,
		Status:     task.Status,
		Error:      task.Error,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
		ImageCount: 0,
	}
	if task.Request != nil {
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
	}
	item.ImageCount = taskListImageCount(task)
	if task.Result != nil && task.Result.SDSSync != nil {
		item.SDSSyncStatus = task.Result.SDSSync.Status
	}
	if taskHasPlatform(task, "shein") || (task.Result != nil && task.Result.Shein != nil) {
		item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
	}
	if task.Result != nil && task.Result.Shein != nil {
		item.SheinWorkflowStatus = deriveSheinWorkflowStatus(task.Result.Shein)
		item.SheinBlockingKeys = sheinBlockingKeys(task.Result.Shein)
		item.SheinWarningKeys = sheinWarningKeys(task.Result.Shein)
		item.SheinStatusOverview = buildSheinTaskStatusOverview(task.Result.Shein)
		item.SheinWorkQueue = deriveSheinWorkQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview)
		item.SheinActionQueue = deriveSheinActionQueue(task, item.SheinWorkflowStatus, item.SheinStatusOverview, item.SheinBlockingKeys, item.SheinWarningKeys)
		if latest := latestSheinSubmissionEvent(task.Result.Shein); latest != nil {
			item.SheinLatestSubmissionStatus = latest.Status
			item.SheinLatestSubmissionError = latest.ErrorMessage
		} else if task.Result.Shein.Submission != nil {
			item.SheinLatestSubmissionStatus = task.Result.Shein.Submission.LastStatus
			item.SheinLatestSubmissionError = task.Result.Shein.Submission.LastError
		}
		applySheinSubmissionRemoteSummary(&item, task.Result.Shein)
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusFailed {
		completedAt := task.UpdatedAt
		item.CompletedAt = &completedAt
	}
	return item
}

func buildSheinTaskStatusOverview(pkg *SheinPackage) *sheinworkspace.StatusOverview {
	if pkg == nil {
		return nil
	}
	readiness := buildSheinSubmitReadiness(pkg)
	return sheinworkspace.BuildStatusOverview(pkg.Inspection, toSheinWorkspaceSubmitState(readiness))
}

func sheinBlockingKeys(pkg *SheinPackage) []string {
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness == nil || len(readiness.BlockingItems) == 0 {
		return nil
	}
	keys := make([]string, 0, len(readiness.BlockingItems))
	for _, item := range readiness.BlockingItems {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		keys = append(keys, item.Key)
	}
	return uniqueNonEmptyStrings(keys)
}

func sheinWarningKeys(pkg *SheinPackage) []string {
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness == nil || len(readiness.WarningItems) == 0 {
		return nil
	}
	keys := make([]string, 0, len(readiness.WarningItems))
	for _, item := range readiness.WarningItems {
		if strings.TrimSpace(item.Key) == "" {
			continue
		}
		keys = append(keys, item.Key)
	}
	return uniqueNonEmptyStrings(keys)
}

func deriveSheinWorkQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview) string {
	if task == nil {
		return ""
	}
	switch task.Status {
	case TaskStatusPending, TaskStatusProcessing:
		return SheinWorkQueueGeneration
	case TaskStatusFailed:
		return SheinWorkQueueGenerationFailed
	}
	switch workflowStatus {
	case SheinWorkflowStatusPublished:
		return SheinWorkQueuePublished
	case SheinWorkflowStatusDraftSaved:
		return SheinWorkQueueDraft
	case SheinWorkflowStatusPublishFailed:
		return SheinWorkQueueSubmitFailed
	}
	if overview == nil {
		return ""
	}
	switch overview.Status {
	case "blocked":
		return SheinWorkQueueRepair
	case "ready_with_warnings":
		return SheinWorkQueueReview
	case "ready":
		return SheinWorkQueueSubmitReady
	default:
		return ""
	}
}

func deriveSheinActionQueue(task *Task, workflowStatus string, overview *sheinworkspace.StatusOverview, blockingKeys []string, warningKeys []string) string {
	if task == nil {
		return ""
	}
	switch task.Status {
	case TaskStatusPending, TaskStatusProcessing, TaskStatusFailed:
		return ""
	}
	switch workflowStatus {
	case SheinWorkflowStatusPublished, SheinWorkflowStatusDraftSaved, SheinWorkflowStatusPublishFailed:
		return ""
	}
	for _, key := range blockingKeys {
		if queue := sheinActionQueueForKey(key); queue != "" {
			return queue
		}
	}
	for _, key := range warningKeys {
		if queue := sheinActionQueueForKey(key); queue != "" {
			return queue
		}
	}
	if overview != nil && overview.Status == "ready" {
		return SheinActionQueueSubmitReady
	}
	return ""
}

func sheinActionQueueForKey(key string) string {
	switch strings.TrimSpace(key) {
	case sheinCookieUnavailableIssueCode:
		return SheinActionQueueStoreAuth
	case "category", "category_review":
		return SheinActionQueueClassification
	case "attributes", "attribute_review":
		return SheinActionQueueAttributes
	case "sale_attributes", "variants":
		return SheinActionQueueVariant
	case "images", "final_images", "variant_image_coverage":
		return SheinActionQueueMedia
	case "pricing":
		return SheinActionQueuePricing
	case "final_review":
		return SheinActionQueueFinalReview
	case "source_facts":
		return SheinActionQueueSourceReview
	case "request_draft", "preview_product":
		return SheinActionQueuePayloadRebuild
	case "manual_notes":
		return SheinActionQueueManualReview
	default:
		return ""
	}
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
	if result == nil {
		return 0
	}
	count := 0
	if result.SDSSync != nil {
		count = max(count, sdsSyncImageCount(result.SDSSync))
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
	if pkg == nil {
		return 0
	}
	urls := make([]string, 0)
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		urls = appendImageDraftURLs(urls, pkg.RequestDraft.ImageInfo)
		for _, skc := range pkg.RequestDraft.SKCList {
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

func applySheinSubmissionRemoteSummary(item *TaskListItem, pkg *SheinPackage) {
	if item == nil || pkg == nil || pkg.Submission == nil {
		return
	}
	submission := pkg.Submission
	item.SheinSubmissionRemoteStatus = submission.RemoteStatus
	item.SheinSubmissionRemoteCheckedAt = submission.RemoteCheckedAt
	record := sheinSubmissionRecordForAction(submission, submission.LastAction)
	if record == nil && submission.Publish != nil {
		record = submission.Publish
	}
	if record == nil && submission.SaveDraft != nil {
		record = submission.SaveDraft
	}
	if record != nil {
		item.SheinSubmissionRemoteRecordID = record.RemoteRecordID
		if item.SheinSubmissionRemoteCheckedAt == nil {
			item.SheinSubmissionRemoteCheckedAt = record.RemoteCheckedAt
		}
	}
}

func deriveSheinWorkflowStatus(pkg *SheinPackage) string {
	if pkg == nil {
		return ""
	}
	if latest := latestSheinSubmissionEvent(pkg); latest != nil {
		if latest.Action == "publish" && latest.Status == "success" {
			return SheinWorkflowStatusPublished
		}
		if latest.Action == "save_draft" && latest.Status == "success" {
			return SheinWorkflowStatusDraftSaved
		}
		if latest.Status == "failed" {
			return SheinWorkflowStatusPublishFailed
		}
	}
	if pkg.Submission != nil {
		if pkg.Submission.Publish != nil && pkg.Submission.Publish.Status == "success" {
			return SheinWorkflowStatusPublished
		}
		if pkg.Submission.SaveDraft != nil && pkg.Submission.SaveDraft.Status == "success" {
			return SheinWorkflowStatusDraftSaved
		}
		if pkg.Submission.LastStatus == "failed" {
			return SheinWorkflowStatusPublishFailed
		}
	}
	readiness := buildSheinSubmitReadiness(pkg)
	if readiness != nil && readiness.Ready {
		return SheinWorkflowStatusReadyToSubmit
	}
	return SheinWorkflowStatusPendingConfirmation
}

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
	return pruneEmptyTaskListSummary(summary)
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

func latestSheinSubmissionEvent(pkg *SheinPackage) *sheinpub.SubmissionEvent {
	if pkg == nil || len(pkg.SubmissionEvents) == 0 {
		return nil
	}
	return &pkg.SubmissionEvents[0]
}

func validateRequest(req *GenerateRequest) error {
	if len(req.ImageURLs) == 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.ProductURL) == "" {
		return fmt.Errorf("at least one of image_urls, text, or product_url must be provided")
	}
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}
	if len(req.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	if err := validateSheinStudioAspectRatio(req); err != nil {
		return err
	}
	return nil
}

func validateSheinStudioAspectRatio(req *GenerateRequest) error {
	if req == nil || req.Options == nil || req.Options.SheinStudio == nil || req.Options.SDS == nil {
		return nil
	}
	studio := req.Options.SheinStudio
	sds := req.Options.SDS
	if studio.SourceDesignWidth <= 0 || studio.SourceDesignHeight <= 0 || sds.PrintableWidth <= 0 || sds.PrintableHeight <= 0 {
		return nil
	}
	sourceRatio := float64(studio.SourceDesignWidth) / float64(studio.SourceDesignHeight)
	targetRatio := float64(sds.PrintableWidth) / float64(sds.PrintableHeight)
	if math.Abs(sourceRatio-targetRatio)/targetRatio > 0.25 {
		return fmt.Errorf("shein studio source image ratio differs too much from SDS printable area ratio")
	}
	return nil
}
