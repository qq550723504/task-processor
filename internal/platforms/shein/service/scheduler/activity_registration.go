// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	managementimpl "task-processor/internal/pkg/management/impl"
	"task-processor/internal/platforms/shein/api/marketing"
	"task-processor/internal/platforms/shein/repo"

	"github.com/sirupsen/logrus"
)

// ActivityRegistrationService 活动报名服务接口
type ActivityRegistrationService interface {
	// FetchAvailableProducts 获取可报名活动的产品列表
	FetchAvailableProducts(ctx context.Context) ([]marketing.SkcInfo, error)

	// RegisterProducts 自动报名产品到活动
	RegisterProducts(ctx context.Context, products []marketing.SkcInfo) (int, error)

	// QueryPromotionGoods 查询促销活动商品列表
	QueryPromotionGoods(ctx context.Context, req *marketing.QueryPromotionGoodsRequest) (*marketing.QueryPromotionGoodsResponse, error)

	// CalculateSupplyPrice 计算供货价格和利润
	CalculateSupplyPrice(ctx context.Context, req *marketing.CalculateSupplyPriceRequest) (*marketing.CalculateSupplyPriceResponse, error)

	// CreateTimeLimitedDiscount 创建限时折扣活动
	CreateTimeLimitedDiscount(ctx context.Context, req *marketing.CreateActivityRequest) (*marketing.CreateActivityResponse, error)

	// AutoCreateTimeLimitedDiscount 自动创建限时折扣活动（完整流程）
	AutoCreateTimeLimitedDiscount(ctx context.Context, config TimeLimitedDiscountConfig) error
}

// activityRegistrationServiceImpl 活动报名服务实现
type activityRegistrationServiceImpl struct {
	managementClient *management.ClientManager
	marketingAPI     repo.MarketingAPIInterface
	logger           *logrus.Entry
}

// NewActivityRegistrationService 创建活动报名服务
func NewActivityRegistrationService(
	managementClient *management.ClientManager,
	marketingAPI repo.MarketingAPIInterface,
) ActivityRegistrationService {
	return &activityRegistrationServiceImpl{
		managementClient: managementClient,
		marketingAPI:     marketingAPI,
		logger:           logrus.WithField("component", "ActivityRegistrationService"),
	}
}

// FetchAvailableProducts 获取可报名活动的产品列表
func (s *activityRegistrationServiceImpl) FetchAvailableProducts(ctx context.Context) ([]marketing.SkcInfo, error) {
	s.logger.Debug("开始获取可报名活动的产品列表")

	var allProducts []marketing.SkcInfo

	// 分页获取所有可报名的产品
	pageNum := 1
	const pageSize = 100

	for {
		req := &marketing.GetAvailableSkcListRequest{
			PageNum:  pageNum,
			PageSize: pageSize,
		}

		// 调用 SHEIN API 获取可报名产品列表
		response, err := s.marketingAPI.GetAvailableSkcList(req)
		if err != nil {
			s.logger.Errorf("获取可报名产品列表失败(页面%d): %v", pageNum, err)
			return nil, fmt.Errorf("获取可报名产品列表失败: %w", err)
		}

		if response.Info == nil {
			break
		}

		s.logger.Debugf("页面%d获取到%d个可报名产品", pageNum, len(response.Info.SkcInfoList))

		allProducts = append(allProducts, response.Info.SkcInfoList...)

		// 如果当前页数据少于页面大小，说明已经到最后一页
		if len(response.Info.SkcInfoList) < pageSize {
			break
		}
		pageNum++
	}

	s.logger.Infof("获取可报名产品列表完成，共%d个产品", len(allProducts))
	return allProducts, nil
}

// RegisterProducts 自动报名产品到活动
func (s *activityRegistrationServiceImpl) RegisterProducts(ctx context.Context, products []marketing.SkcInfo) (int, error) {
	s.logger.WithField("product_count", len(products)).Debug("开始报名产品到活动")

	if len(products) == 0 {
		s.logger.Info("没有产品需要报名")
		return 0, nil
	}

	// 构建活动配置列表
	configList := s.buildActivityConfigs(products)

	// 调用 SHEIN API 保存活动配置（报名）
	saveReq := &marketing.SaveConfigRequest{
		ConfigList: configList,
	}

	response, err := s.marketingAPI.SaveConfig(saveReq)
	if err != nil {
		s.logger.Errorf("保存活动配置失败: %v", err)
		return 0, fmt.Errorf("保存活动配置失败: %w", err)
	}

	if response.Code != "0" {
		return 0, fmt.Errorf("保存活动配置失败: %s", response.Msg)
	}

	s.logger.Infof("成功报名 %d 个产品到活动", len(products))
	return len(products), nil
}

