package product

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management/api"
	commontypes "task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/jsonx"
	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
	"task-processor/internal/core/logger"
)

// SavePublishResultHandler 保存发品成功后返回信息处理器（参考SHEIN实现）
type SavePublishResultHandler struct {
	mappingClient api.ProductImportMappingAPI
	memoryManager *state.MemoryManager
	logger        *logrus.Entry
}

// NewSavePublishResultHandler 创建新的保存发品成功后返回信息处理器
func NewSavePublishResultHandler(mappingClient api.ProductImportMappingAPI, memoryManager *state.MemoryManager) *SavePublishResultHandler {
	return &SavePublishResultHandler{
		mappingClient: mappingClient,
		memoryManager: memoryManager,
		logger:        logger.GetGlobalLogger("SavePublishResultHandler"),
	}
}

// Name 返回处理器名称
func (h *SavePublishResultHandler) Name() string {
	return "保存发品成功后返回的信息"
}

// Handle 执行保存发品成功后返回信息处理（兼容pipeline.Handler接口）
func (h *SavePublishResultHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 执行保存发品成功后返回信息处理（强类型上下文）
func (h *SavePublishResultHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始保存发品成功后的信息")

	input, err := buildSavePublishResultInput(temuCtx)
	if err != nil {
		h.logger.Warn("TEMU提交响应数据为空，跳过保存")
		return nil
	}

	if input.Product == nil {
		h.logger.Warn("产品数据不存在，跳过发布结果保存")
		return nil
	}

	// 记录响应数据到日志
	if err := h.logSubmitResponseWithInput(input); err != nil {
		h.logger.Warnf("记录响应数据失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 创建产品导入映射关系
	if err := h.createProductImportMappingWithInput(input); err != nil {
		h.logger.Warnf("创建产品导入映射关系失败: %v", err)
		// 不阻断流程，继续执行
	}

	// 记录每日上架成功数量并检查限额
	h.recordDailyListingCountWithInput(input)

	h.logger.Info("发品成功后返回信息保存完成")
	return nil
}

// createProductImportMapping 创建产品导入映射关系
func (h *SavePublishResultHandler) createProductImportMapping(temuCtx *temucontext.TemuTaskContext) error {
	input, err := buildSavePublishResultInput(temuCtx)
	if err != nil {
		return err
	}

	return h.createProductImportMappingWithInput(input)
}

// getStringValue 安全获取字符串指针的值
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// recordDailyListingCount 记录每日上架成功数量并检查限额（参考SHEIN实现）
func (h *SavePublishResultHandler) recordDailyListingCount(temuCtx *temucontext.TemuTaskContext) {
	input, err := buildSavePublishResultInput(temuCtx)
	if err != nil {
		h.logger.Debugf("build publish result input failed, skip daily listing count: %v", err)
		return
	}

	h.recordDailyListingCountWithInput(input)
}

// calculateIncrementFromContext 根据店铺配置的限制类型计算增量
func (h *SavePublishResultHandler) calculateIncrementFromContext(temuCtx *temucontext.TemuTaskContext, dailyLimitType string) int64 {
	// 检查TEMU产品数据是否存在
	if temuCtx.TemuProduct == nil {
		h.logger.Warn("TEMU产品数据为空，无法计算增量")
		return 0
	}

	temuProduct := temuCtx.TemuProduct

	switch dailyLimitType {
	case "SPU":
		// SPU级别：每个产品算1个
		return 1
	case "SKC":
		// SKC级别：按SKC数量计算
		skcCount := int64(len(temuProduct.SkcList))
		h.logger.Debugf("SKC计数: %d", skcCount)
		return skcCount
	case "SKU":
		// SKU级别：按所有SKU数量计算
		var skuCount int64
		for _, skc := range temuProduct.SkcList {
			skuCount += int64(len(skc.SkuList))
		}
		h.logger.Debugf("SKU计数: %d", skuCount)
		return skuCount
	default:
		// 默认按SPU计算
		h.logger.Warnf("未知的限制类型: %s，默认按SPU计算", dailyLimitType)
		return 1
	}
}

// pauseShopUntilEndOfDay 暂停店铺到当日结束
func (h *SavePublishResultHandler) pauseShopUntilEndOfDay(temuCtx *temucontext.TemuTaskContext, reason string) {
	task := temuCtx.GetTask()
	if task == nil {
		return
	}

	h.memoryManager.ShopPauseManager.PauseShopUntilEndOfDay(
		task.TenantID,
		task.StoreID,
		reason,
	)

	h.logger.Infof("已暂停店铺 %d:%d 上架到当日结束，原因: %s", task.TenantID, task.StoreID, reason)
}

// logSubmitResponse 记录提交响应数据到日志
func (h *SavePublishResultHandler) logSubmitResponse(temuCtx *temucontext.TemuTaskContext, submitResponse *temuapi.SubmitResponse) error {
	input, err := buildSavePublishResultInput(temuCtx)
	if err != nil {
		return err
	}
	input.SubmitResponse = submitResponse
	return h.logSubmitResponseWithInput(input)
}

// logResponseDetails 记录响应详细信息
func (h *SavePublishResultHandler) logResponseDetails(submitResponse *temuapi.SubmitResponse, task *commontypes.Task) {
	if submitResponse == nil || task == nil {
		return
	}

	h.logger.WithFields(logrus.Fields{
		"task_id":    task.ID,
		"success":    submitResponse.Success,
		"error_code": submitResponse.ErrorCode,
		"message":    submitResponse.Message,
	}).Info("TEMU????????????")

	if submitResponse.Result == nil {
		return
	}

	h.logger.WithFields(logrus.Fields{
		"task_id":                task.ID,
		"listing_commit_id":      submitResponse.Result.ListingCommitID,
		"listing_commit_version": submitResponse.Result.ListingCommitVersion,
		"goods_commit_id":        submitResponse.Result.GoodsCommitID,
		"status":                 submitResponse.Result.Status,
		"result_message":         submitResponse.Result.Message,
	}).Info("TEMU?????????")
}

// saveResponseToFile 保存响应数据到文件
func (h *SavePublishResultHandler) saveResponseToFile(taskID int64, responseData []byte) error {
	// 创建文件名
	filename := fmt.Sprintf("submit_response_%d_%s.json", taskID, time.Now().Format("20060102_150405"))

	// 确保目录存在
	logDir := "logs/responses"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建响应日志目录失败: %w", err)
	}

	// 写入文件
	filePath := filepath.Join(logDir, filename)
	if err := os.WriteFile(filePath, responseData, 0644); err != nil {
		return fmt.Errorf("写入响应文件失败: %w", err)
	}

	h.logger.Infof("响应数据已保存到文件: %s", filePath)
	return nil
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *SavePublishResultHandler) marshalWithoutHTMLEscape(v any) ([]byte, error) {
	return jsonx.MarshalWithoutHTMLEscape(v)
}

// buildFilterRuleRange 构建筛选规则范围字符串
func (h *SavePublishResultHandler) buildFilterRuleRange(filterRule *api.FilterRuleRespDTO) string {
	if filterRule == nil {
		return ""
	}

	var rangeParts []string

	// 价格范围
	if filterRule.PriceMin != nil || filterRule.PriceMax != nil {
		var priceRange string
		if filterRule.PriceMin != nil && filterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("价格:%.2f-%.2f", *filterRule.PriceMin, *filterRule.PriceMax)
		} else if filterRule.PriceMin != nil {
			priceRange = fmt.Sprintf("价格:>=%.2f", *filterRule.PriceMin)
		} else if filterRule.PriceMax != nil {
			priceRange = fmt.Sprintf("价格:<=%.2f", *filterRule.PriceMax)
		}
		if priceRange != "" {
			rangeParts = append(rangeParts, priceRange)
		}
	}

	// 库存范围
	if filterRule.StockMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("库存:>=%d", *filterRule.StockMin))
	}

	// 评分范围
	if filterRule.RatingMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("评分:>=%.1f", *filterRule.RatingMin))
	}

	// 评论数量范围
	if filterRule.ReviewCountMin != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("评论数:>=%d", *filterRule.ReviewCountMin))
	}

	// 发货时效
	if filterRule.DeliveryTimeMax != nil {
		rangeParts = append(rangeParts, fmt.Sprintf("发货时效:<=%d天", *filterRule.DeliveryTimeMax))
	}

	// 配送方式
	if filterRule.FulfillmentType != "" && filterRule.FulfillmentType != "ALL" {
		rangeParts = append(rangeParts, fmt.Sprintf("配送:%s", filterRule.FulfillmentType))
	}

	if len(rangeParts) == 0 {
		return ""
	}

	return fmt.Sprintf("[%s]", strings.Join(rangeParts, ","))
}
