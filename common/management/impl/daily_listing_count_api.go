package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
)

// DailyListingCountAPIClientImpl 每日上架数量API客户端实现
type DailyListingCountAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// NewDailyListingCountAPIClient 创建每日上架数量API客户端
func NewDailyListingCountAPIClient(baseURL string) *DailyListingCountAPIClientImpl {
	return &DailyListingCountAPIClientImpl{
		ManagementAPIClientImpl: NewManagementAPIClientWithBaseURL(baseURL),
	}
}

// GetDailyListingCount 获取每日上架数量
func (c *DailyListingCountAPIClientImpl) GetDailyListingCount(tenantID, storeID, userID int64, date string) (*api.DailyListingCountRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/store/get-daily-listing-count", c.baseURL)

	// 构建查询参数
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

	// API返回的data字段是Integer类型，需要转换为int64
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

	// 构建响应DTO
	return &api.DailyListingCountRespDTO{
		TenantID: tenantID,
		StoreID:  storeID,
		UserID:   userID,
		Date:     date,
		Count:    count,
	}, nil
}

// SetDailyListingCount 设置每日上架数量
func (c *DailyListingCountAPIClientImpl) SetDailyListingCount(req *api.DailyListingCountSetReqDTO) error {
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-daily-listing-count", c.baseURL)

	var result APIResponse
	result.Data = 0
	if err := c.apiRequest(http.MethodPut, url, req, &result); err != nil {
		return fmt.Errorf("设置每日上架数量失败: %w", err)
	}

	return nil
}

// SetRemainingListingQuota 设置剩余发品额度
func (c *DailyListingCountAPIClientImpl) SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error) {
	// 使用URL参数，因为Spring Boot接口使用@RequestParam，使用PUT方法
	url := fmt.Sprintf("%s/rpc-api/listing/store/set-remaining-listing-quota?tenantId=%d&storeId=%d&quota=%d",
		c.baseURL, tenantID, storeID, quota)

	var result APIResponse
	result.Data = false
	// PUT请求，参数在URL中，不传body
	if err := c.apiRequest(http.MethodPut, url, nil, &result); err != nil {
		return false, fmt.Errorf("设置剩余发品额度失败: %w", err)
	}

	// 解析返回的布尔值
	success, ok := result.Data.(bool)
	if !ok {
		return false, fmt.Errorf("无法解析响应数据类型: %T, 值: %v", result.Data, result.Data)
	}

	return success, nil
}
