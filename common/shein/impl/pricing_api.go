package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/shein/api"
	"task-processor/common/shein/api/pricing"
)

// PricingAPI 定价相关API实现
type PricingAPI struct {
	*BaseAPIClient
}

// NewPricingAPI 创建新的定价API实现
func NewPricingAPI(baseClient *BaseAPIClient) *PricingAPI {
	return &PricingAPI{
		BaseAPIClient: baseClient,
	}
}

// BatchHandleCostDiscuss 批量处理成本讨论
func (p *PricingAPI) BatchHandleCostDiscuss(reqBody *pricing.BatchHandleCostDiscussRequest) (*pricing.BatchHandleCostDiscussResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), batchHandleCostDiscussEndpoint)

	var result pricing.BatchHandleCostDiscussResponse

	if err := p.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("批量处理成本讨论失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// BargainPage 获取议价页面数据
func (p *PricingAPI) BargainPage(req *pricing.PageRequest, status int) (*pricing.BargainPageResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", p.GetBaseURL(), bargainPageNewEndpoint, req.PageNum, req.PageSize)

	reqBody := map[string]interface{}{
		"bargain_status": status,
	}

	var result pricing.BargainPageResponse

	if err := p.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("获取议价页面失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
