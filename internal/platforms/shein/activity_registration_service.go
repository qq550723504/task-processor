// Package shein 提供SHEIN活动报名同步服务
package shein

import (
	"fmt"
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/platforms/shein/client"
	"task-processor/internal/platforms/shein/client/api/marketing"

	"github.com/sirupsen/logrus"
)

// ActivityRegistrationService SHEIN活动报名同步服务
type ActivityRegistrationService struct {
	repositoryFactory func(storeID, tenantID int64) api.ActivityRegistrationAPI
}

// NewActivityRegistrationService 创建SHEIN活动报名同步服务
func NewActivityRegistrationService(repositoryFactory func(storeID, tenantID int64) api.ActivityRegistrationAPI) *ActivityRegistrationService {
	return &ActivityRegistrationService{
		repositoryFactory: repositoryFactory,
	}
}

// RegisterActivityProducts 报名活动产品
func (s *ActivityRegistrationService) RegisterActivityProducts(apiClient *shops.ShopAPIClient, tenantID, storeID int64, activityID, activityName string, products []marketing.SkcInfo) (int, error) {
	logrus.WithFields(logrus.Fields{
		"platform":      "SHEIN",
		"tenant_id":     tenantID,
		"store_id":      storeID,
		"activity_id":   activityID,
		"activity_name": activityName,
		"product_count": len(products),
	}).Info("开始报名SHEIN活动产品")

	if len(products) == 0 {
		logrus.Info("没有产品需要报名")
		return 0, nil
	}

	// 为当前店铺创建专用的repository
	repository := s.repositoryFactory(storeID, tenantID)

	// 构建报名配置
	configList := s.buildActivityConfig(products)

	// 调用SHEIN API报名活动
	saveConfigReq := &marketing.SaveConfigRequest{
		ConfigList: configList,
	}

	resp, err := apiClient.MarketingAPI.SaveConfig(saveConfigReq)
	if err != nil {
		return 0, fmt.Errorf("调用SHEIN报名API失败: %w", err)
	}

	if resp.Code != "0" {
		return 0, fmt.Errorf("SHEIN报名API返回错误: %s", resp.Msg)
	}

	// 转换为报名记录格式并保存到后端
	registrationRecords := s.convertToRegistrationRecords(products, tenantID, storeID, activityID, activityName)

	if err := repository.BatchSaveActivityRegistrations(registrationRecords); err != nil {
		return 0, fmt.Errorf("保存报名记录到后端失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"store_id":    storeID,
		"activity_id": activityID,
		"count":       len(products),
	}).Info("SHEIN活动产品报名完成")

	return len(products), nil
}
