package pricing

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/api/pricing"
	"task-processor/internal/platforms/shein/client"
)

// PricingAPI 定价相关API实现
type PricingAPI struct {
	*client.BaseAPIClient
}

// NewPricingAPI 创建新的定价API实现
func NewPricingAPI(baseClient *client.BaseAPIClient) *PricingAPI {
	return &PricingAPI{
		BaseAPIClient: baseClient,
	}
}

// BatchHandleCostDiscuss 批量处理成本讨论
func (p *PricingAPI) BatchHandleCostDiscuss(reqBody *pricing.BatchHandleCostDiscussRequest) (*pricing.BatchHandleCostDiscussResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), client.GetBatchHandleCostDiscussEndpoint())

	var result pricing.BatchHandleCostDiscussResponse

	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
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
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", p.GetBaseURL(), client.GetBargainPageNewEndpoint(), req.PageNum, req.PageSize)

	reqBody := map[string]any{
		"bargain_status": status,
	}

	// 添加时间参数（如果提供）
	if req.StartTime != "" {
		reqBody["start_time"] = req.StartTime
	}
	if req.EndTime != "" {
		reqBody["end_time"] = req.EndTime
	}

	var result pricing.BargainPageResponse

	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
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

// BatchReQuote 批量重新核价
func (p *PricingAPI) BatchReQuote(reqBody *pricing.BatchReQuoteRequest) (*pricing.BatchReQuoteResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), client.GetBatchReQuoteEndpoint())

	var result pricing.BatchReQuoteResponse

	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	// 统一错误处理
	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("批量重新核价失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
