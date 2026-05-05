package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
)

// DailyListingCountAPIClient 每日上架数量API客户端实现
type DailyListingCountAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
}

// GetDailyListingCount 获取每日上架数量
func (c *DailyListingCountAPIClient) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*api.DailyListingCountRespDTO, error) {
	if c.localDataProvider != nil {
		if resp, err := c.localDataProvider.GetDailyListingCount(tenantID, storeID, userID, date); err != nil || resp != nil {
			return resp, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-daily-listing-count", c.baseURL)

	params := map[string]any{
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
	if c.localDataProvider != nil {
		return c.localDataProvider.SetDailyListingCount(req)
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-daily-listing-count", c.baseURL)

	var result APIResponse
	result.Data = 0
	if err := c.apiRequest(http.MethodPut, url, req, &result); err != nil {
		return fmt.Errorf("设置每日上架数量失败: %w", err)
	}

	return nil
}

// TryConsumeDailyQuota 原子预占每日上架额度
func (c *DailyListingCountAPIClient) TryConsumeDailyQuota(req *api.TryConsumeDailyQuotaReqDTO) (*api.TryConsumeDailyQuotaRespDTO, error) {
	if c.localDataProvider != nil {
		return c.localDataProvider.TryConsumeDailyQuota(req)
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/try-consume-daily-quota", c.baseURL)

	result, err := getTypedResult[api.TryConsumeDailyQuotaRespDTO](c.ManagementAPIClient, http.MethodPut, url, req)
	if err != nil {
		return nil, fmt.Errorf("原子预占每日上架额度失败: %w", err)
	}

	return &result, nil
}

// RollbackDailyQuota 回滚每日上架额度预占
func (c *DailyListingCountAPIClient) RollbackDailyQuota(req *api.RollbackDailyQuotaReqDTO) (int64, error) {
	if c.localDataProvider != nil {
		return c.localDataProvider.RollbackDailyQuota(req)
	}
	url := fmt.Sprintf("%s/rpc-api/listing/store/rollback-daily-quota", c.baseURL)

	result, err := getTypedResult[int64](c.ManagementAPIClient, http.MethodPut, url, req)
	if err != nil {
		return 0, fmt.Errorf("回滚每日上架额度失败: %w", err)
	}

	return result, nil
}

// SetRemainingListingQuota 设置剩余发品额度
func (c *DailyListingCountAPIClient) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	if c.localDataProvider != nil {
		return c.localDataProvider.SetRemainingListingQuota(tenantID, storeID, quota)
	}
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
