package pricing

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// Client 定价相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的定价API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

// BatchHandleCostDiscuss 批量处理成本讨论
func (p *Client) BatchHandleCostDiscuss(reqBody *BatchHandleCostDiscussRequest) (*BatchHandleCostDiscussResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), client.GetBatchHandleCostDiscussEndpoint())

	var result BatchHandleCostDiscussResponse
	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("批量处理成本讨论失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// BargainPage 获取议价页面数据
func (p *Client) BargainPage(req *PageRequest, status int) (*BargainPageResponse, error) {
	url := fmt.Sprintf("%s%s?page_num=%d&page_size=%d", p.GetBaseURL(), client.GetBargainPageNewEndpoint(), req.PageNum, req.PageSize)

	reqBody := map[string]any{"bargain_status": status}
	if req.StartTime != "" {
		reqBody["start_time"] = req.StartTime
	}
	if req.EndTime != "" {
		reqBody["end_time"] = req.EndTime
	}

	var result BargainPageResponse
	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取议价页面失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}

// BatchReQuote 批量重新核价
func (p *Client) BatchReQuote(reqBody *BatchReQuoteRequest) (*BatchReQuoteResponse, error) {
	url := fmt.Sprintf("%s%s", p.GetBaseURL(), client.GetBatchReQuoteEndpoint())

	var result BatchReQuoteResponse
	if err := p.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("批量重新核价失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
