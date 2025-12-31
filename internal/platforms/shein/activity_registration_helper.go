// Package shein 提供SHEIN活动报名辅助方法
package shein

import (
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/api/marketing"
	"task-processor/internal/platforms/shein/service"
)

// buildActivityConfig 构建活动配置
func (s *ActivityRegistrationService) buildActivityConfig(products []marketing.SkcInfo) []marketing.ActivityConfig {
	helper := service.NewActivityRegistrationHelper()
	return helper.BuildActivityConfig(products)
}

// convertToRegistrationRecords 转换为报名记录格式
func (s *ActivityRegistrationService) convertToRegistrationRecords(products []marketing.SkcInfo, tenantID, storeID int64, activityID, activityName string) []*api.ActivityRegistrationDTO {
	helper := service.NewActivityRegistrationHelper()
	return helper.ConvertToRegistrationRecords(products, tenantID, storeID, activityID, activityName)
}
