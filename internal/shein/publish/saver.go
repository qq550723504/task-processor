// Package publish 提供SHEIN平台产品发布结果保存功能
package publish

import (
	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/recovery"
	shein "task-processor/internal/shein"
	product "task-processor/internal/shein/api/product"

	"github.com/sirupsen/logrus"
)

// PublishProductSaver 产品发布结果保存器
type PublishProductSaver struct {
	logger *logrus.Entry
}

// NewPublishProductSaver 创建新的产品发布结果保存器
func NewPublishProductSaver() *PublishProductSaver {
	return &PublishProductSaver{
		logger: logger.GetGlobalLogger("publish_saver"),
	}
}

// SavePublishResult 保存发布成功后的所有对应记录
func (s *PublishProductSaver) SavePublishResult(ctx *shein.TaskContext, response *product.SheinResponse) error {
	// 保存SPU名称
	if response.Info.SPUName != "" {
		ctx.ProductData.SPUName = response.Info.SPUName
	}

	// 保存版本信息
	// ...

	// 保存SKC和SKU的对应关系
	if ctx.SupplierSkuMap == nil {
		ctx.SupplierSkuMap = make(map[string]string)
	}

	// 遍历返回的SKC列表，建立ASIN和SKU的对应关系
	for _, skc := range response.Info.SKCList {
		// 遍历每个SKC中的SKU列表
		for _, sku := range skc.SKUList {
			// 保存对应关系到AsinSkuMap中
			ctx.SupplierSkuMap[sku.SKUCode] = sku.SupplierSKU
		}
	}

	return nil
}

// UpdateTaskStatusToDraft 更新任务状态为草稿箱
func (s *PublishProductSaver) UpdateTaskStatusToDraft(ctx *shein.TaskContext) {
	// 检查必要的上下文信息
	if ctx.ManagementClientMgr == nil {
		s.logger.Warn("管理客户端管理器未初始化，跳过状态更新")
		return
	}

	if ctx.Task == nil {
		s.logger.Warn("任务信息未初始化，跳过状态更新")
		return
	}

	// 获取导入任务客户端
	importTaskClient := ctx.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		s.logger.Warn("导入任务客户端未初始化，跳过状态更新")
		return
	}

	// Task.ID已经是int64类型，直接使用
	taskID := ctx.Task.ID

	// 构建更新请求
	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     taskID,
		Status: model.TaskStatusDraft.Int16(),
	}

	// 提前捕获，避免 goroutine 内访问可能已变更的外部状态
	taskLogger := s.logger.WithField("task_id", taskID)

	// 异步更新状态
	go func() {
		defer recovery.Recover("更新任务状态", taskLogger)

		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			taskLogger.Errorf("更新任务状态为草稿箱失败 (TaskID: %d): %v", taskID, err)
		} else {
			taskLogger.Infof("✅ 任务状态已更新为草稿箱 (TaskID: %d)", taskID)
		}
	}()
}
