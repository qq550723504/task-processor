package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// DailyListingCountAPIClient 每日上架数量API客户端实现
type DailyListingCountAPIClient struct {
	*ManagementAPIClient
}

// GetDailyListingCount 获取每日上架数量
func (c *DailyListingCountAPIClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*api.DailyListingCountRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-daily-listing-count", c.baseURL)

	params := map[string]interface{}{
		"tenantId": tenantID,
		"storeId":  storeID,
		"userId":   userID,
		"date":     date,
	}

	var result APIResponse
	if err := c.apiRequest(http.MethodGet, url, params, &result); err != nil {
		return nil, fmt.Errorf("获取每日上架数量失败: %w", err)
	}

	var count int64
	switch data := result.Data.(type) {
	case float64:
		count = int64(data)
	case int:
		count = int64(data)
	case int64:
		count = data
	default:
		return nil, fmt.Errorf("无法解析响应数据类型: %T, 值: %v", result.Data, result.Data)
	}

	return &api.DailyListingCountRespDTO{
		TenantID: tenantID,
		StoreID:  storeID,
		UserID:   userID,
		Date:     date,
		Count:    count,
	}, nil
}

// SetDailyListingCount 设置每日上架数量
func (c *DailyListingCountAPIClient) SetDailyListingCount(req *api.DailyListingCountSetReqDTO) error {
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-daily-listing-count", c.baseURL)

	var result APIResponse
	result.Data = 0
	if err := c.apiRequest(http.MethodPut, url, req, &result); err != nil {
		return fmt.Errorf("设置每日上架数量失败: %w", err)
	}

	return nil
}

// SetRemainingListingQuota 设置剩余发品额度
func (c *DailyListingCountAPIClient) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-remaining-listing-quota?tenantId=%d&storeId=%d&quota=%d",
		c.baseURL, tenantID, storeID, quota)

	var result APIResponse
	result.Data = false
	if err := c.apiRequest(http.MethodPut, url, nil, &result); err != nil {
		return false, fmt.Errorf("设置剩余发品额度失败: %w", err)
	}

	success, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("无法解析响应数据类型: %T, 值: %v", result.Data, result.Data)
	}

	return success, nil
}
