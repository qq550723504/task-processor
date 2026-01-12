package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// 文本检查接口
	textCheckEndpoint = "/mms/marigold/query/commit/check_text"
	// 模板查询接口
	templateQueryEndpoint = "/mms/marigold/query/commit/query_template"
	// 规格查询接口
	specQueryEndpoint = "/mms/marigold/edit/commit/spec_query"
	// SKU编码检查接口
	skuSnCheckEndpoint = "/mms/marigold/query/commit/out_sku_sn_batch_check"
	// 成本模板查询接口
	costTemplateEndpoint = "/mms/marigold/query/commit/query_cost_template"
	// 提交详情查询接口
	commitDetailEndpoint = "/mms/marigold/query/commit/query_commit_detail"
	// 价格查询接口
	priceQueryEndpoint = "/mms/marigold/price/retail/max/info"
)

// QueryAPI 查询API管理器
type QueryAPI struct {
	client client.APIClientInterface
	logger *logrus.Entry
}

// NewQueryAPI 创建新的查询API管理器
func NewQueryAPI(client client.APIClientInterface, logger *logrus.Entry) *QueryAPI {
	return &QueryAPI{
		client: client,
		logger: logger,
	}
}

// CheckText 检查文本内容
func (q *QueryAPI) CheckText(request *models.TextCheckRequest) (*models.TextCheckResponse, error) {
	q.logger.Info("开始检查文本内容")

	if request == nil {
		return nil, fmt.Errorf("检查请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     textCheckEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.TextCheckResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("文本检查请求失败")
		return nil, fmt.Errorf("文本检查请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("文本检查失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("文本检查失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("文本检查完成")
	return &result, nil
}

// QueryTemplate 查询模板信息
func (q *QueryAPI) QueryTemplate(request *models.TemplateQueryRequest) (*models.TemplateQueryResponse, error) {
	q.logger.Info("开始查询模板信息")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     templateQueryEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.TemplateQueryResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("模板查询请求失败")
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("模板查询完成")
	return &result, nil
}

// QuerySpec 查询规格信息
func (q *QueryAPI) QuerySpec(request *models.SpecQueryRequest) (*models.SpecQueryResponse, error) {
	q.logger.Info("开始查询规格信息")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     specQueryEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.SpecQueryResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("规格查询请求失败")
		return nil, fmt.Errorf("规格查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("规格查询失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("规格查询失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("规格查询完成")
	return &result, nil
}

// CheckSkuSn 检查SKU编码
func (q *QueryAPI) CheckSkuSn(request *models.SkuSnCheckRequest) (*models.SkuSnCheckResponse, error) {
	q.logger.Info("开始检查SKU编码")

	if request == nil {
		return nil, fmt.Errorf("检查请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     skuSnCheckEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.SkuSnCheckResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("SKU编码检查请求失败")
		return nil, fmt.Errorf("SKU编码检查请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("SKU编码检查失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("SKU编码检查失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("SKU编码检查完成")
	return &result, nil
}

// QueryCostTemplate 查询成本模板
func (q *QueryAPI) QueryCostTemplate(request *models.CostTemplateRequest) (*models.CostTemplateResponse, error) {
	q.logger.Info("开始查询成本模板")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     costTemplateEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.CostTemplateResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("成本模板查询请求失败")
		return nil, fmt.Errorf("成本模板查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("成本模板查询失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("成本模板查询失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("成本模板查询完成")
	return &result, nil
}

// QueryCommitDetail 查询提交详情
func (q *QueryAPI) QueryCommitDetail(request *models.CommitDetailRequest) (*models.CommitDetailResponse, error) {
	q.logger.Info("开始查询提交详情")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Microsoft Edge\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     commitDetailEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.CommitDetailResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("提交详情查询请求失败")
		return nil, fmt.Errorf("提交详情查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("提交详情查询失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("提交详情查询失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.Info("提交详情查询完成")
	return &result, nil
}

// QueryMaxRetailPrice 查询最大零售价格
func (q *QueryAPI) QueryMaxRetailPrice(request *models.PriceQueryRequest) (*models.PriceQueryResponse, error) {
	q.logger.Info("开始查询最大零售价格")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	headers := client.GetDefaultHeaders()
	headers["accept"] = "application/json, text/plain, */*"
	headers["accept-language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["content-type"] = "application/json;charset=UTF-8"
	headers["priority"] = "u=1, i"
	headers["sec-ch-ua"] = "\"Chromium\";v=\"142\", \"Microsoft Edge\";v=\"142\", \"Not_A Brand\";v=\"99\""
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = "\"Windows\""
	headers["sec-fetch-dest"] = "empty"
	headers["sec-fetch-mode"] = "cors"
	headers["sec-fetch-site"] = "same-origin"

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     priceQueryEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.PriceQueryResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("价格查询请求失败")
		return nil, fmt.Errorf("价格查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("价格查询失败: errorCode=%d, message=%s", result.ErrorCode, result.ErrorMsg)
		return nil, fmt.Errorf("价格查询失败: errorCode=%d, message=%s", result.ErrorCode, result.ErrorMsg)
	}

	q.logger.Info("价格查询完成")
	return &result, nil
}
