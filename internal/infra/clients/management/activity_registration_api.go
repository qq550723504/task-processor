package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
	"time"
)

// ActivityRegistrationAPIClient 活动报名记录API客户端实现
type ActivityRegistrationAPIClient struct {
	*ManagementAPIClient
	StoreID int64
}

// BatchSaveActivityRegistrations 批量保存活动报名记录
func (c *ActivityRegistrationAPIClient) BatchSaveActivityRegistrations(registrations []*api.ActivityRegistrationDTO) error {
	if len(registrations) == 0 {
		return nil
	}

	url := fmt.Sprintf("%s/rpc-api/listing/activity-registration/batch-save", c.baseURL)

	platformGroups := c.groupRegistrationsByPlatform(registrations)

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

func (c *ActivityRegistrationAPIClient) groupRegistrationsByPlatform(registrations []*api.ActivityRegistrationDTO) map[string][]*api.ActivityRegistrationDTO {
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

func (c *ActivityRegistrationAPIClient) buildRegistrationBatchSaveRequest(platform string, registrations []*api.ActivityRegistrationDTO) map[string]interface{} {
	registrationItems := make([]map[string]interface{}, 0, len(registrations))

	for _, registration := range registrations {
		sitePriceInfoList := make([]map[string]interface{}, 0, len(registration.SitePriceInfoList))
		for _, sitePrice := range registration.SitePriceInfoList {
			sitePriceInfoList = append(sitePriceInfoList, map[string]interface{}{
				"siteCode":    sitePrice.SiteCode,
				"salePrice":   sitePrice.SalePrice,
				"currency":    sitePrice.Currency,
				"isAvailable": sitePrice.IsAvailable,
			})
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

	var tenantID int64
	var region, activityID, activityName string
	if len(registrations) > 0 {
		tenantID = registrations[0].TenantID
		region = registrations[0].Region
		activityID = registrations[0].ActivityID
		activityName = registrations[0].ActivityName
	}

	return map[string]interface{}{
		"platform":            platform,
		"tenantId":            tenantID,
		"storeId":             c.StoreID,
		"region":              region,
		"activityId":          activityID,
		"activityName":        activityName,
		"registrationRecords": registrationItems,
		"registrationTime":    time.Now().UnixMilli(),
	}
}
