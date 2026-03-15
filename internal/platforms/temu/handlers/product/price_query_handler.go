package product

import (
	"fmt"
	"strconv"
	"task-processor/internal/core/logger"
	"task-processor/internal/pipeline"
	temuapi "task-processor/internal/platforms/temu/api"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// PriceQueryHandler 价格查询处理器
type PriceQueryHandler struct {
	logger *logrus.Entry
}

// NewPriceQueryHandler 创建新的价格查询处理器
func NewPriceQueryHandler() *PriceQueryHandler {
	return &PriceQueryHandler{
		logger: logger.GetGlobalLogger("temu.handlers.price_query"),
	}
}

// Name 返回处理器名称
func (h *PriceQueryHandler) Name() string {
	return "价格查询处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (h *PriceQueryHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始查询SKU最大零售价格")

	// 直接使用强类型上下文
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 检查是否有商品ID
	if temuCtx.TemuProduct.GoodsBasic.GoodsID == "" {
		h.logger.Warn("商品ID为空，跳过价格查询")
		return nil
	}

	// 查询价格信息
	err := h.queryMaxRetailPrices(temuCtx)
	if err != nil {
		h.logger.WithError(err).Error("查询价格失败")
		return fmt.Errorf("查询价格失败: %w", err)
	}

	h.logger.Info("价格查询完成")
	return nil
}

// queryMaxRetailPrices 查询最大零售价格
func (h *PriceQueryHandler) queryMaxRetailPrices(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始查询TEMU最大零售价格")

	// 收集所有SKU的供应商价格
	priceItems := h.collectSkuPrices(temuCtx)
	if len(priceItems) == 0 {
		h.logger.Warn("没有找到SKU价格信息，跳过价格查询")
		return nil
	}

	// 创建ProductAPI实例（QueryMaxRetailPrice 在 product.API 里）
	productAPI := temuapi.NewProductAPI(temuCtx.APIClient, h.logger)

	// 构造价格查询请求
	request := &temuapi.PriceQueryRequest{
		GoodsID:                      temuCtx.TemuProduct.GoodsBasic.GoodsID,
		MmsSkuMaxRetailPriceQryItems: priceItems,
	}

	// 发送请求到TEMU API
	response, err := productAPI.QueryMaxRetailPrice(request)
	if err != nil {
		h.logger.WithError(err).Error("价格查询API调用失败")
		return fmt.Errorf("价格查询失败: %w", err)
	}

	// 记录查询结果
	h.logger.WithFields(logrus.Fields{
		"error_code": response.ErrorCode,
	}).Info("价格查询成功")
	if response.Result != nil {
		h.logger.WithFields(logrus.Fields{
			"price_count": len(response.Result.MmsSkuMaxRetailPriceItems),
		}).Info("获取价格信息")

		// 更新SKU的最大零售价格
		h.updateSkuMaxRetailPrices(temuCtx, response.Result.MmsSkuMaxRetailPriceItems)
	}

	// 将查询结果存储到强类型上下文
	temuCtx.PriceQueryResponse = response

	// 保持兼容性，也设置到通用数据存储（可选）
	temuCtx.SetData("price_query_response", response)

	return nil
}

// collectSkuPrices 收集所有SKU的供应商价格
func (h *PriceQueryHandler) collectSkuPrices(temuCtx *temucontext.TemuTaskContext) []temuapi.MaxRetailPriceQueryItem {
	var priceItems []temuapi.MaxRetailPriceQueryItem
	priceMap := make(map[string]bool) // 用于去重

	// 直接使用强类型上下文中的TEMU产品信息
	if temuCtx.TemuProduct == nil {
		h.logger.Warn("TEMU产品信息为空")
		return priceItems
	}

	for _, skc := range temuCtx.TemuProduct.SkcList {
		for _, sku := range skc.SkuList {
			// 跳过删除的SKU
			if sku.SkuDeleted {
				continue
			}

			// 获取供应商价格
			basePriceStr := sku.SupplierPriceStr
			if basePriceStr == "" || basePriceStr == "0" {
				continue
			}

			// 创建价格查询项的键用于去重
			key := fmt.Sprintf("%s_%s", basePriceStr, sku.Currency)
			if priceMap[key] {
				continue // 已存在，跳过
			}

			priceItems = append(priceItems, temuapi.MaxRetailPriceQueryItem{
				BasePriceStr: basePriceStr,
				Currency:     sku.Currency,
			})
			priceMap[key] = true

			h.logger.Debugf("收集价格: %s %s", basePriceStr, sku.Currency)
		}
	}

	h.logger.WithFields(logrus.Fields{
		"price_item_count": len(priceItems),
	}).Info("收集价格项完成")
	return priceItems
}

// updateSkuMaxRetailPrices 更新SKU的最大零售价格
func (h *PriceQueryHandler) updateSkuMaxRetailPrices(temuCtx *temucontext.TemuTaskContext, priceResults []temuapi.MaxRetailPriceResultItem) {
	// 直接使用强类型上下文中的TEMU产品信息
	if temuCtx.TemuProduct == nil {
		h.logger.Warn("TEMU产品信息为空")
		return
	}

	// 创建价格映射
	priceMap := make(map[string]string)
	for _, result := range priceResults {
		key := fmt.Sprintf("%s_%s", result.BasePriceStr, result.Currency)
		priceMap[key] = result.MaxRetailPriceStr
		h.logger.Debugf("价格映射: %s -> %s", key, result.MaxRetailPriceStr)
	}

	// 更新所有SKU的最大零售价格
	updatedCount := 0
	for skcIndex := range temuCtx.TemuProduct.SkcList {
		for skuIndex := range temuCtx.TemuProduct.SkcList[skcIndex].SkuList {
			sku := &temuCtx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex]

			// 跳过删除的SKU
			if sku.SkuDeleted {
				continue
			}

			// 查找对应的最大零售价格
			key := fmt.Sprintf("%s_%s", sku.SupplierPriceStr, sku.Currency)
			if maxRetailPriceStr, exists := priceMap[key]; exists {
				sku.MaxRetailPriceStr = maxRetailPriceStr

				// 同时更新整数形式的价格（如果需要）
				if maxRetailPrice, err := strconv.ParseFloat(maxRetailPriceStr, 64); err == nil {
					sku.MaxRetailPrice = int(maxRetailPrice * 100) // 转换为分
				}

				h.logger.Debugf("更新SKU最大零售价格: %s -> %s", key, maxRetailPriceStr)
				updatedCount++
			}
		}
	}

	h.logger.WithFields(logrus.Fields{
		"updated_count": updatedCount,
	}).Info("更新SKU最大零售价格完成")
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (h *PriceQueryHandler) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return h.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}
