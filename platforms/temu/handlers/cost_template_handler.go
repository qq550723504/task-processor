package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

	"github.com/sirupsen/logrus"
)

// CostTemplateHandler 成本模板处理器
type CostTemplateHandler struct {
	logger *logrus.Entry
}

// CostTemplateRequest 成本模板查询请求结构体
type CostTemplateRequest struct {
	ListingCommitID      string `json:"listing_commit_id"`
	GoodsCommitID        string `json:"goods_commit_id"`
	GoodsID              string `json:"goods_id"`
	CatID                int    `json:"cat_id"`
	ListingCommitVersion string `json:"listing_commit_version"`
	ClickType            string `json:"click_type"`
	QueryAll             bool   `json:"query_all"`
}

// CostTemplateResponse 成本模板查询响应结构体
type CostTemplateResponse struct {
	Success   bool                `json:"success"`
	ErrorCode int                 `json:"error_code"`
	Result    *CostTemplateResult `json:"result,omitempty"`
	Message   string              `json:"message,omitempty"`
}

// CostTemplateResult 成本模板结果数据
type CostTemplateResult struct {
	CostTemplateList []CostTemplateItem `json:"cost_template_list"`
	CostTemplateURL  string             `json:"cost_template_url"`
}

// CostTemplateItem 成本模板项
type CostTemplateItem struct {
	CostTemplateID  string `json:"cost_template_id"`
	TemplateName    string `json:"template_name"`
	Disabled        bool   `json:"disabled"`
	DefaultTemplate bool   `json:"default_template"`
}

func NewCostTemplateHandler() *CostTemplateHandler {
	return &CostTemplateHandler{
		logger: logrus.WithField("handler", "CostTemplateHandler"),
	}
}

func (h *CostTemplateHandler) Name() string {
	return "成本模板处理器"
}

func (h *CostTemplateHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始查询成本模板")

	// 检查API客户端
	if ctx.APIClient == nil {
		h.logger.Warn("API客户端未初始化，使用默认成本模板")
		if ctx.TemuProduct != nil {
			ctx.TemuProduct.GoodsServicePromise.CostTemplateID = "default_template_001"
		}
		return nil
	}

	// 构造成本模板查询请求
	request := h.buildCostTemplateRequest(ctx)

	// 发送成本模板查询请求
	err := h.queryCostTemplate(ctx, request)
	if err != nil {
		h.logger.WithError(err).Warn("查询成本模板失败，使用默认模板")
		// 设置默认成本模板
		if ctx.TemuProduct != nil {
			ctx.TemuProduct.GoodsServicePromise.CostTemplateID = "default_template_001"
		}
		// 继续执行，不返回错误
	}

	h.logger.Info("成本模板处理完成")
	return nil
}

// buildCostTemplateRequest 构造成本模板查询请求
func (h *CostTemplateHandler) buildCostTemplateRequest(ctx *pipeline.TaskContext) *CostTemplateRequest {
	request := &CostTemplateRequest{
		ClickType:            "8",  // 根据实际API调用设置
		ListingCommitVersion: "1",  // 默认版本
		QueryAll:             true, // 查询所有模板
	}

	// 从上下文中获取实际的参数值
	if ctx.TemuProduct != nil {
		if ctx.TemuProduct.GoodsBasic.ListingCommitID != "" {
			request.ListingCommitID = ctx.TemuProduct.GoodsBasic.ListingCommitID
		}
		if ctx.TemuProduct.GoodsBasic.GoodsCommitID != "" {
			request.GoodsCommitID = ctx.TemuProduct.GoodsBasic.GoodsCommitID
		}
		if ctx.TemuProduct.GoodsBasic.GoodsID != "" {
			request.GoodsID = ctx.TemuProduct.GoodsBasic.GoodsID
		}
		if ctx.TemuProduct.GoodsBasic.CatID > 0 {
			request.CatID = ctx.TemuProduct.GoodsBasic.CatID
		}
	}

	return request
}

// queryCostTemplate 发送成本模板查询请求到TEMU API
func (h *CostTemplateHandler) queryCostTemplate(ctx *pipeline.TaskContext, request *CostTemplateRequest) error {
	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/query_cost_template",
		"headers": map[string]string{
			"accept":             "application/json, text/plain, */*",
			"accept-language":    "zh-CN,zh;q=0.9",
			"content-type":       "application/json;charset=UTF-8",
			"priority":           "u=1, i",
			"sec-ch-ua":          "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
			"sec-ch-ua-mobile":   "?0",
			"sec-ch-ua-platform": "\"Windows\"",
			"sec-fetch-dest":     "empty",
			"sec-fetch-mode":     "cors",
			"sec-fetch-site":     "same-origin",
		},
		"body": request,
	}

	// 发送请求
	response := &CostTemplateResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}

	templateCount := 0
	if response.Result != nil {
		templateCount = len(response.Result.CostTemplateList)
	}

	h.logger.WithFields(logrus.Fields{
		"listingCommitID": request.ListingCommitID,
		"goodsCommitID":   request.GoodsCommitID,
		"catID":           request.CatID,
		"success":         response.Success,
		"errorCode":       response.ErrorCode,
		"templateCount":   templateCount,
	}).Info("成本模板查询响应")

	// 检查响应是否成功
	if !response.Success {
		return fmt.Errorf("成本模板查询失败: error_code=%d", response.ErrorCode)
	}

	// 从响应中解析成本模板ID
	if ctx.TemuProduct != nil {
		costTemplateID := h.extractCostTemplateID(response)
		if costTemplateID != "" {
			ctx.TemuProduct.GoodsServicePromise.CostTemplateID = costTemplateID
			h.logger.Infof("设置成本模板ID: %s", costTemplateID)
		} else {
			// 如果无法解析，使用默认模板
			ctx.TemuProduct.GoodsServicePromise.CostTemplateID = "default_template_001"
			h.logger.Warn("无法解析成本模板ID，使用默认模板")
		}
	}

	return nil
}

// extractCostTemplateID 从响应中提取成本模板ID
func (h *CostTemplateHandler) extractCostTemplateID(response *CostTemplateResponse) string {
	if response == nil || response.Result == nil {
		h.logger.Warn("响应数据为空")
		return ""
	}

	if len(response.Result.CostTemplateList) == 0 {
		h.logger.Warn("成本模板列表为空")
		return ""
	}

	// 优先选择默认模板
	for _, template := range response.Result.CostTemplateList {
		if template.DefaultTemplate && !template.Disabled {
			h.logger.Infof("选择默认成本模板: %s (%s)", template.TemplateName, template.CostTemplateID)
			return template.CostTemplateID
		}
	}

	// 如果没有默认模板，选择第一个可用的模板
	for _, template := range response.Result.CostTemplateList {
		if !template.Disabled {
			h.logger.Infof("选择第一个可用成本模板: %s (%s)", template.TemplateName, template.CostTemplateID)
			return template.CostTemplateID
		}
	}

	// 如果所有模板都被禁用，选择第一个模板
	if len(response.Result.CostTemplateList) > 0 {
		template := response.Result.CostTemplateList[0]
		h.logger.Warnf("所有模板都被禁用，强制选择第一个: %s (%s)", template.TemplateName, template.CostTemplateID)
		return template.CostTemplateID
	}

	h.logger.Warn("响应中未找到任何成本模板")
	return ""
}
