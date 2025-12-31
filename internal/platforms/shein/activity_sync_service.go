// Package shein 提供SHEIN活动产品同步服务
package shein

import (
	"fmt"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// ActivitySyncService SHEIN活动产品同步服务
type ActivitySyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ActivityProductAPI
}

// NewActivitySyncService 创建SHEIN活动产品同步服务
func NewActivitySyncService(repositoryFactory func(storeID, tenantID int64) api.ActivityProductAPI) *ActivitySyncService {
	return &ActivitySyncService{
		repositoryFactory: repositoryFactory,
	}
}

// SyncActivityProducts 同步SHEIN活动产品
func (s *ActivitySyncService) SyncActivityProducts(apiClient *shops.ShopAPIClient, tenantID, storeID int64) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"sync_type": "activity_products",
	}).Info("开始同步SHEIN活动产品")

	// 为当前店铺创建专用的repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 直接使用apiClient中的营销API（ShopAPIClient已包含MarketingAPI）
	// 获取所有可报名活动的产品
	allProducts, err := s.fetchAllActivityProducts(apiClient.MarketingAPI)
	if err != nil {
		return 0, fmt.Errorf("获取活动产品列表失败: %w", err)
	}

	if len(allProducts) == 0 {
		logrus.Info("没有可同步的活动产品")
		return 0, nil
	}

	// 转换为后端API格式
	backendProducts := s.convertToBackendFormat(allProducts, tenantID, storeID)

	// 批量保存到后端
	if err := repository.BatchSaveActivityProducts(backendProducts); err != nil {
		return 0, fmt.Errorf("保存活动产品到后端失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"store_id": storeID,
		"count":    len(allProducts),
	}).Info("SHEIN活动产品同步完成")

	return len(allProducts), nil
}
