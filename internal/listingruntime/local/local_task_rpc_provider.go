package local

import (
	"encoding/json"
	"fmt"
	"time"

	"task-processor/internal/model"
	"task-processor/internal/pkg/types"
	api "task-processor/internal/taskrpcapi"

	"gorm.io/gorm"
)

type LocalTaskRPCProvider struct {
	db *gorm.DB
}

func NewLocalTaskRPCProvider(provider *LocalDataProvider) *LocalTaskRPCProvider {
	if provider == nil || provider.db == nil {
		return nil
	}
	return &LocalTaskRPCProvider{db: provider.db}
}

func (p *LocalTaskRPCProvider) SubmitTask(req *api.TaskSubmitReqDTO, urgent bool) (*api.TaskSubmitRespDTO, bool, error) {
	if p == nil || p.db == nil || req == nil {
		return nil, false, nil
	}

	priority := req.BusinessPriority
	if urgent && priority > 1 {
		priority = 1
	}
	if priority <= 0 {
		priority = 5
	}
	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	now := time.Now()
	row := localImportTaskRow{
		ID:            req.TaskID,
		TenantID:      req.TenantID,
		StoreID:       req.StoreID,
		Platform:      req.Platform,
		Region:        req.Region,
		CategoryID:    req.CategoryID,
		ProductID:     req.ProductID,
		Status:        model.TaskStatusPending.Int16(),
		Stage:         req.TaskType,
		RetryCount:    0,
		MaxRetryCount: maxRetries,
		Remark:        req.Description,
		Priority:      priority,
		Creator:       "local-task-rpc",
		Updater:       "local-task-rpc",
		CreateTime:    now,
		UpdateTime:    now,
	}
	if row.ID == 0 {
		row.ID = time.Now().UnixNano()
	}
	if err := p.db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		return nil, true, err
	}

	meta := localTaskStatusMetadata(row.Status)
	resp := &api.TaskSubmitRespDTO{
		TaskID:             row.ID,
		Success:            true,
		MessagePriority:    priority,
		RoutingKey:         buildLocalQueueName(row.Platform, row.StoreID),
		QueueName:          buildLocalQueueName(row.Platform, row.StoreID),
		SubmitTime:         now.Format(time.RFC3339),
		EstimatedStartTime: now.Format(time.RFC3339),
		StatusKey:          meta.Key,
		StatusName:         meta.Name,
		CanonicalStatus:    meta.Canonical,
	}
	return resp, true, nil
}

func (p *LocalTaskRPCProvider) SubmitBatchTasks(req *api.TaskBatchSubmitReqDTO) (*api.TaskBatchSubmitRespDTO, bool, error) {
	if p == nil || p.db == nil || req == nil {
		return nil, false, nil
	}
	now := time.Now()
	resp := &api.TaskBatchSubmitRespDTO{
		BatchID:         req.BatchID,
		SubmitTime:      now.Format(time.RFC3339),
		Status:          "success",
		StatusKey:       "SUBMITTED",
		StatusName:      "已提交",
		CanonicalStatus: "completed",
	}
	if resp.BatchID == "" {
		resp.BatchID = fmt.Sprintf("local-%d", now.UnixNano())
	}
	resp.TotalCount = len(req.Tasks)
	for _, taskReq := range req.Tasks {
		item, _, err := p.SubmitTask(&taskReq, false)
		if err != nil {
			resp.FailureCount++
			resp.FailureTasks = append(resp.FailureTasks, api.TaskSubmitRespDTO{
				TaskID:          taskReq.TaskID,
				Success:         false,
				ErrorMessage:    err.Error(),
				StatusKey:       "FAILED",
				StatusName:      "提交失败",
				CanonicalStatus: "failed",
			})
			continue
		}
		resp.SuccessCount++
		resp.SuccessTasks = append(resp.SuccessTasks, *item)
	}
	if resp.FailureCount > 0 {
		resp.Status = "partial_success"
		resp.StatusKey = "PARTIAL_SUCCESS"
		resp.StatusName = "部分成功"
		resp.CanonicalStatus = "completed"
	}
	return resp, true, nil
}

func (p *LocalTaskRPCProvider) GetTaskStatus(taskID int64) (*api.TaskStatusRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	row, found, err := p.getImportTaskRow(taskID)
	if err != nil || !found {
		return nil, found, err
	}
	resp := row.toTaskStatusResp()
	return &resp, true, nil
}

func (p *LocalTaskRPCProvider) GetBatchTaskStatus(taskIDs []int64) ([]api.TaskStatusRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	if len(taskIDs) == 0 {
		return []api.TaskStatusRespDTO{}, true, nil
	}
	var rows []localImportTaskRow
	if err := p.db.Table("listing_product_import_task").Where("id IN ?", taskIDs).Find(&rows).Error; err != nil {
		return nil, true, err
	}
	result := make([]api.TaskStatusRespDTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toTaskStatusResp())
	}
	return result, true, nil
}

func (p *LocalTaskRPCProvider) CancelTask(taskID int64) (*api.TaskActionRespDTO, bool, error) {
	return p.transitionAction(taskID, model.TaskStatusCancelled, "cancel")
}

