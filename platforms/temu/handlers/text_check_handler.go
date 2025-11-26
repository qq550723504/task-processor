package handlers

import (
	"fmt"
	"task-processor/common/pipeline"

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

// Handle 处理任务
func (h *TextCheckHandler) Handle(ctx *pipeline.TaskContext) error {
	// 实现文本检查逻辑
	logrus.Println("执行文本检查")

	// 示例文本检查请求
	content := ctx.AmazonProduct.Title

	// 发送文本检查请求
	err := h.checkText(ctx, content)
	if err != nil {
		logrus.Errorf("文本检查失败: %v", err)
		return err
	}

	logrus.Println("文本检查完成")
	return nil
}

// checkText 发送文本检查请求到TEMU API
func (h *TextCheckHandler) checkText(ctx *pipeline.TaskContext, content string) error {
	if ctx.GetAPIClient() == nil {
		logrus.Error("API客户端未初始化")
		return fmt.Errorf("API客户端未初始化")
	}

	apiClient := ctx.GetAPIClient()

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
	err := apiClient.SendTEMURequest(apiReq, textCheckResult)
	if err != nil {
		logrus.Errorf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}

	logrus.Infof("文本检查成功: %+v", textCheckResult)
	return nil
}
