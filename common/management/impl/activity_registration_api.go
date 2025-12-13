// Package impl 活动报名记录API实现
package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
	"time"
)

// ActivityRegistrationAPIClientImpl 活动报名记录API客户端实现
type ActivityRegistrationAPIClientImpl struct {
	*ManagementAPIClientImpl
	StoreID int64
}

// BatchSaveActivityRegistrations 批量保存活动报名记录
func (c *ActivityRegistrationAPIClientImpl) BatchSaveActivityRegistrations(registrations []*api.ActivityRegistrationDTO) error {
	if len(registrations) == 0 {
		return nil
	}

	// 使用新的 RPC API 路径
	url := fmt.Sprintf("%s/rpc-api/listing/activity-registration/batch-save", c.baseURL)

	// 按平台分组
	platformGroups := c.groupRegistrationsByPlatform(registrations)

	// 按平台分别调用
	for platform, groupRegistrations := range platformGroups {
		reqBody := c.buildRegistrationBatchSaveRequest(platform, groupRegistrations)

		var result APIResponse
		if err := c.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
			return fmt.Errorf("批量保存平台 %s 的活动报名记录失败: %w", platform, err)
		}

		if err := c.ProcessAPIResponse(&result, 0); err != nil {
			return fmt.Errorf("处理平台 %s 的API响应失败: %w", platform, err)
		}
	}

	return nil
}

// groupRegistrationsByPlatform 按平台分组活动报名记录
func (c *ActivityRegistrationAPIClientImpl) groupRegistrationsByPlatform(registrations []*api.ActivityRegistrationDTO) map[string][]*api.ActivityRegistrationDTO {
	platformGroups := make(map[string][]*api.ActivityRegistrationDTO)
	for _, registration := range registrations {
		platform := registration.Platform
		if platform == "" {
			platform = "UNKNOWN"
		}
		platformGroups[platform] = append(platformGroups[platform], registration)
	}
	return platformGroups
}

// buildRegistrationBatchSaveRequest 构建活动报名记录批量保存请求体
func (c *ActivityRegistrationAPIClientImpl) buildRegistrationBatchSaveRequest(platform string, registrations []*api.ActivityRegistrationDTO) map[string]interface{} {
	registrationItems := make([]map[string]interface{}, 0, len(registrations))

	for _, registration := range registrations {
		// 构建站点价格信息列表
		sitePriceInfoList := make([]map[string]interface{}, 0, len(registration.SitePriceInfoList))
		for _, sitePrice := range registration.SitePriceInfoList {
			sitePriceInfo := map[string]interface{}{
				"siteCode":    sitePrice.SiteCode,
				"salePrice":   sitePrice.SalePrice,
				"currency":    sitePrice.Currency,
				"isAvailable": sitePrice.IsAvailable,
			}
			sitePriceInfoList = append(sitePriceInfoList, sitePriceInfo)
		}

		item := map[string]interface{}{
			"skc":                registration.SKC,
			"goodsName":          registration.GoodsName,
			"image":              registration.Image,
			"supplierNo":         registration.SupplierNo,
			"actStock":           registration.ActStock,
			"dropRate":           registration.DropRate,
			"reservedActStock":   registration.ReservedActStock,
			"sitePriceInfoList":  sitePriceInfoList,
			"registrationStatus": registration.RegistrationStatus,
		}

		// 可选字段
		if registration.FailureReason != "" {
			item["failureReason"] = registration.FailureReason
		}
		if registration.CostPrice > 0 {
			item["costPrice"] = registration.CostPrice
		}
		if registration.ProfitRate > 0 {
			item["profitRate"] = registration.ProfitRate
		}

		registrationItems = append(registrationItems, item)
	}

	// 从第一个记录中获取公共信息
	var tenantID int64
	var region string
	var activityID string
	var activityName string
	if len(registrations) > 0 {
		tenantID = registrations[0].TenantID
		region = registrations[0].Region
		activityID = registrations[0].ActivityID
		activityName = registrations[0].ActivityName
	}

	reqBody := map[string]interface{}{
		"platform":            platform,
		"tenantId":            tenantID,
		"storeId":             c.StoreID,
		"region":              region,
		"activityId":          activityID,
		"activityName":        activityName,
		"registrationRecords": registrationItems,
		"registrationTime":    time.Now().UnixMilli(),
	}

	return reqBody
}
