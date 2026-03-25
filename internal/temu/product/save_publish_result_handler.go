package product

import (
	"fmt"
	"strings"

	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pipeline"
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
