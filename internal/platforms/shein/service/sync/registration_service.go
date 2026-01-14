// Package sync 提供SHEIN活动报名同步服务
package sync

import (
	"fmt"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"
	"task-processor/internal/platforms/shein/repo/client"

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
func (s *ActivityRegistrationService) RegisterActivityProducts(apiClient *client.APIClient, tenantID, storeID int64, activityID, activityName string, products []marketing.SkcInfo) (int, error) {
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

	// TODO: 实现 MarketingAPI 调用
	// resp, err := apiClient.MarketingAPI.SaveConfig(saveConfigReq)
	_ = saveConfigReq // 避免未使用变量警告

	// 暂时模拟成功响应
	// if resp.Code != "0" {
	//     return 0, fmt.Errorf("SHEIN报名API返回错误: %s", resp.Msg)
	// }

	logrus.Debug("活动报名API调用(待实现)")

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

// buildActivityConfig 构建活动配置
func (s *ActivityRegistrationService) buildActivityConfig(products []marketing.SkcInfo) []marketing.ActivityConfig {
	// TODO: 实现活动配置构建逻辑
	configList := make([]marketing.ActivityConfig, 0, len(products))
	for _, product := range products {
		// 根据产品信息构建配置项
		config := marketing.ActivityConfig{
			Skc:      product.Skc,
			ActStock: product.Stock,
			// 其他字段根据需要填充
		}
		configList = append(configList, config)
	}
	return configList
}

// convertToRegistrationRecords 转换为报名记录
func (s *ActivityRegistrationService) convertToRegistrationRecords(
	products []marketing.SkcInfo,
	tenantID, storeID int64,
	activityID, activityName string,
) []*api.ActivityRegistrationDTO {
	// TODO: 实现转换逻辑
	records := make([]*api.ActivityRegistrationDTO, 0, len(products))
	for _, product := range products {
		// 转换每个产品为报名记录
		_ = product // 避免未使用变量警告
		// records = append(records, ...)
	}
	return records
}
