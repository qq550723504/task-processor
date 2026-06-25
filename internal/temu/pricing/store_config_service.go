// package pricing 提供店铺配置服务
package pricing

import (
	"context"
	"fmt"
	"task-processor/internal/listingadmin"
	api "task-processor/internal/listingadmin"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// StoreConfigService 店铺配置服务
type StoreConfigService struct {
	storeID     int64
	storeConfig *api.StoreRespDTO
	logger      *logrus.Entry
}

// NewStoreConfigService 创建店铺配置服务
func NewStoreConfigService(storeID int64, runtime runtime) (StoreConfigProvider, error) {
	logger := logger.GetGlobalLogger("StoreConfigService").WithField("storeID", storeID)

	service := &StoreConfigService{
		storeID: storeID,
		logger:  logger,
	}

	// 加载店铺配置
	if err := service.loadStoreConfig(runtime); err != nil {
		return nil, fmt.Errorf("加载店铺配置失败: %w", err)
	}

	return service, nil
}

// loadStoreConfig 加载店铺配置
func (s *StoreConfigService) loadStoreConfig(runtime runtime) error {
	if runtime == nil {
		return fmt.Errorf("核价运行时为空")
	}

	if repo := runtime.GetLocalStoreRepository(); repo != nil {
		store, err := repo.FindStoreByID(context.Background(), s.storeID)
		if err != nil {
			s.logger.WithError(err).Warn("通过本地仓储获取店铺配置失败，回退远程店铺接口")
		} else if store != nil {
			s.storeConfig = storeConfigDTOFromListingStore(store)
			s.logger.Infof("店铺配置通过本地仓储加载成功: 重新议价=%v, 价格类型=%s, 核价拒绝策略=%s",
				s.IsRebargainEnabled(), s.GetPriceType(), s.GetPriceRejectStrategy())
			return nil
		}
	}

	storeClient := runtime.GetStoreAPI()
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
	return s.getConfigString(s.storeConfig.PriceType, "special", "价格类型")
}

// GetPriceRejectStrategy 获取核价拒绝策略
func (s *StoreConfigService) GetPriceRejectStrategy() string {
	return s.getConfigString(s.storeConfig.TemuPriceRejectStrategy, "KEEP_ONLINE", "拒绝策略")
}

// getConfigString 通用：从店铺配置中读取字符串字段，配置为空时返回默认值
func (s *StoreConfigService) getConfigString(value, defaultVal, fieldName string) string {
	if s.storeConfig == nil {
		s.logger.Debugf("店铺配置为空，使用默认%s: %s", fieldName, defaultVal)
		return defaultVal
	}
	if value == "" {
		s.logger.Debugf("%s配置为空，使用默认值: %s", fieldName, defaultVal)
		return defaultVal
	}
	s.logger.Debugf("使用配置的%s: %s", fieldName, value)
	return value
}

// GetStoreConfig 获取完整的店铺配置（用于调试和扩展）
func (s *StoreConfigService) GetStoreConfig() *api.StoreRespDTO {
	return s.storeConfig
}

// RefreshConfig 刷新店铺配置
func (s *StoreConfigService) RefreshConfig(runtime runtime) error {
	s.logger.Info("刷新店铺配置")
	return s.loadStoreConfig(runtime)
}

func storeConfigDTOFromListingStore(store *listingadmin.Store) *api.StoreRespDTO {
	if store == nil {
		return nil
	}
	return &api.StoreRespDTO{
		ID:                       store.ID,
		TenantID:                 store.TenantID,
		StoreID:                  store.StoreID,
		Name:                     store.Name,
		Username:                 store.Username,
		Password:                 store.Password,
		LoginUrl:                 store.LoginURL,
		ShopType:                 store.ShopType,
		Region:                   store.Region,
		Platform:                 store.Platform,
		DailyLimit:               store.DailyLimit,
		DailyLimitType:           store.DailyLimitType,
		FixedStockCount:          store.FixedStockCount,
		SkuGenerateStrategy:      store.SKUGenerateStrategy,
		Prefix:                   store.Prefix,
		Suffix:                   store.Suffix,
		Proxy:                    store.Proxy,
		EnableAutoListing:        store.EnableAutoListing,
		EnableAutoLogin:          store.EnableAutoLogin,
		EnableDraft:              store.EnableDraft,
		EnableAutoPrice:          store.EnableAutoPrice,
		EnableRebargain:          store.EnableRebargain,
		EnableBrandAuthorization: store.EnableBrandAuthorization,
		AuthorizedBrandCode:      store.AuthorizedBrandCode,
		AuthorizedBrandName:      store.AuthorizedBrandName,
		TemuPriceRejectStrategy:  store.TemuPriceRejectStrategy,
		PriceType:                store.PriceType,
		Remark:                   store.Remark,
		Status:                   store.Status,
		Creator:                  store.CreatedBy,
	}
}
