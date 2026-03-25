// Package skc 提供SHEIN平台SKC翻译处理功能
package skc

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/prompt"
	"task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/translate"
)

// SKCTranslationHandler SKC翻译处理器
type SKCTranslationHandler struct {
	runtime      *SKCRuntimeInput
	openaiClient openaiClient.ChatCompleter
}

// NewSKCTranslationHandler 创建新的SKC翻译处理器
func NewSKCTranslationHandler(runtime *SKCRuntimeInput, openaiClient openaiClient.ChatCompleter) *SKCTranslationHandler {
	return &SKCTranslationHandler{
		runtime:      runtime,
		openaiClient: openaiClient,
	}
}

// CreateSKC 创建SKC的工厂函数
func (h *SKCTranslationHandler) CreateSKC(ctx *shein.TaskContext, params shein.SKCCreationParams) product.SKC {
	targetLanguages := translate.GetTargetLanguagesByRegion(h.runtime.Region)
	sourceTitle := h.findBestSourceTitle(params)
	sourceLang := h.detectTitleLanguage(sourceTitle)
	multiLanguageNameList := h.initializeMultiLanguageContent(targetLanguages)

	h.translateToAllLanguages(ctx.Context, sourceTitle, sourceLang, &multiLanguageNameList)
	h.optimizeMultiLanguageContent(ctx.Context, &multiLanguageNameList, sourceTitle)

	primaryLanguageContent := h.selectPrimaryDisplayLanguage(targetLanguages, multiLanguageNameList, sourceTitle)

	return product.SKC{
		SaleAttribute: product.SaleAttribute{
			AttributeID:        params.AttributeID,
			AttributeValueID:   params.AttributeValueID,
			IsSPPSaleAttribute: false,
			PreFillSpec:        false,
		},
		SKUS:                    params.SKUS,
		ImageInfo:               params.ImageInfo,
		SiteDetailImageInfoList: []product.SiteDetailImageInfo{},
		ShelfWay:                1,
		ShelfRequire:            0,
		MultiLanguageName:       primaryLanguageContent,
		MultiLanguageNameList:   multiLanguageNameList,
		Sort:                    params.Sort,
	}
}

func (h *SKCTranslationHandler) findBestSourceTitle(params shein.SKCCreationParams) string {
	logger.GetGlobalLogger("shein/product").Debugf("开始查找源标题")

	if len(h.runtime.Variants) > 0 && len(params.SKUS) > 0 {
		if params.SKUS[0].SupplierSKU != "" {
			supplierSKU := params.SKUS[0].SupplierSKU
			var targetASIN string
			for asin, sku := range h.runtime.AsinSkuMap {
				if sku == supplierSKU {
					targetASIN = asin
					break
				}
			}
			if targetASIN != "" {
				for _, variant := range h.runtime.Variants {
					if variant.Asin == targetASIN && variant.Title != "" {
						return variant.Title
					}
				}
			}
		}

		for _, variant := range h.runtime.Variants {
			if variant.Title != "" {
				return variant.Title
			}
		}
	}

	if h.runtime.AmazonProduct != nil && h.runtime.AmazonProduct.Title != "" {
		return h.runtime.AmazonProduct.Title
	}
	return ""
}

func (h *SKCTranslationHandler) detectTitleLanguage(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "en"
	}

	var japaneseCount, chineseCount, englishCount int
	for _, r := range title {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF):
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF:
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	if japaneseCount > chineseCount && japaneseCount > englishCount {
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		return "zh"
	}
	return "en"
}

func (h *SKCTranslationHandler) initializeMultiLanguageContent(targetLanguages []string) []product.LanguageContent {
	multiLanguageNameList := make([]product.LanguageContent, 0, len(targetLanguages))
	for _, lang := range targetLanguages {
		multiLanguageNameList = append(multiLanguageNameList, product.LanguageContent{Language: lang, Name: ""})
	}
	return multiLanguageNameList
}

