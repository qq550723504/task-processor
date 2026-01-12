// Package pricing 提供店铺配置服务
package pricing

import (
	"fmt"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// StoreConfigService 店铺配置服务
type StoreConfigService struct {
	storeID     int64
	storeConfig *api.StoreRespDTO
	logger      *logrus.Entry
}

// NewStoreConfigService 创建店铺配置服务
func NewStoreConfigService(storeID int64, managementClient *management.ClientManager) (*StoreConfigService, error) {
	logger := logrus.WithFields(logrus.Fields{
		"component": "StoreConfigService",
		"storeID":   storeID,
	})

	service := &StoreConfigService{
		storeID: storeID,
		logger:  logger,
	}

	// 加载店铺配置
	if err := service.loadStoreConfig(managementClient); err != nil {
		return nil, fmt.Errorf("加载店铺配置失败: %w", err)
	}

	return service, nil
}

// loadStoreConfig 加载店铺配置
func (s *StoreConfigService) loadStoreConfig(managementClient *management.ClientManager) error {
	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		return fmt.Errorf("店铺客户端未初始化")
	}

	store, err := storeClient.GetStore(s.storeID)
	if err != nil {
		return fmt.Errorf("获取店铺配置失败: %w", err)
	}

	s.storeConfig = store
	s.logger.Infof("店铺配置加载成功: 重新议价=%v, 核价拒绝策略=%s",
		s.IsRebargainEnabled(), s.GetPriceRejectStrategy())
	return nil
}

// IsRebargainEnabled 是否启用重新议价
func (s *StoreConfigService) IsRebargainEnabled() bool {
	if s.storeConfig == nil || s.storeConfig.EnableRebargain == nil {
		return false
	}
	return *s.storeConfig.EnableRebargain
}

// GetPriceType 获取店铺配置的价格类型
func (s *StoreConfigService) GetPriceType() string {
	if s.storeConfig == nil || s.storeConfig.PriceType == "" {
		return "special" // 默认使用特价
	}
	return s.storeConfig.PriceType
}

// GetPriceRejectStrategy 获取核价拒绝策略
func (s *StoreConfigService) GetPriceRejectStrategy() string {
	if s.storeConfig == nil || s.storeConfig.TemuPriceRejectStrategy == "" {
		return "KEEP_ONLINE" // 默认保留在售
	}
	return s.storeConfig.TemuPriceRejectStrategy
}