func (p *LocalTaskRPCProvider) RetryTask(taskID int64) (*api.TaskActionRespDTO, bool, error) {
	return p.transitionAction(taskID, model.TaskStatusPendingRetry, "retry")
}

func (p *LocalTaskRPCProvider) GetQueueStats() (string, bool, error) {
	if p == nil || p.db == nil {
		return "", false, nil
	}
	type countRow struct {
		Status int16 `gorm:"column:status"`
		Count  int64 `gorm:"column:count"`
	}
	var rows []countRow
	if err := p.db.Table("listing_product_import_task").Select("status, COUNT(*) AS count").Group("status").Find(&rows).Error; err != nil {
		return "", true, err
	}
	byStatus := make(map[string]int64, len(rows))
	var pending int64
	var processing int64
	for _, row := range rows {
		meta := localTaskStatusMetadata(row.Status)
		byStatus[meta.Key] = row.Count
		switch meta.Canonical {
		case "pending":
			pending += row.Count
		case "processing":
			processing += row.Count
		}
	}
	payload, err := json.Marshal(map[string]any{
		"source": "local-db",
		"summary": map[string]any{
			"pending":    pending,
			"processing": processing,
			"total":      pending + processing,
		},
		"byStatus": byStatus,
	})
	if err != nil {
		return "", true, err
	}
	return string(payload), true, nil
}

func (p *LocalTaskRPCProvider) transitionAction(taskID int64, target model.TaskStatus, action string) (*api.TaskActionRespDTO, bool, error) {
	if p == nil || p.db == nil {
		return nil, false, nil
	}
	row, found, err := p.getImportTaskRow(taskID)
	if err != nil || !found {
		return nil, found, err
	}
	current, parseErr := model.ParseTaskStatus(row.Status)
	if parseErr != nil {
		return nil, true, parseErr
	}
	if err := model.ValidateTaskStatusTransition(current, target); err != nil && current != target {
		return &api.TaskActionRespDTO{
			TaskID:          taskID,
			Action:          action,
			Success:         false,
			StatusKey:       localTaskStatusMetadata(row.Status).Key,
			StatusName:      localTaskStatusMetadata(row.Status).Name,
			CanonicalStatus: localTaskStatusMetadata(row.Status).Canonical,
			ErrorMessage:    err.Error(),
			ActionTime:      time.Now().Format(time.RFC3339),
		}, true, nil
	}
	updates := map[string]any{
		"status":      target.Int16(),
		"update_time": time.Now(),
		"updater":     "local-task-rpc",
	}
	if target == model.TaskStatusPendingRetry {
		updates["retry_count"] = row.RetryCount + 1
	}
	if err := p.db.Table("listing_product_import_task").Where("id = ?", taskID).Updates(updates).Error; err != nil {
		return nil, true, err
	}
	meta := localTaskStatusMetadata(target.Int16())
	return &api.TaskActionRespDTO{
		TaskID:          taskID,
		Action:          action,
		Success:         true,
		StatusKey:       meta.Key,
		StatusName:      meta.Name,
		CanonicalStatus: meta.Canonical,
		ActionTime:      time.Now().Format(time.RFC3339),
	}, true, nil
}

func (p *LocalTaskRPCProvider) getImportTaskRow(taskID int64) (*localImportTaskRow, bool, error) {
	var row localImportTaskRow
	err := p.db.Table("listing_product_import_task").Where("id = ?", taskID).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func buildLocalQueueName(platform string, storeID int64) string {
	return fmt.Sprintf("%s.tasks.store.%d", platform, storeID)
}

func (r localImportTaskRow) toTaskStatusResp() api.TaskStatusRespDTO {
	meta := localTaskStatusMetadata(r.Status)
	resp := api.TaskStatusRespDTO{
		TaskID:          r.ID,
		Status:          meta.Canonical,
		StatusKey:       meta.Key,
		StatusName:      meta.Name,
		CanonicalStatus: meta.Canonical,
		Platform:        r.Platform,
		Region:          r.Region,
		TaskType:        r.Stage,
		Priority:        r.Priority,
		RetryCount:      r.RetryCount,
		MaxRetries:      r.MaxRetryCount,
		CreatedAt:       flexibleTimePtrFromTime(r.CreateTime),
		QueueName:       buildLocalQueueName(r.Platform, r.StoreID),
		ErrorMessage:    r.ErrorMessage,
		TaskDetails:     r.Remark,
	}
	if r.Status == model.TaskStatusProcessing.Int16() {
		resp.StartedAt = flexibleTimePtrFromTime(r.UpdateTime)
		resp.ProgressPercent = 50
	}
	if meta.Canonical == "completed" || meta.Canonical == "failed" || meta.Canonical == "cancelled" {
		resp.CompletedAt = flexibleTimePtrFromTime(r.UpdateTime)
		resp.ProgressPercent = 100
	}
	return resp
}

func flexibleTimePtrFromTime(ts time.Time) *types.FlexibleTime {
	if ts.IsZero() {
		return nil
	}
	value := types.FlexibleTime{Time: ts}
	return &value
}
