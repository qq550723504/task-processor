// Package model 提供统一的数据模型定义
package model

import (
	"time"

	"task-processor/common/types"
)

// UnifiedTask 统一任务结构，兼容三个平台的需求
type UnifiedTask struct {
	// 基础任务信息（基于common/types.Task）
	ID         string `json:"id"`
	TenantID   int64  `json:"tenantId"`
	ProductID  string `json:"productId"`
	Platform   string `json:"platform"` // 源平台（1688, amazon等）
	Region     string `json:"region"`
	StoreID    int64  `json:"storeId"`
	CategoryID int64  `json:"categoryId"`
	CreateTime int64  `json:"createTime"`
	RetryCount int    `json:"retryCount"`
	Priority   int    `json:"priority"`
	Creator    string `json:"creator"`

	// 扩展字段
	TargetPlatform   string                 `json:"target_platform"`   // 目标平台（amazon, temu, shein）
	MarketplaceID    string                 `json:"marketplace_id"`    // 目标市场ID
	LanguageTag      string                 `json:"language_tag"`      // 语言标签
	Currency         string                 `json:"currency"`          // 目标货币
	RawJSONData      string                 `json:"raw_json_data"`     // 原始产品数据
	SourcePlatform   string                 `json:"source_platform"`   // 数据来源平台
	ProcessingConfig map[string]interface{} `json:"processing_config"` // 处理配置
	Metadata         map[string]interface{} `json:"metadata"`          // 元数据

	// 状态管理
	Status      types.TaskStatus `json:"status"`
	StatusMsg   string           `json:"status_msg"`
	StartTime   *time.Time       `json:"start_time,omitempty"`
	EndTime     *time.Time       `json:"end_time,omitempty"`
	ProcessTime time.Duration    `json:"process_time"`
}

// TaskData 任务数据结构，用于传递给处理器
type TaskData struct {
	ProductID        string                 `json:"product_id"`
	StoreID          int64                  `json:"store_id"`
	TenantID         int64                  `json:"tenant_id"`
	RawJSONData      string                 `json:"raw_json_data"`
	SourcePlatform   string                 `json:"source_platform"`
	TargetPlatform   string                 `json:"target_platform"`
	MarketplaceID    string                 `json:"marketplace_id"`
	LanguageTag      string                 `json:"language_tag"`
	Currency         string                 `json:"currency"`
	ProcessingConfig map[string]interface{} `json:"processing_config"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// ToTaskData 将UnifiedTask转换为TaskData
func (t *UnifiedTask) ToTaskData() *TaskData {
	return &TaskData{
		ProductID:        t.ProductID,
		StoreID:          t.StoreID,
		TenantID:         t.TenantID,
		RawJSONData:      t.RawJSONData,
		SourcePlatform:   t.SourcePlatform,
		TargetPlatform:   t.TargetPlatform,
		MarketplaceID:    t.MarketplaceID,
		LanguageTag:      t.LanguageTag,
		Currency:         t.Currency,
		ProcessingConfig: t.ProcessingConfig,
		Metadata:         t.Metadata,
	}
}

// ToCommonTask 转换为common/types.Task
func (t *UnifiedTask) ToCommonTask() *types.Task {
	return &types.Task{
		ID:         t.ID,
		TenantID:   t.TenantID,
		ProductID:  t.ProductID,
		Platform:   t.Platform,
		Region:     t.Region,
		StoreID:    t.StoreID,
		CategoryID: t.CategoryID,
		CreateTime: t.CreateTime,
		RetryCount: t.RetryCount,
		Priority:   t.Priority,
		Creator:    t.Creator,
	}
}

// NewUnifiedTaskFromCommon 从common/types.Task创建UnifiedTask
func NewUnifiedTaskFromCommon(task *types.Task, targetPlatform string) *UnifiedTask {
	return &UnifiedTask{
		ID:               task.ID,
		TenantID:         task.TenantID,
		ProductID:        task.ProductID,
		Platform:         task.Platform,
		Region:           task.Region,
		StoreID:          task.StoreID,
		CategoryID:       task.CategoryID,
		CreateTime:       task.CreateTime,
		RetryCount:       task.RetryCount,
		Priority:         task.Priority,
		Creator:          task.Creator,
		TargetPlatform:   targetPlatform,
		Status:           types.TaskStatusPending,
		Metadata:         make(map[string]interface{}),
		ProcessingConfig: make(map[string]interface{}),
	}
}

// UpdateStatus 更新任务状态
func (t *UnifiedTask) UpdateStatus(status types.TaskStatus, message string) {
	t.Status = status
	t.StatusMsg = message

	now := time.Now()
	if status == types.TaskStatusProcessing && t.StartTime == nil {
		t.StartTime = &now
	}

	if status == types.TaskStatusPublished || status == types.TaskStatusCrawlFailed {
		t.EndTime = &now
		if t.StartTime != nil {
			t.ProcessTime = now.Sub(*t.StartTime)
		}
	}
}

// IsCompleted 检查任务是否已完成
func (t *UnifiedTask) IsCompleted() bool {
	return t.Status == types.TaskStatusPublished ||
		t.Status == types.TaskStatusCrawlFailed ||
		t.Status == types.TaskStatusCancelled ||
		t.Status == types.TaskStatusTerminated
}

// CanRetry 检查任务是否可以重试
func (t *UnifiedTask) CanRetry(maxRetries int) bool {
	return t.RetryCount < maxRetries &&
		(t.Status == types.TaskStatusCrawlFailed || t.Status == types.TaskStatusPendingRetry)
}