func (h *SKCTranslationHandler) translateToAllLanguages(ctx context.Context, sourceTitle string, sourceLang string, multiLanguageNameList *[]product.LanguageContent) {
	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]
		if langContent.Language == sourceLang {
			langContent.Name = sourceTitle
			continue
		}
		translatedTitle, err := h.runtime.TranslateAPI.Translate(sourceTitle, sourceLang, langContent.Language)
		if err != nil {
			logger.GetGlobalLogger("shein/product").Warnf("翻译到语言 %s 失败: %v，使用源标题作为后备", langContent.Language, err)
			langContent.Name = sourceTitle
			continue
		}
		langContent.Name = translatedTitle
		_ = ctx
	}
}

func (h *SKCTranslationHandler) optimizeMultiLanguageContent(ctx context.Context, multiLanguageNameList *[]product.LanguageContent, sourceTitle string) {
	if multiLanguageNameList == nil || len(*multiLanguageNameList) == 0 {
		return
	}

	var englishContents []string
	var englishIndexes []int
	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]
		if langContent.Language == "en" && langContent.Name != "" {
			englishContents = append(englishContents, langContent.Name)
			englishIndexes = append(englishIndexes, i)
		}
	}
	if len(englishContents) == 0 {
		return
	}

	aiCtx, cancel := timeout.WithAIShortTimeout(ctx)
	defer cancel()

	cacheKey := aicache.HashKey(englishContents...)
	if h.runtime.AICache != nil {
		var cached []string
		if h.runtime.AICache.Get(aicache.TypeSKCTranslate, cacheKey, &cached) && len(cached) == len(englishContents) {
			for i, optimizedContent := range cached {
				if i >= len(englishIndexes) {
					break
				}
				langContent := &(*multiLanguageNameList)[englishIndexes[i]]
				cleanedName := strings.TrimSpace(optimizedContent)
				if len(cleanedName) >= 10 {
					if len(cleanedName) > 800 {
						cleanedName = h.truncateContent(cleanedName, 800)
					}
					langContent.Name = cleanedName
				}
			}
			return
		}
	}

	optimizedContents, err := h.batchOptimizeEnglishContent(aiCtx, englishContents, sourceTitle)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("批量优化英文内容失败: %v，保持原内容", err)
		return
	}

	for i, optimizedContent := range optimizedContents {
		if i >= len(englishIndexes) {
			break
		}
		langContent := &(*multiLanguageNameList)[englishIndexes[i]]
		cleanedName := strings.TrimSpace(optimizedContent)
		if len(cleanedName) < 10 {
			continue
		}
		if len(cleanedName) > 800 {
			cleanedName = h.truncateContent(cleanedName, 800)
		}
		langContent.Name = cleanedName
	}

	if h.runtime.AICache != nil && len(optimizedContents) > 0 {
		h.runtime.AICache.Set(aicache.TypeSKCTranslate, cacheKey, optimizedContents)
	}
}

func (h *SKCTranslationHandler) batchOptimizeEnglishContent(ctx context.Context, contents []string, sourceTitle string) ([]string, error) {
	systemPrompt := prompt.GlobalRegistry.Get(prompt.KSheinTranslationBatchOptimizeSystem, `你是一个专业的电商产品内容优化专家。请批量优化产品标题，使其更适合SHEIN平台销售。

要求：
1. 所有标题必须是英文，长度在10-800个字符之间
2. 突出产品主要特征和卖点
3. 使用简洁、吸引人的描述
4. 避免使用品牌名称和敏感词汇
5. 符合英语语法规范
6. 返回JSON格式：{"optimized_titles": ["优化后的标题1", "优化后的标题2", ...]}`)

	var contentList strings.Builder
	for i, content := range contents {
		contentList.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
	}

	userPrompt := fmt.Sprintf(`原始标题：%s

需要优化的英文内容列表：
%s
请批量优化这些英文产品标题，使其更适合SHEIN平台销售。返回JSON格式：{"optimized_titles": ["优化后的标题1", "优化后的标题2", ...]}`,
		sourceTitle,
		contentList.String())

	return h.callOpenAIForBatchOptimization(ctx, systemPrompt, userPrompt, len(contents))
}

