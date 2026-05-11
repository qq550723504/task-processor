// Package translate 提供SHEIN平台的翻译处理功能，包括产品标题和描述的多语言翻译
package translate

import (
	"strings"
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/submitprep"
)

// TranslateHandler 翻译处理器
type TranslateHandler struct {
	openaiClient openaiClient.ChatCompleter
}

// NewTranslateHandler 创建新的翻译处理器
func NewTranslateHandler(client openaiClient.ChatCompleter) *TranslateHandler {
	return &TranslateHandler{
		openaiClient: client,
	}
}

// Name 返回处理器名称
func (h *TranslateHandler) Name() string {
	return "翻译产品信息"
}

// Handle 执行翻译处理
func (h *TranslateHandler) Handle(ctx *shein.TaskContext) error {
	features := strings.Join(ctx.AmazonProduct.Features, ", ")
	nameList, descList, err := submitprep.BuildLocalizedTitleAndDescription(
		ctx.Context,
		ctx.Task.Region,
		ctx.AmazonProduct.Title,
		ctx.AmazonProduct.Description,
		features,
		ctx.AmazonProduct.Brand,
		h.openaiClient,
		ctx.AICache,
		ctx.TranslateAPI,
	)
	if err != nil {
		return err
	}
	ctx.ProductData.MultiLanguageNameList = nameList
	ctx.ProductData.MultiLanguageDescList = descList
	return nil
}
