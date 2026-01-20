package services

import (
	"fmt"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/types"

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
	q.logger.WithField("Content", request.Content).Info("开始检查文本内容")

	if request == nil {
		return nil, fmt.Errorf("检查请求不能为空")
	}

	if request.Content == "" {
		return nil, fmt.Errorf("检查文本不能为空")
	}

	// 构建请求头，使用与之前工作版本完全相同的配置
	headers := map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"priority":           "u=1, i",
		"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "\"Windows\"",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
		"x-document-referer": "https://seller.temu.com/product-add.html?is_back=1",
	}

	apiReq := map[string]interface{}{
		"method":  "POST",
		"url":     textCheckEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result models.TextCheckResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).WithFields(map[string]interface{}{
			"endpoint": textCheckEndpoint,
			"content":  request.Content,
		}).Error("文本检查请求失败")
		return nil, fmt.Errorf("文本检查请求失败: %w", err)
	}

	// 记录详细的响应信息
	q.logger.WithFields(map[string]interface{}{
		"success":       result.Success,
		"errorCode":     result.ErrorCode,
		"resultSuccess": result.Result.Success,
	}).Info("收到文本检查响应")

	if !result.Success {
		q.logger.WithFields(map[string]interface{}{
			"errorCode": result.ErrorCode,
			"content":   request.Content,
		}).Error("文本检查API返回失败状态")
		return nil, fmt.Errorf("文本检查失败: errorCode=%d", result.ErrorCode)
	}

	// 检查结果中的 success 字段
	if !result.Result.Success {
		q.logger.WithFields(map[string]interface{}{
			"content": request.Content,
		}).Warn("文本检查未通过")
		return nil, fmt.Errorf("文本检查未通过")
	}

	q.logger.WithFields(map[string]interface{}{
		"resultSuccess": result.Result.Success,
	}).Info("文本检查完成")

	return &result, nil
}

// QueryTemplate 查询模板信息
func (q *QueryAPI) QueryTemplate(request *models.TemplateQueryRequest) (*models.TemplateQueryResponse, error) {
	q.logger.Info("开始查询模板信息")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    templateQueryEndpoint,
		"body":   request,
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

	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    specQueryEndpoint,
		"body":   request,
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

	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    skuSnCheckEndpoint,
		"body":   request,
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

	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    costTemplateEndpoint,
		"body":   request,
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

	apiReq := map[string]interface{}{
		"method": "POST",
		"url":    commitDetailEndpoint,
		"body":   request,
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

// QueryTemplateAdvanced 查询模板信息（高级版本，支持完整的types结构）
func (q *QueryAPI) QueryTemplateAdvanced(request *types.TemplateQueryRequest) (*types.TemplateQueryResponse, error) {
	q.logger.Info("开始查询模板信息（高级版本）")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	// 构建请求头，使用与之前工作版本完全相同的配置
	headers := map[string]string{
		"accept":             "application/json, text/plain, */*",
		"accept-language":    "zh-CN,zh;q=0.9",
		"priority":           "u=1, i",
		"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": "\"Windows\"",
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-origin",
	}

	apiReq := map[string]any{
		"method":  "POST",
		"url":     templateQueryEndpoint,
		"headers": headers,
		"body":    request,
	}

	var result types.TemplateQueryResponse
	if err := q.client.SendTEMURequest(apiReq, &result); err != nil {
		q.logger.WithError(err).Error("模板查询请求失败")
		return nil, fmt.Errorf("模板查询请求失败: %w", err)
	}

	if !result.Success {
		q.logger.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
		return nil, fmt.Errorf("模板查询失败: errorCode=%d", result.ErrorCode)
	}

	q.logger.WithFields(map[string]any{
		"templateID":          result.Result.TemplateInfo.TemplateID,
		"specPropertiesCount": len(result.Result.TemplateInfo.GoodsSpecProperties),
		"success":             result.Success,
	}).Info("模板查询完成（高级版本）")

	return &result, nil
}

// QueryMaxRetailPrice 查询最大零售价格
func (q *QueryAPI) QueryMaxRetailPrice(request *models.PriceQueryRequest) (*models.PriceQueryResponse, error) {
	q.logger.Info("开始查询最大零售价格")

	if request == nil {
		return nil, fmt.Errorf("查询请求不能为空")
	}

	apiReq := map[string]any{
		"method": "POST",
		"url":    priceQueryEndpoint,
		"body":   request,
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
