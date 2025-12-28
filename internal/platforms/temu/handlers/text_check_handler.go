package handlers

import (
	"fmt"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// TextCheckHandler 文本检查处理器
type TextCheckHandler struct{}

// TextCheckRequest 文本检查请求结构体
type TextCheckRequest struct {
	Content string `json:"content"`
	Type    int    `json:"type"`
}

// TextCheckResult 文本检查结果结构体
type TextCheckResult struct {
	Success bool `json:"success"`
}

// NewTextCheckHandler 创建新的文本检查处理器
func NewTextCheckHandler() *TextCheckHandler {
	return &TextCheckHandler{}
}

// Name 返回处理器名称
func (h *TextCheckHandler) Name() string {
	return "文本检查"
}

// Handle 处理任务（兼容pipeline.Handler接口）
func (h *TextCheckHandler) Handle(ctx pipeline.TaskContext) error {
	// 类型断言为强类型上下文
	temuCtx, ok := ctx.(*temucontext.TemuTaskContext)
	if !ok {
		return fmt.Errorf("上下文类型错误，期望TemuTaskContext")
	}
	return h.HandleTemu(temuCtx)
}

// HandleTemu 处理任务（强类型上下文）
func (h *TextCheckHandler) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 实现文本检查逻辑
	logrus.Println("执行文本检查")

	// 获取Amazon产品数据
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		logrus.Warn("Amazon产品数据为空，跳过文本检查")
		return nil
	}

	// 示例文本检查请求
	content := amazonProduct.Title

	// 发送文本检查请求
	err := h.checkText(temuCtx, content)
	if err != nil {
		logrus.Errorf("文本检查失败: %v", err)
		return err
	}

	// 文本检查通过后，将标题赋值给TEMU产品
	err = h.assignTitleToTemuProduct(temuCtx, content)
	if err != nil {
		logrus.Errorf("赋值标题到TEMU产品失败: %v", err)
		return err
	}

	logrus.Println("文本检查完成")
	return nil
}

// checkText 发送文本检查请求到TEMU API
func (h *TextCheckHandler) checkText(temuCtx *temucontext.TemuTaskContext, content string) error {
	// 检查API客户端
	if temuCtx.APIClient == nil {
		logrus.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	// 构造请求体
	requestBody := TextCheckRequest{
		Content: content,
		Type:    1,
	}

	// 构造API请求
	apiReq := map[string]any{
		"method": "POST",
		"url":    "/mms/marigold/query/commit/check_text",
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

	textCheckResult := &TextCheckResult{}

	// 进行类型断言获取TEMU API客户端
	type TEMUAPIClient interface {
		SendTEMURequest(request map[string]any, response any) error
	}

	// 使用类型断言检查API客户端是否支持TEMU请求
	if temuClient, ok := interface{}(temuCtx.APIClient).(TEMUAPIClient); ok {
		err := temuClient.SendTEMURequest(apiReq, textCheckResult)
		if err != nil {
			logrus.Errorf("发送请求失败: %v", err)
			return fmt.Errorf("发送请求失败: %v", err)
		}
	} else {
		return fmt.Errorf("API客户端不支持TEMU请求")
	}

	logrus.Infof("文本检查成功: %+v", textCheckResult)
	return nil
}

// assignTitleToTemuProduct 将检查通过的标题赋值给TEMU产品
func (h *TextCheckHandler) assignTitleToTemuProduct(temuCtx *temucontext.TemuTaskContext, checkedTitle string) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 设置商品名称到GoodsBasic中
	temuCtx.TemuProduct.GoodsBasic.GoodsName = checkedTitle

	logrus.Infof("已将检查通过的标题设置到TEMU产品: %s", checkedTitle)
	return nil
}
