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
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

type SKCTranslationHandler struct {
	runtime      *SKCRuntimeInput
	openaiClient openaiClient.ChatCompleter
}

func NewSKCTranslationHandler(runtime *SKCRuntimeInput, openaiClient openaiClient.ChatCompleter) *SKCTranslationHandler {
	return &SKCTranslationHandler{
		runtime:      runtime,
		openaiClient: openaiClient,
	}
}

func (h *SKCTranslationHandler) CreateSKC(ctx context.Context, params shein.SKCCreationParams) product.SKC {
	targetLanguages := submitprep.GetTargetLanguagesByRegion(h.runtime.Region)
	sourceTitle := h.findBestSourceTitle(params)
	sourceLang := h.detectTitleLanguage(sourceTitle)
	multiLanguageNameList := h.initializeMultiLanguageContent(targetLanguages)

	h.translateToAllLanguages(ctx, sourceTitle, sourceLang, &multiLanguageNameList)
	h.optimizeMultiLanguageContent(ctx, &multiLanguageNameList, sourceTitle)

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
	logger.GetGlobalLogger("shein/product").Debug("start resolving SKC source title")

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
	return submitprep.DetectLanguage(title, "")
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
			logger.GetGlobalLogger("shein/product").Warnf("translate to %s failed: %v, use source title as fallback", langContent.Language, err)
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
		logger.GetGlobalLogger("shein/product").Warnf("batch optimize english content failed: %v, keep original content", err)
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
	systemPrompt := prompt.GlobalRegistry.Get(prompt.KSheinTranslationBatchOptimizeSystem, `You are an e-commerce content optimization expert. Optimize product titles for SHEIN listings.
Requirements:
1. Output must be English and 10-800 characters long.
2. Highlight major product features and selling points.
3. Keep wording concise and attractive.
4. Avoid brand names and sensitive words.
5. Return JSON: {"optimized_titles": ["title1", "title2"]}`)

	var contentList strings.Builder
	for i, content := range contents {
		contentList.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
	}

	userPrompt := fmt.Sprintf("Original title:\n%s\n\nEnglish content list to optimize:\n%s\nReturn JSON in the format {\"optimized_titles\": [\"title1\", \"title2\"]}.", sourceTitle, contentList.String())
	return h.callOpenAIForBatchOptimization(ctx, systemPrompt, userPrompt, len(contents))
}

func (h *SKCTranslationHandler) callOpenAIForBatchOptimization(ctx context.Context, systemPrompt, userPrompt string, expectedCount int) ([]string, error) {
	temperature := float32(0.7)
	messages := []openaiClient.ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}}
	req := &openaiClient.ChatCompletionRequest{Model: h.openaiClient.GetDefaultModel(), Messages: messages, Temperature: &temperature}
	resp, err := h.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("call OpenAI API failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API returned no choices")
	}
	return h.parseBatchOptimizedResponse(resp.Choices[0].Message.Content, expectedCount)
}

func (h *SKCTranslationHandler) parseBatchOptimizedResponse(content string, expectedCount int) ([]string, error) {
	cleanContent := jsonx.CleanLLMResponse(content)
	type BatchOptimizedResponse struct {
		OptimizedTitles []string `json:"optimized_titles"`
	}
	var response BatchOptimizedResponse
	if err := jsonx.UnmarshalBytes([]byte(cleanContent), &response, "failed to parse JSON response"); err != nil {
		return nil, err
	}
	if len(response.OptimizedTitles) == 0 {
		return nil, fmt.Errorf("optimized title list is empty")
	}
	if len(response.OptimizedTitles) != expectedCount {
		logger.GetGlobalLogger("shein/product").Warnf("optimized title count %d does not match expected %d", len(response.OptimizedTitles), expectedCount)
	}
	return response.OptimizedTitles, nil
}

func (h *SKCTranslationHandler) callOpenAIForOptimization(ctx context.Context, client *openaiClient.Client, systemPrompt, userPrompt string) (string, error) {
	temperature := float32(0.7)
	messages := []openaiClient.ChatCompletionMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}}
	req := &openaiClient.ChatCompletionRequest{Model: client.GetDefaultModel(), Messages: messages, Temperature: &temperature}
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("call OpenAI API failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API returned no choices")
	}
	return h.parseOptimizedResponse(resp.Choices[0].Message.Content)
}

func (h *SKCTranslationHandler) parseOptimizedResponse(content string) (string, error) {
	cleanContent := jsonx.CleanLLMResponse(content)
	type OptimizedResponse struct {
		OptimizedTitle string `json:"optimized_title"`
	}
	var response OptimizedResponse
	if err := jsonx.UnmarshalBytes([]byte(cleanContent), &response, "failed to parse JSON response"); err != nil {
		return "", err
	}
	if response.OptimizedTitle == "" {
		return "", fmt.Errorf("optimized title is empty")
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
