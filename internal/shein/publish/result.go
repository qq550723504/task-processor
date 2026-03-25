// Package publish 提供SHEIN平台的各种处理模块，包括发布结果保存等功能
package publish

import (
	"time"

	"task-processor/internal/core/logger"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pkg/timex"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/validation"

	"github.com/sirupsen/logrus"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器
type SavePublishResultHandler struct {
	logger *logrus.Entry
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
// 返回一个用于保存发布结果信息的处理器实例
func NewSavePublishResultHandler() *SavePublishResultHandler {
	return &SavePublishResultHandler{
		logger: logger.GetGlobalLogger("save_publish_result"),
	}
}

// Name 返回处理器名称
// 实现Handler接口，用于标识当前处理器的功能
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理
// 处理发布成功后的信息保存，包括：
// 1. 创建产品导入映射关系
// 2. 记录每日上架成功数量并检查限额
// 3. 更新任务状态为已上架
// 参数:
//   - ctx: 任务上下文，包含产品数据、任务信息等
//
// 返回值:
//   - error: 处理过程中的错误，如果为nil表示处理成功
func (h *SavePublishResultHandler) Handle(ctx *shein.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return shein.NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMapping(ctx); err != nil {
		// 创建映射关系失败可能是网络或系统问题，可重试
		h.logger.Warnf("创建产品导入映射关系失败%v", err)
	}

	// 记录每日上架成功数量并检查限额
	h.recordDailyListingCount(ctx)

	// 更新任务状态为已上架
	updateTaskStatusToPublished(ctx)

	h.logger.Info("发品成功后返回信息保存完成")

	return nil
}

// createProductImportMapping 创建产品导入映射关系
func (h *SavePublishResultHandler) createProductImportMapping(ctx *shein.TaskContext) error {
	if ctx.ManagementClientMgr == nil {
		return shein.NewNonRetryableError("管理客户端管理器未初始化", nil)
	}
	if ctx.Task == nil {
		return shein.NewNonRetryableError("任务信息未初始化", nil)
	}

	mappingClient := ctx.ManagementClientMgr.GetProductImportMappingClient()
	if mappingClient == nil {
		return shein.NewNonRetryableError("产品导入映射客户端未初始化", nil)
	}

	if ctx.SheinResponse == nil || len(ctx.SheinResponse.Info.SKCList) == 0 {
		return nil
	}

	mappingInput, err := buildMappingRequestInput(ctx)
	if err != nil {
		return err
	}

	// 使用 map 去重，避免同一个 SupplierSKU 创建多次映射
	processed := make(map[string]bool)
	createdCount := 0

	for _, skc := range ctx.SheinResponse.Info.SKCList {
		for _, sku := range skc.SKUList {
			if processed[sku.SupplierSKU] {
				h.logger.Debugf("SKU %s 已处理，跳过重复创建", sku.SupplierSKU)
				continue
			}
			processed[sku.SupplierSKU] = true

			// 反向查找 ASIN
			asin := ""
			for a, s := range ctx.AsinSkuMap {
				if s == sku.SupplierSKU {
					asin = a
					break
				}
			}
			if asin == "" {
				if ctx.Task.ProductID != "" {
					asin = ctx.Task.ProductID
					h.logger.Warnf("SKU %s 在AsinSkuMap中未找到对应ASIN，使用任务ProductID: %s", sku.SupplierSKU, asin)
				} else {
					h.logger.Errorf("SKU %s 未找到对应ASIN且任务ProductID为空，跳过", sku.SupplierSKU)
					continue
				}
			}

			createReq := buildMappingReq(mappingInput, asin, sku.SupplierSKU, model.TaskStatusPublished)
			createReq.PlatformProductId = &sku.SKUCode

			// 幂等：已存在则更新，否则创建
			existing, err := mappingClient.GetProductImportMappingByTaskAndSku(ctx.Task.ID, sku.SupplierSKU)
			if err != nil {
				h.logger.Warnf("查询已存在的映射关系失败 (SKU: %s): %v，尝试创建新记录", sku.SupplierSKU, err)
			}

			var id int64
			if existing != nil && existing.ID > 0 {
				createReq.ID = &existing.ID
				if err := mappingClient.UpdateProductImportMapping(createReq); err != nil {
					h.logger.Errorf("更新产品导入映射关系失败 (SKU: %s): %v", sku.SupplierSKU, err)
					continue
				}
				id = existing.ID
				h.logger.Infof("✅ 成功更新产品映射关系 - ID: %d, SKU: %s, PlatformSKU: %s", id, sku.SupplierSKU, sku.SKUCode)
			} else {
				id, err = mappingClient.CreateProductImportMapping(createReq)
				if err != nil {
					h.logger.Errorf("创建产品导入映射关系失败 (SKU: %s): %v", sku.SupplierSKU, err)
					continue
				}
				h.logger.Infof("✅ 成功创建产品映射关系 - ID: %d, SKU: %s, PlatformSKU: %s", id, sku.SupplierSKU, sku.SKUCode)
			}
			createdCount++
		}
	}

	h.logger.Infof("成功创建 %d 个产品导入映射关系", createdCount)
	return nil
}

// recordDailyListingCount 记录每日上架成功数量并检查限额
func (h *SavePublishResultHandler) recordDailyListingCount(ctx *shein.TaskContext) {
	// 检查必要的上下文信息
	if ctx.MemoryManager == nil {
		h.logger.Warn("内存管理器未初始化，跳过每日上架计数")
		return
	}

	if ctx.Task == nil {
		h.logger.Warn("任务信息未初始化，跳过每日上架计数")
		return
	}

	if ctx.StoreInfo == nil {
		h.logger.Warn("店铺信息未初始化，跳过每日上架计数")
		return
	}

	// 检查店铺是否有每日上架限额
	if ctx.StoreInfo.DailyLimit == nil || *ctx.StoreInfo.DailyLimit <= 0 {
		h.logger.Debugf("店铺 %d 没有设置每日上架限额，跳过限额检查", ctx.StoreInfo.ID)
		return
	}

	dailyLimit := *ctx.StoreInfo.DailyLimit
	h.logger.Debugf("店铺 %d 的每日上架限额为: %d，限制类型: %s", ctx.StoreInfo.ID, dailyLimit, ctx.StoreInfo.DailyLimitType)

	// 获取当前日期（格式：YYYY-MM-DD）
	currentDate := timex.NowDate()

	// 根据店铺配置的限制类型计算增加的数量
	increment := h.calculateIncrement(ctx)
	if increment <= 0 {
		h.logger.Warn("计算增量失败，跳过计数更新")
		return
	}

	// 增加每日上架计数
	count := ctx.MemoryManager.DailyCountManager.IncrementCount(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		currentDate,
		increment,
	)

	h.logger.Infof("店铺 %d 在 %s 的上架计数: %d (本次增加: %d, 类型: %s)",
		ctx.StoreInfo.ID, currentDate, count, increment, ctx.StoreInfo.DailyLimitType)

	// 检查是否超过限额
	if count > int64(dailyLimit) {
		h.logger.Warnf("店铺 %d 在 %s 的上架数量(%d)已超过限额(%d)，将暂停上架", ctx.StoreInfo.ID, currentDate, count, dailyLimit)

		// 暂停店铺上架并清理相关缓存
		h.pauseShopWithCacheCleanup(
			ctx,
			"超过每日上架限额",
			24*time.Hour,
		)

		// 记录日志
		h.logger.Infof("已暂停店铺 %d 上架24小时并清理缓存，因为已超过每日限额 %d", ctx.StoreInfo.ID, dailyLimit)

	} else {
		h.logger.Infof("店铺 %d 在 %s 的上架数量(%d)未超过限额(%d)", ctx.StoreInfo.ID, currentDate, count, dailyLimit)
	}
}

// calculateIncrement 委托给 validation.EstimateListingIncrement，发布后 SheinResponse 已填充，可得精确值。
func (h *SavePublishResultHandler) calculateIncrement(ctx *shein.TaskContext) int64 {
	if ctx.SheinResponse == nil {
		h.logger.Warn("SheinResponse为空，无法计算增量")
		return 0
	}
	return validation.EstimateListingIncrement(ctx)
}

// pauseShopWithCacheCleanup 暂停店铺并清理相关缓存
func (h *SavePublishResultHandler) pauseShopWithCacheCleanup(ctx *shein.TaskContext, reason string, duration time.Duration) {
	if ctx.MemoryManager != nil {
		h.logger.Infof("正在清理店铺 %d:%d 的相关缓存", ctx.Task.TenantID, ctx.Task.StoreID)
	}

	ctx.MemoryManager.ShopPauseManager.PauseShop(
		ctx.Task.TenantID,
		ctx.Task.StoreID,
		reason,
		duration,
	)
}

// updateTaskStatusToPublished 更新任务状态为已上架（包级辅助函数）
func updateTaskStatusToPublished(ctx *shein.TaskContext) {
	log := logger.GetGlobalLogger("publish_result")

	if ctx.ManagementClientMgr == nil {
		log.Warn("管理客户端管理器未初始化，跳过状态更新")
		return
	}

	if ctx.Task == nil {
		log.Warn("任务信息未初始化，跳过状态更新")
		return
	}

	importTaskClient := ctx.ManagementClientMgr.GetImportTaskClient()
	if importTaskClient == nil {
		log.Warn("导入任务客户端未初始化，跳过状态更新")
		return
	}

	req := &management_api.ProductImportTaskUpdateReqDTO{
		ID:     ctx.Task.ID,
		Status: model.TaskStatusPublished.Int16(),
	}

	go func() {
		defer recovery.Recover("更新任务状态", log.WithField("task_id", ctx.Task.ID))

		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			log.Errorf("更新任务状态为已上架失败 (TaskID: %d): %v", ctx.Task.ID, err)
		} else {
			log.Infof("✅ 任务状态已更新为已上架 (TaskID: %d)", ctx.Task.ID)
		}
	}()
}
