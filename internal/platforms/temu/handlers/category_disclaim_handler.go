package handlers

import (
	"fmt"
	"task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// CategoryDisclaimHandler 分类免责声明处理器
type CategoryDisclaimHandler struct {
	logger *logrus.Entry
}

// NewCategoryDisclaimHandler 创建新的分类免责声明处理器
func NewCategoryDisclaimHandler() *CategoryDisclaimHandler {
	return &CategoryDisclaimHandler{
		logger: logrus.WithField("handler", "CategoryDisclaimHandler"),
	}
}

// Name 返回处理器名称
func (h *CategoryDisclaimHandler) Name() string {
	return "分类免责声明处理器"
}

// Handle 处理任务
func (h *CategoryDisclaimHandler) Handle(ctx *pipeline.TaskContext) error {
	h.logger.Info("开始处理分类免责声明")

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 检查分类ID是否存在
	if ctx.TemuProduct.GoodsBasic.CatID == 0 {
		h.logger.Warn("分类ID为空，跳过免责声明处理")
		return nil
	}

	// 获取分类免责声明
	err := h.getCategoryDisclaimer(ctx)
	if err != nil {
		h.logger.Warnf("获取分类免责声明警告: %v", err)
		// 免责声明获取失败不阻止流程继续
	}

	h.logger.Info("分类免责声明处理完成")
	return nil
}

// getCategoryDisclaimer 获取分类免责声明
func (h *CategoryDisclaimHandler) getCategoryDisclaimer(ctx *pipeline.TaskContext) error {
	catID := ctx.TemuProduct.GoodsBasic.CatID
	if catID == 0 {
		return fmt.Errorf("分类ID为空")
	}

	h.logger.Infof("获取分类免责声明: CatID=%d", catID)

	// 检查API客户端
	if ctx.APIClient == nil {
		h.logger.Warn("API客户端未初始化，使用默认免责声明")
		return nil
	}

	// 调用TEMU API获取分类免责声明
	disclaimer, err := h.queryDisclaimerFromAPI(ctx, catID)
	if err != nil {
		h.logger.Warnf("API获取免责声明失败，使用默认免责声明: %v", err)
	}

	// 设置免责声明到产品
	ctx.TemuProduct.GoodsBasic.CategoryDisclaimer = disclaimer

	h.logger.Infof("成功设置分类免责声明: %d 条提示", len(disclaimer.PromptList))
	return nil
}

// queryDisclaimerFromAPI 从TEMU API获取分类免责声明
func (h *CategoryDisclaimHandler) queryDisclaimerFromAPI(ctx *pipeline.TaskContext, catID int) (types.Disclaimer, error) {
	h.logger.Infof("调用TEMU API获取分类免责声明: CatID=%d", catID)

	// 构造请求体
	requestBody := map[string]any{
		"cate_id": catID,
	}

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/category/query_disclaim",
		"headers": map[string]string{
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
		},
		"body": requestBody,
	}

	// 定义响应结构
	type CategoryDisclaimResponse struct {
		Success   bool `json:"success"`
		ErrorCode int  `json:"error_code"`
		Result    struct {
			DisclaimerDTO struct {
				PromptList []string `json:"prompt_list"`
			} `json:"disclaimer_dto"`
		} `json:"result"`
	}

	response := &CategoryDisclaimResponse{}
	err := ctx.APIClient.SendTEMURequest(apiReq, response)
	if err != nil {
		h.logger.Errorf("发送API请求失败: %v", err)
		return types.Disclaimer{}, fmt.Errorf("发送API请求失败: %w", err)
	}

	h.logger.Debugf("API响应: Success=%t, ErrorCode=%d", response.Success, response.ErrorCode)

	// 检查响应状态
	if !response.Success {
		h.logger.Errorf("API返回失败，错误码: %d", response.ErrorCode)
		return types.Disclaimer{}, fmt.Errorf("API返回失败，错误码: %d", response.ErrorCode)
	}

	// 转换响应数据
	disclaimer := types.Disclaimer{
		PromptList: response.Result.DisclaimerDTO.PromptList,
	}

	h.logger.Infof("成功从API获取分类免责声明: %d 条提示", len(disclaimer.PromptList))
	return disclaimer, nil
}
