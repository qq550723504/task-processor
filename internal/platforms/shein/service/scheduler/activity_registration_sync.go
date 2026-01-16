// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	managementapi "task-processor/internal/pkg/management/api"
	managementimpl "task-processor/internal/pkg/management/impl"
	"task-processor/internal/platforms/shein/api/marketing"
)

// SyncRegistrationsToManagement 同步报名记录到管理系统
func (s *activityRegistrationServiceImpl) SyncRegistrationsToManagement(
	ctx context.Context,
	products []marketing.SkcInfo,
	tenantID, storeID int64,
	activityID, activityName string,
) error {
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