func (h *SKCTranslationHandler) callOpenAIForBatchOptimization(ctx context.Context, systemPrompt, userPrompt string, expectedCount int) ([]string, error) {
	temperature := float32(0.7)
	messages := []openaiClient.ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}}
	req := &openaiClient.ChatCompletionRequest{Model: h.openaiClient.GetDefaultModel(), Messages: messages, Temperature: &temperature}
	resp, err := h.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用OpenAI API失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API返回空响应")
	}
	return h.parseBatchOptimizedResponse(resp.Choices[0].Message.Content, expectedCount)
}

func (h *SKCTranslationHandler) parseBatchOptimizedResponse(content string, expectedCount int) ([]string, error) {
	cleanContent := jsonx.CleanLLMResponse(content)
	type BatchOptimizedResponse struct {
		OptimizedTitles []string `json:"optimized_titles"`
	}
	var response BatchOptimizedResponse
	if err := jsonx.UnmarshalBytes([]byte(cleanContent), &response, "解析JSON响应失败"); err != nil {
		return nil, err
	}
	if len(response.OptimizedTitles) == 0 {
		return nil, fmt.Errorf("优化后的标题列表为空")
	}
	if len(response.OptimizedTitles) != expectedCount {
		logger.GetGlobalLogger("shein/product").Warnf("返回的优化标题数量(%d)与期望数量(%d)不匹配", len(response.OptimizedTitles), expectedCount)
	}
	return response.OptimizedTitles, nil
}

func (h *SKCTranslationHandler) callOpenAIForOptimization(ctx context.Context, client *openaiClient.Client, systemPrompt, userPrompt string) (string, error) {
	temperature := float32(0.7)
	messages := []openaiClient.ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}}
	req := &openaiClient.ChatCompletionRequest{Model: client.GetDefaultModel(), Messages: messages, Temperature: &temperature}
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("调用OpenAI API失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API返回空响应")
	}
	return h.parseOptimizedResponse(resp.Choices[0].Message.Content)
}

func (h *SKCTranslationHandler) parseOptimizedResponse(content string) (string, error) {
	cleanContent := jsonx.CleanLLMResponse(content)
	type OptimizedResponse struct {
		OptimizedTitle string `json:"optimized_title"`
	}
	var response OptimizedResponse
	if err := jsonx.UnmarshalBytes([]byte(cleanContent), &response, "解析JSON响应失败"); err != nil {
		return "", err
	}
	if response.OptimizedTitle == "" {
		return "", fmt.Errorf("优化后的标题为空")
	}
	return response.OptimizedTitle, nil
}

func (h *SKCTranslationHandler) truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	truncated := content[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 && lastSpace > maxLength-50 {
		truncated = truncated[:lastSpace]
	}
	return strings.TrimSpace(truncated)
}

func (h *SKCTranslationHandler) selectPrimaryDisplayLanguage(targetLanguages []string, multiLanguageNameList []product.LanguageContent, sourceTitle string) product.LanguageContent {
	if len(targetLanguages) == 0 {
		if len(multiLanguageNameList) > 0 && multiLanguageNameList[0].Name != "" {
			return multiLanguageNameList[0]
		}
		return product.LanguageContent{Language: "en", Name: sourceTitle}
	}

	primaryTargetLang := targetLanguages[0]
	for _, langContent := range multiLanguageNameList {
		if langContent.Language == primaryTargetLang && langContent.Name != "" {
			return langContent
		}
	}
	return product.LanguageContent{Language: primaryTargetLang, Name: sourceTitle}
}
