// Package translate 提供SHEIN平台的翻译处理功能，包括产品标题和描述的多语言翻译
package translate

import (
	"strings"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/languageconfig"
	"task-processor/internal/shein/namelimit"
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
	h.loadTargetLanguages(ctx)
	h.loadProductNameLengthLimits(ctx)
	features := strings.Join(ctx.AmazonProduct.Features, ", ")
	nameList, descList, err := submitprep.BuildLocalizedTitleAndDescription(
		ctx.Context,
		append([]string(nil), ctx.TargetLanguages...),
		ctx.AmazonProduct.Title,
		ctx.AmazonProduct.Description,
		features,
		ctx.AmazonProduct.Brand,
		h.openaiClient,
		ctx.AICache,
		ctx.TranslateAPI,
		ctx.ProductNameLengthLimits,
	)
	if err != nil {
		return err
	}
	ctx.ProductData.MultiLanguageNameList = nameList
	ctx.ProductData.MultiLanguageDescList = descList
	return nil
}

func (h *TranslateHandler) loadTargetLanguages(ctx *shein.TaskContext) {
	if len(ctx.TargetLanguages) > 0 {
		return
	}
	if ctx.ProductAPI == nil {
		logger.GetGlobalLogger("shein/translate").Warn("query product language list skipped: product API is unavailable")
		ctx.TargetLanguages = languageconfig.Resolve(nil, ctx.Task.Region)
		return
	}
	items, err := ctx.ProductAPI.QueryLanguageList()
	if err != nil {
		logger.GetGlobalLogger("shein/translate").Warnf("query product language list failed: %v", err)
		ctx.TargetLanguages = languageconfig.Resolve(nil, ctx.Task.Region)
		return
	}
	ctx.TargetLanguages = languageconfig.Resolve(items, ctx.Task.Region)
	if len(languageconfig.Normalize(items)) == 0 {
		logger.GetGlobalLogger("shein/translate").Warn("product language list contains no enabled languages; using region fallback")
	}
}

func (h *TranslateHandler) loadProductNameLengthLimits(ctx *shein.TaskContext) {
	if ctx.ProductNameLengthLimits != nil {
		return
	}
	ctx.ProductNameLengthLimits = make(namelimit.Limits)
	if ctx.ProductAPI == nil || ctx.ProductData == nil || ctx.ProductData.CategoryID <= 0 {
		logger.GetGlobalLogger("shein/translate").Warn("skip product name length config: product API or category ID is unavailable")
		return
	}

	items, err := ctx.ProductAPI.QueryProductNameLengthConfig(ctx.ProductData.CategoryID)
	if err != nil {
		logger.GetGlobalLogger("shein/translate").Warnf("query product name length config for category %d failed: %v", ctx.ProductData.CategoryID, err)
		return
	}
	ctx.ProductNameLengthLimits = namelimit.Normalize(items)
}
