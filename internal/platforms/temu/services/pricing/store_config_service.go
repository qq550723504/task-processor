// Package pricing 提供店铺配置服务
package pricing

import (
	"fmt"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// StoreConfigService 店铺配置服务
type StoreConfigService struct {
	storeID     int64
	storeConfig *api.StoreRespDTO
	logger      *logrus.Entry
}

// NewStoreConfigService 创建店铺配置服务
func NewStoreConfigService(storeID int64, managementClient *management.ClientManager) (StoreConfigProvider, error) {
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
	if managementClient == nil {
		return fmt.Errorf("管理客户端为空")
	}

	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		return fmt.Errorf("店铺客户端未初始化")
	}

	store, err := storeClient.GetStore(s.storeID)
	if err != nil {
		return fmt.Errorf("获取店铺配置失败: %w", err)
	}

	if store == nil {
		return fmt.Errorf("店铺配置为空")
	}

	s.storeConfig = store
	s.logger.Infof("店铺配置加载成功: 重新议价=%v, 价格类型=%s, 核价拒绝策略=%s",
		s.IsRebargainEnabled(), s.GetPriceType(), s.GetPriceRejectStrategy())
	return nil
}

// IsRebargainEnabled 是否启用重新议价
func (s *StoreConfigService) IsRebargainEnabled() bool {
	if s.storeConfig == nil {
		s.logger.Debug("店铺配置为空，重新议价功能默认禁用")
		return false
	}

	if s.storeConfig.EnableRebargain == nil {
		s.logger.Debug("重新议价配置为空，默认禁用")
		return false
	}

	enabled := *s.storeConfig.EnableRebargain
	s.logger.Debugf("重新议价功能状态: %v", enabled)
	return enabled
}

// GetPriceType 获取店铺配置的价格类型
func (s *StoreConfigService) GetPriceType() string {
	const defaultPriceType = "special"

	if s.storeConfig == nil {
		s.logger.Debugf("店铺配置为空，使用默认价格类型: %s", defaultPriceType)
		return defaultPriceType
	}

	if s.storeConfig.PriceType == "" {
		s.logger.Debugf("价格类型配置为空，使用默认值: %s", defaultPriceType)
		return defaultPriceType
	}

	priceType := s.storeConfig.PriceType
	s.logger.Debugf("使用配置的价格类型: %s", priceType)
	return priceType
}

// GetPriceRejectStrategy 获取核价拒绝策略
func (s *StoreConfigService) GetPriceRejectStrategy() string {
	const defaultStrategy = "KEEP_ONLINE"

	if s.storeConfig == nil {
		s.logger.Debugf("店铺配置为空，使用默认拒绝策略: %s", defaultStrategy)
		return defaultStrategy
	}

	if s.storeConfig.TemuPriceRejectStrategy == "" {
		s.logger.Debugf("拒绝策略配置为空，使用默认值: %s", defaultStrategy)
		return defaultStrategy
	}

	strategy := s.storeConfig.TemuPriceRejectStrategy
	s.logger.Debugf("使用配置的拒绝策略: %s", strategy)
	return strategy
}

// GetStoreConfig 获取完整的店铺配置（用于调试和扩展）
func (s *StoreConfigService) GetStoreConfig() *api.StoreRespDTO {
	return s.storeConfig
}

// RefreshConfig 刷新店铺配置
func (s *StoreConfigService) RefreshConfig(managementClient *management.ClientManager) error {
	s.logger.Info("刷新店铺配置")
	return s.loadStoreConfig(managementClient)
}
