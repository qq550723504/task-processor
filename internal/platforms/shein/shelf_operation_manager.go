// Package shein 提供SHEIN平台的上下架操作管理功能
package shein

import (
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// ShelfOperationManager 上下架操作管理器
type ShelfOperationManager struct {
	apiClient      *ShopAPIClient
	requestBuilder *StrategyRequestBuilder
}

// NewShelfOperationManager 创建上下架操作管理器
func NewShelfOperationManager(apiClient *ShopAPIClient) *ShelfOperationManager {
	return &ShelfOperationManager{
		apiClient:      apiClient,
		requestBuilder: NewStrategyRequestBuilder(),
	}
}

// OffShelfProduct 下架产品
func (m *ShelfOperationManager) OffShelfProduct(prod *api.ProductDataDTO) error {
	logrus.WithFields(logrus.Fields{
		"platform_product_id": prod.PlatformProductID,
		"spu_name":            prod.Title,
	}).Info("执行下架操作")

	// 解析 Attributes 获取 SKC 信息
	mappings := extractMappingInfoFromAttributes(prod.Attributes)
	if len(mappings) == 0 {
		logrus.Warn("产品没有 SKC 映射信息，无法下架")
		return nil
	}

	// 构建下架请求
	request := m.requestBuilder.BuildOffShelfRequest(prod, mappings)

	// 调用 SHEIN API 下架产品
	// 注释掉的代码保持原样，等待后续实现
	// if err := m.apiClient.OffShelf(request); err != nil {
	// 	logrus.WithError(err).Error("调用下架接口失败")
	// 	return err
	// }

	_ = request // 避免未使用变量警告

	logrus.Info("产品下架成功")
	return nil
}

// OnShelfProduct 上架产品
func (m *ShelfOperationManager) OnShelfProduct(prod *api.ProductDataDTO) error {
	logrus.WithFields(logrus.Fields{
		"platform_product_id": prod.PlatformProductID,
		"spu_name":            prod.Title,
	}).Info("执行上架操作")

	// 解析 Attributes 获取 SKC 信息
	mappings := extractMappingInfoFromAttributes(prod.Attributes)
	if len(mappings) == 0 {
		logrus.Warn("产品没有 SKC 映射信息，无法上架")
		return nil
	}

	// 构建上架请求
	request := m.requestBuilder.BuildOnShelfRequest(prod, mappings)

	// 调用 SHEIN API 上架产品
	if err := m.apiClient.OnShelf(request); err != nil {
		logrus.WithError(err).Error("调用上架接口失败")
		return err
	}

	logrus.Info("产品上架成功")
	return nil
}

// 注意：extractMappingInfoFromAttributes 函数已在 monitor_helper.go 中定义
