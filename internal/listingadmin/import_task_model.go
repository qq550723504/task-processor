package listingadmin

import (
	"strings"

	taskdomain "task-processor/internal/domain/task"
)

func (t listingProductImportTask) toImportTask() ImportTask {
	storeID := t.StoreID
	return ImportTask{
		ID:             t.ID,
		TenantID:       t.TenantID,
		StoreID:        &storeID,
		Platform:       taskdomain.NormalizePlatform(t.Platform),
		TargetPlatform: taskdomain.NormalizePlatform(t.TargetPlatform),
		SourcePlatform: taskdomain.NormalizePlatform(t.SourcePlatform),
		Region:         t.Region,
		CategoryID:     t.CategoryID,
		ProductID:      t.ProductID,
		Status:         t.Status,
		ProcessingNode: t.ProcessingNode,
		ErrorMessage:   t.ErrorMessage,
		ReasonCode:     t.ReasonCode,
		Stage:          t.Stage,
		RetryCount:     t.RetryCount,
		MaxRetryCount:  t.MaxRetryCount,
		Remark:         t.Remark,
		Priority:       t.Priority,
		Creator:        t.Creator,
		Updater:        t.Updater,
		CreateTime:     t.CreateTime,
		UpdateTime:     t.UpdateTime,
		PublishedTime:  t.PublishedTime,
	}
}

func listingProductImportTaskFromImportTask(task ImportTask) listingProductImportTask {
	var storeID int64
	if task.StoreID != nil {
		storeID = *task.StoreID
	}
	var categoryID *int64
	if task.CategoryID != nil {
		value := *task.CategoryID
		categoryID = &value
	}
	platform := taskdomain.NormalizePlatform(task.Platform)
	targetPlatform := taskdomain.NormalizePlatform(task.TargetPlatform)
	sourcePlatform := taskdomain.NormalizePlatform(task.SourcePlatform)
	if sourcePlatform == "" {
		sourcePlatform = platform
	}
	return listingProductImportTask{
		ID:             task.ID,
		TenantID:       task.TenantID,
		StoreID:        storeID,
		Platform:       platform,
		TargetPlatform: targetPlatform,
		SourcePlatform: sourcePlatform,
		Region:         strings.TrimSpace(task.Region),
		CategoryID:     categoryID,
		ProductID:      strings.TrimSpace(task.ProductID),
		Status:         task.Status,
		ProcessingNode: strings.TrimSpace(task.ProcessingNode),
		ErrorMessage:   strings.TrimSpace(task.ErrorMessage),
		ReasonCode:     strings.TrimSpace(task.ReasonCode),
		Stage:          strings.TrimSpace(task.Stage),
		RetryCount:     task.RetryCount,
		MaxRetryCount:  task.MaxRetryCount,
		Remark:         strings.TrimSpace(task.Remark),
		Priority:       task.Priority,
		Creator:        strings.TrimSpace(task.Creator),
		Updater:        strings.TrimSpace(task.Updater),
	}
}

func applyImportTaskDefaults(row *listingProductImportTask) {
	row.Platform = taskdomain.NormalizePlatform(row.Platform)
	row.TargetPlatform = taskdomain.NormalizePlatform(row.TargetPlatform)
	row.SourcePlatform = taskdomain.NormalizePlatform(row.SourcePlatform)
	if row.SourcePlatform == "" {
		row.SourcePlatform = row.Platform
	}
	if row.Region == "" {
		row.Region = "US"
	}
	if row.Priority <= 0 {
		row.Priority = 5
	}
	if row.MaxRetryCount <= 0 {
		row.MaxRetryCount = 3
	}
}

func applyImportTaskAuditFields(row *listingProductImportTask, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
