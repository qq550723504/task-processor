// Package query 提供TEMU平台查询相关的API
package query

import (
	"fmt"
	"strings"

	"task-processor/internal/temu/api/client"
	temutemplate "task-processor/internal/temu/api/template"

	"github.com/sirupsen/logrus"
)

// API 查询API管理器
type API struct {
	client client.ClientAPI
	logger *logrus.Entry
}

// NewAPI 创建查询API管理器
func NewAPI(c client.ClientAPI, logger *logrus.Entry) *API {
	return &API{client: c, logger: logger}
}

func (a *API) defaultHeaders() map[string]string {
	return map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"x-document-referer": "https://seller.temu.com/product-add.html?is_back=1",
	}
}

// CheckText 检查文本内容
func (a *API) CheckText(request *TextCheckRequest) (*TextCheckResponse, error) {
	if request == nil || request.Content == "" {
		return nil, fmt.Errorf("检查文本不能为空")
	}

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/query/commit/check_text",
		"headers": a.defaultHeaders(),
		"body":    request,
	}

	var result TextCheckResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("文本检查请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("文本检查失败: errorCode=%d", result.ErrorCode)
	}
	if !result.Result.Success {
		return nil, fmt.Errorf("文本检查未通过")
	}
	return &result, nil
}

// QueryTemplate 查询模板信息
func (a *API) QueryTemplate(request *TemplateQueryRequest) (*TemplateQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_template",
		"body":   request,
	}

	var result TemplateQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryTemplateAdvanced 查询模板信息（支持完整 types 结构）
func (a *API) QueryTemplateAdvanced(request *temutemplate.TemplateQueryRequest) (*temutemplate.TemplateQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method":  "POST",
		"url":     "/mms/marigold/query/commit/query_template",
		"headers": a.defaultHeaders(),
		"body":    request,
	}

	var result temutemplate.TemplateQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QuerySpec 查询规格信息
func (a *API) QuerySpec(request *SpecQueryRequest) (*SpecQueryResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/edit/commit/spec_query",
		"body":   request,
	}

	var result SpecQueryResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("规格查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("规格查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// CheckSkuSn 检查SKU编码
func (a *API) CheckSkuSn(request *SkuSnCheckRequest) (*SkuSnCheckResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("检查请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/out_sku_sn_batch_check",
		"body":   request,
	}

	var result SkuSnCheckResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("SKU编码检查请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("SKU编码检查失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryCostTemplate 查询成本模板
func (a *API) QueryCostTemplate(request *CostTemplateRequest) (*CostTemplateResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_cost_template",
		"body":   request,
	}

	var result CostTemplateResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("成本模板查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("成本模板查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QueryCommitDetail 查询提交详情
func (a *API) QueryCommitDetail(request *CommitDetailRequest) (*CommitDetailResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_commit_detail",
		"body":   request,
	}

	var result CommitDetailResponse
	if err := a.client.SendTEMURequest(req, &result); err != nil {
		return nil, fmt.Errorf("提交详情查询请求失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("提交详情查询失败: errorCode=%d", result.ErrorCode)
	}
	return &result, nil
}

// QuerySkuPriceAndStock 查询SKU价格与库存
func (a *API) QuerySkuPriceAndStock(commitID, goodsID string) (*SkuQueryResponse, error) {
	if commitID == "" || goodsID == "" {
		return nil, fmt.Errorf("commitID 和 goodsID 不能为空")
	}

	req := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/sku/query",
		"headers": map[string]string{
			"x-document-referer": "https://seller.temu.com/products.html",
		},
		"body": &SkuQueryRequest{
			CommitID:             commitID,
			GoodsID:              goodsID,
			SourceTypeOfSkuQuery: 1,
			Source:               0,
		},
	}

	var result SkuQueryResponse
	authManager := client.NewAuthManager(a.logger)
	if err := authManager.SendRequestWithAuth(a.client, req, &result); err != nil {
		return nil, fmt.Errorf("调用SKU查询API失败: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API返回错误: error_code=%d", result.ErrorCode)
	}
	return &result, nil
}

// QuerySkuPriceAndStockWithOptions 使用选项查询SKU价格与库存
func (a *API) QuerySkuPriceAndStockWithOptions(options SkuQueryOptions) (*SkuQueryResponse, error) {
	return a.QuerySkuPriceAndStock(options.CommitID, options.GoodsID)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "Client.Timeout exceeded")
}