// buildActivityConfigs 构建活动配置列表
func (s *activityRegistrationServiceImpl) buildActivityConfigs(products []marketing.SkcInfo) []marketing.ActivityConfig {
	configList := make([]marketing.ActivityConfig, 0, len(products))

	for _, product := range products {
		// 跳过已配置的产品
		if product.IsConfigured {
			s.logger.Debugf("产品 [%s] 已配置，跳过", product.Skc)
			continue
		}

		// 构建活动配置
		config := marketing.ActivityConfig{
			Skc:               product.Skc,
			ActStock:          product.Stock, // 使用全部库存作为活动库存
			DropRate:          10,            // 默认降价10%
			ReservedActStock:  0,             // 不预留库存
			SitePriceInfoList: s.convertSitePriceInfo(product.SitePriceInfoList),
		}

		configList = append(configList, config)
	}

	return configList
}

// convertSitePriceInfo 转换站点价格信息
func (s *activityRegistrationServiceImpl) convertSitePriceInfo(siteInfoList []marketing.SitePriceInfo) []marketing.ActivitySitePriceInfo {
	activitySiteInfoList := make([]marketing.ActivitySitePriceInfo, 0, len(siteInfoList))

	for _, siteInfo := range siteInfoList {
		activitySiteInfo := marketing.ActivitySitePriceInfo{
			SiteCode:    siteInfo.SiteCode,
			SalePrice:   siteInfo.SalePrice * 0.9, // 降价10%
			Currency:    siteInfo.Currency,
			IsAvailable: siteInfo.IsAvailable,
		}
		activitySiteInfoList = append(activitySiteInfoList, activitySiteInfo)
	}

	return activitySiteInfoList
}

// SyncRegistrationsToManagement 同步报名记录到管理系统
func (s *activityRegistrationServiceImpl) SyncRegistrationsToManagement(ctx context.Context, products []marketing.SkcInfo, tenantID, storeID int64, activityID, activityName string) error {
	s.logger.WithField("count", len(products)).Debug("开始同步报名记录到管理系统")

	if len(products) == 0 {
		s.logger.Info("没有报名记录需要同步")
		return nil
	}

	// 转换为报名记录格式
	registrations := s.convertToRegistrations(products, tenantID, storeID, activityID, activityName)

	// 创建 ActivityRegistrationAPI 客户端
	baseClient := s.managementClient.GetClient()
	activityRegistrationAPI := &managementimpl.ActivityRegistrationAPIClientImpl{
		ManagementAPIClientImpl: baseClient,
		StoreID:                 storeID,
	}

	// 保存到管理系统
	if err := activityRegistrationAPI.BatchSaveActivityRegistrations(registrations); err != nil {
		s.logger.Errorf("保存报名记录失败: %v", err)
		return fmt.Errorf("保存报名记录失败: %w", err)
	}

	s.logger.Infof("成功同步 %d 条报名记录到管理系统", len(registrations))
	return nil
}

// convertToRegistrations 转换为报名记录
func (s *activityRegistrationServiceImpl) convertToRegistrations(
	products []marketing.SkcInfo,
	tenantID, storeID int64,
	activityID, activityName string,
) []*managementapi.ActivityRegistrationDTO {
	registrations := make([]*managementapi.ActivityRegistrationDTO, 0, len(products))

	for _, product := range products {
		// 转换站点价格信息
		sitePriceInfoList := make([]managementapi.ActivityRegistrationSitePriceDTO, 0, len(product.SitePriceInfoList))
		for _, siteInfo := range product.SitePriceInfoList {
			sitePriceInfoList = append(sitePriceInfoList, managementapi.ActivityRegistrationSitePriceDTO{
				SiteCode:  siteInfo.SiteCode,
				SalePrice: siteInfo.SalePrice * 0.9, // 活动价格（降价10%）
				Currency:  siteInfo.Currency,
			})
		}

		registration := &managementapi.ActivityRegistrationDTO{
			SKC:                product.Skc,
			SupplierNo:         product.SupplierNo,
			ActStock:           product.Stock,
			DropRate:           10, // 降价10%
			ReservedActStock:   0,
			SitePriceInfoList:  sitePriceInfoList,
			RegistrationStatus: 1, // 1:已报名
			TenantID:           tenantID,
			StoreID:            storeID,
			Region:             "US", // 默认美国站
			ActivityID:         activityID,
			ActivityName:       activityName,
		}

		registrations = append(registrations, registration)
	}

	return registrations
}
