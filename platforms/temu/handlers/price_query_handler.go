package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// PriceQueryHandler 价格查询处理器
type PriceQueryHandler struct {
	logger *logrus.Entry
}

// PriceQueryRequest 价格查询请求结构体
type PriceQueryRequest struct {
	GoodsID                      string                    `json:"goods_id"`
	MmsSkuMaxRetailPriceQryItems []MaxRetailPriceQueryItem `json:"mms_sku_max_retail_price_qry_items"`
}

// MaxRetailPriceQueryItem 最大零售价查询项
type MaxRetailPriceQueryItem struct {
	BasePriceStr string `json:"base_price_str"`
	Currency     string `json:"currency"`
}

// PriceQueryResponse 价格查询响应结构体
type PriceQueryResponse struct {
	Success   bool              `json:"success"`
	ErrorCode int               `json:"error_code"`
	ErrorMsg  string            `json:"error_msg,omitempty"`
	Result    *PriceQueryResult `json:"result,omitempty"`
}

// PriceQueryResult 价格查询结果
type PriceQueryResult struct {
	MmsSkuMaxRetailPriceItems []MaxRetailPriceResultItem `json:"mms_sku_max_retail_price_items"`
}

// MaxRetailPriceResultItem 最大零售价结果项
type MaxRetailPriceResultItem struct {
	BasePriceStr        string `json:"base_price_str"`
	Currency            string `json:"currency"`
	MaxRetailPriceStr   string `json:"max_retail_price_str"`
	RetailPriceCurrency string `json:"retail_price_currency"`
}

// NewPriceQueryHandler 创建新的价格查询处理器
func NewPriceQueryHandler() *PriceQueryHandler {
	return &PriceQueryHandler{
		logger: logrus.WithField("handler", "PriceQueryHandler"),
	}
}

// Name 返回处理器名称
func (h *PriceQueryHandler) Name() string {
	return "价格查询处理器"
}

// Handle 处理任务
func (h *PriceQueryHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始查询SKU最大零售价格")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 检查是否有商品ID
	if ctx.TemuProduct.GoodsBasic.GoodsID == "" {
		h.logger.Warn("商品ID为空，跳过价格查询")
		return nil
	}

	// 查询价格信息
	err := h.queryMaxRetailPrices(ctx)
	if err != nil {
		h.logger.Errorf("查询价格失败: %v", err)
		return fmt.Errorf("查询价格失败: %w", err)
	}

	h.logger.Info("价格查询完成")
	return nil
}

// queryMaxRetailPrices 查询最大零售价格
func (h *PriceQueryHandler) queryMaxRetailPrices(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始查询TEMU最大零售价格")

	// 检查API客户端
	if ctx.APIClient == nil {
		return fmt.Errorf("API客户端未初始化")
	}

	// 收集所有SKU的供应商价格
	priceItems := h.collectSkuPrices(ctx)
	if len(priceItems) == 0 {
		h.logger.Warn("没有找到SKU价格信息，跳过价格查询")
		return nil
	}

	// 构造价格查询请求
	request := &PriceQueryRequest{
		GoodsID:                      ctx.TemuProduct.GoodsBasic.GoodsID,
		MmsSkuMaxRetailPriceQryItems: priceItems,
	}

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/price/retail/max/info",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Chromium\";v=\"142\", \"Microsoft Edge\";v=\"142\", \"Not_A Brand\";v=\"99\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": request,
	}

	// 发送请求到TEMU API
	response := &PriceQueryResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("发送TEMU价格查询请求失败: %v", err)
		return fmt.Errorf("发送价格查询请求失败: %w", err)
	}

	// 检查响应结果
	if !response.Success {
		h.logger.Errorf("TEMU价格查询API响应失败: success=%v, error_code=%d", response.Success, response.ErrorCode)
		if response.ErrorMsg != "" {
			h.logger.Errorf("错误信息: %s", response.ErrorMsg)
		}
		responseJSON, _ := h.marshalWithoutHTMLEscape(response)
		h.logger.Errorf("完整响应: %s", string(responseJSON))

		return fmt.Errorf("价格查询失败: error_code=%d, message=%s", response.ErrorCode, response.ErrorMsg)
	}

	// 记录查询结果
	h.logger.Infof("价格查询成功: error_code=%d", response.ErrorCode)
	if response.Result != nil {
		h.logger.Infof("获取到 %d 个价格信息", len(response.Result.MmsSkuMaxRetailPriceItems))

		// 更新SKU的最大零售价格
		h.updateSkuMaxRetailPrices(ctx, response.Result.MmsSkuMaxRetailPriceItems)
	}

	// 将查询结果存储到上下文
	ctx.SetData("price_query_response", response)

	return nil
}

// collectSkuPrices 收集所有SKU的供应商价格
func (h *PriceQueryHandler) collectSkuPrices(ctx *pipeline.TaskContext) []MaxRetailPriceQueryItem {
	var priceItems []MaxRetailPriceQueryItem
	priceMap := make(map[string]bool) // 用于去重

	for _, skc := range ctx.TemuProduct.SkcList {
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

			priceItems = append(priceItems, MaxRetailPriceQueryItem{
				BasePriceStr: basePriceStr,
				Currency:     sku.Currency,
			})
			priceMap[key] = true

			h.logger.Debugf("收集价格: %s %s", basePriceStr, sku.Currency)
		}
	}

	h.logger.Infof("收集到 %d 个不同的价格进行查询", len(priceItems))
	return priceItems
}

// updateSkuMaxRetailPrices 更新SKU的最大零售价格
func (h *PriceQueryHandler) updateSkuMaxRetailPrices(ctx *pipeline.TaskContext, priceResults []MaxRetailPriceResultItem) {
	// 创建价格映射
	priceMap := make(map[string]string)
	for _, result := range priceResults {
		key := fmt.Sprintf("%s_%s", result.BasePriceStr, result.Currency)
		priceMap[key] = result.MaxRetailPriceStr
		h.logger.Debugf("价格映射: %s -> %s", key, result.MaxRetailPriceStr)
	}

	// 更新所有SKU的最大零售价格
	updatedCount := 0
	for skcIndex := range ctx.TemuProduct.SkcList {
		for skuIndex := range ctx.TemuProduct.SkcList[skcIndex].SkuList {
			sku := &ctx.TemuProduct.SkcList[skcIndex].SkuList[skuIndex]

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

	h.logger.Infof("成功更新 %d 个SKU的最大零售价格", updatedCount)
}

// marshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
func (h *PriceQueryHandler) marshalWithoutHTMLEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // 关闭HTML转义，避免&被转义为\u0026
	encoder.SetIndent("", "  ")  // 设置缩进以便于阅读

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// 移除最后的换行符
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}
