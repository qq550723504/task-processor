// Package handler 提供产品数据处理器实现
package handler

import (
	"fmt"
	"task-processor/common/management/api"
	"task-processor/platforms/amazon/internal/model"
)

// ProductDataHandler 产品数据处理器
type ProductDataHandler struct {
	*BaseHandler
}

// NewProductDataHandler 创建产品数据处理器
func NewProductDataHandler() *ProductDataHandler {
	return &ProductDataHandler{
		BaseHandler: NewBaseHandler("获取产品数据"),
	}
}

// Execute 执行产品数据处理
func (h *ProductDataHandler) Execute(services *model.Services, data map[string]interface{}) error {
	h.logger.Info("开始获取产品数据")

	// 检查必要的服务
	if services.ManagementClient == nil {
		return fmt.Errorf("管理客户端未初始化")
	}

	// 从数据中获取产品ID
	productIDValue, exists := data["product_id"]
	if !exists {
		return fmt.Errorf("产品ID不存在")
	}

	productID, ok := productIDValue.(string)
	if !ok {
		return fmt.Errorf("产品ID格式错误")
	}

	// 获取原始JSON数据客户端
	rawDataClient := services.ManagementClient.GetRawJsonDataClient()
	if rawDataClient == nil {
		return fmt.Errorf("原始数据客户端未初始化")
	}

	// 构建请求参数
	req := &api.RawJsonDataReqDTO{
		TenantID:   data["tenant_id"].(int64),
		Platform:   "amazon",
		ProductID:  productID,
		Region:     "US", // 默认美国站
		StoreID:    data["store_id"].(int64),
		CategoryID: 1, // 默认分类
		Creator:    "system",
	}

	// 获取产品原始数据
	rawData, err := rawDataClient.GetRawJsonData(req)
	if err != nil {
		return fmt.Errorf("获取产品原始数据失败: %w", err)
	}

	if rawData == nil {
		return fmt.Errorf("产品原始数据为空")
	}

	// 保存原始数据到上下文
	data["raw_product_data"] = rawData.RawJSONData
	data["data_source"] = rawData.Platform

	h.logger.Infof("产品数据获取成功: ProductID=%s, Source=%s", productID, rawData.Platform)
	return nil
}
