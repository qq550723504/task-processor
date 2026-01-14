// Package sync 提供SHEIN活动产品同步服务
package sync

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/repo/client"

	"github.com/sirupsen/logrus"
)

// ActivitySyncService SHEIN活动产品同步服务
type ActivitySyncService struct {
	repositoryFactory func(storeID, tenantID int64) api.ActivityProductAPI
	activityFetcher   *ActivityFetcher
	activityConverter *ActivityConverter
}

// NewActivitySyncService 创建SHEIN活动产品同步服务
func NewActivitySyncService(repositoryFactory func(storeID, tenantID int64) api.ActivityProductAPI) *ActivitySyncService {
	return &ActivitySyncService{
		repositoryFactory: repositoryFactory,
		activityFetcher:   NewActivityFetcher(),
		activityConverter: NewActivityConverter(),
	}
}

// SyncActivityProducts 同步SHEIN活动产品
func (s *ActivitySyncService) SyncActivityProducts(apiClient *client.APIClient, tenantID, storeID int64) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":  "SHEIN",
		"tenant_id": tenantID,
		"store_id":  storeID,
		"sync_type": "activity_products",
	}).Info("开始同步SHEIN活动产品")

	// 为当前店铺创建专用的repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 获取所有可报名活动的产品
	allProducts, err := s.activityFetcher.FetchAllActivityProducts(apiClient)
	if err != nil {
		return 0, fmt.Errorf("获取活动产品列表失败: %w", err)
	}

	if len(allProducts) == 0 {
		logrus.Info("没有可同步的活动产品")
		return 0, nil
	}

	// 转换为后端API格式
	backendProducts := s.activityConverter.ConvertToBackendFormat(allProducts, tenantID, storeID)

	// 转换为指针切片
	backendProductPtrs := make([]*api.ActivityProductDTO, len(backendProducts))
	for i := range backendProducts {
		backendProductPtrs[i] = &backendProducts[i]
	}

	// 批量保存到后端
	if err := repository.BatchSaveActivityProducts(backendProductPtrs); err != nil {
		return 0, fmt.Errorf("保存活动产品到后端失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"store_id": storeID,
		"count":    len(allProducts),
	}).Info("SHEIN活动产品同步完成")

	return len(allProducts), nil
}
