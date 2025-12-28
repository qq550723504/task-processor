package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"
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

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *CategoryDisclaimHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *CategoryDisclaimHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	h.logger.Info("开始处理分类免责声明")

	// 获取任务信息
	task := temuCtx.GetTask()
	if task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 从强类型上下文获取TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	temuProduct := temuCtx.TemuProduct

	// 检查分类ID是否存在
	if temuProduct.GoodsBasic.CatID == 0 {
		h.logger.Warn("分类ID为空，跳过免责声明处理")
		return nil
	}

	// 获取分类免责声明
	err := h.getCategoryDisclaimer(temuCtx, temuProduct)
	if err != nil {
		h.logger.Warnf("获取分类免责声明警告: %v", err)
		// 免责声明获取失败不阻止流程继续
	}

	h.logger.Info("分类免责声明处理完成")
	return nil
}

// getCategoryDisclaimer 获取分类免责声明
func (h *CategoryDisclaimHandler) getCategoryDisclaimer(temuCtx *temucontext.TemuTaskContext, temuProduct *types.Product) error {
	catID := temuProduct.GoodsBasic.CatID
	if catID == 0 {
		return fmt.Errorf("分类ID为空")
	}

	h.logger.Infof("获取分类免责声明: CatID=%d", catID)

	// 获取API客户端
	if temuCtx.APIClient == nil {
		h.logger.Warn("API客户端未初始化，使用默认免责声明")
		return nil
	}

	// 调用TEMU API获取分类免责声明
	disclaimer, err := h.queryDisclaimerFromAPI(temuCtx, temuCtx.APIClient, catID)
	if err != nil {
		h.logger.Warnf("API获取免责声明失败，使用默认免责声明: %v", err)
	}

	// 设置免责声明到产品
	temuProduct.GoodsBasic.CategoryDisclaimer = disclaimer

	h.logger.Infof("成功设置分类免责声明: %d 条提示", len(disclaimer.PromptList))
	return nil
}

// queryDisclaimerFromAPI 从TEMU API获取分类免责声明
func (h *CategoryDisclaimHandler) queryDisclaimerFromAPI(temuCtx *temucontext.TemuTaskContext, apiClient any, catID int) (types.Disclaimer, error) {
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
	response := &CategoryDisclaimResponse{}

	// 类型断言获取TEMU API客户端
	type TEMUAPIClient interface {
		SendTEMURequest(request map[string]any, response any) error
	}

	if temuClient, ok := apiClient.(TEMUAPIClient); ok {
		err := temuClient.SendTEMURequest(apiReq, response)
		if err != nil {
			h.logger.Errorf("发送API请求失败: %v", err)
			return types.Disclaimer{}, fmt.Errorf("发送API请求失败: %w", err)
		}
	} else {
		return types.Disclaimer{}, fmt.Errorf("API客户端不支持TEMU请求")
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
