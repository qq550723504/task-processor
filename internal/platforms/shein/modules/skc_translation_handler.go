// Package modules 提供SHEIN平台SKC翻译处理功能
package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/shein/api/product"
	"time"

	"github.com/sirupsen/logrus"
)

// SKCTranslationHandler SKC翻译处理器
type SKCTranslationHandler struct {
	taskContext      *TaskContext
	contentOptimizer *ContentOptimizer
}

// NewSKCTranslationHandler 创建新的SKC翻译处理器
func NewSKCTranslationHandler(taskContext *TaskContext, openaiConfig *openaiClient.ClientConfig) *SKCTranslationHandler {
	var contentOptimizer *ContentOptimizer
	if openaiConfig != nil {
		contentOptimizer = NewContentOptimizer(openaiClient.NewClient(openaiConfig))
	}

	return &SKCTranslationHandler{
		taskContext:      taskContext,
		contentOptimizer: contentOptimizer,
	}
}

// CreateSKC 创建SKC的工厂函数
func (h *SKCTranslationHandler) CreateSKC(ctx *TaskContext, params SKCCreationParams) product.SKC {
	// 1. 获取目标语言列表
	targetLanguages := GetTargetLanguagesByRegion(ctx.Task.Region)

	// 2. 查找标题作为翻译源
	sourceTitle := h.findBestSourceTitle(ctx, params)

	// 3. 检测源标题的语言
	sourceLang := h.detectTitleLanguage(sourceTitle)

	// 4. 初始化多语言内容结构
	multiLanguageNameList := h.initializeMultiLanguageContent(targetLanguages)

	// 5. 翻译到所有目标语言
	h.translateToAllLanguages(ctx, sourceTitle, sourceLang, &multiLanguageNameList)

	// 6. 使用AI优化多语言内容
	h.optimizeMultiLanguageContent(ctx, &multiLanguageNameList, sourceTitle)

	// 7. 选择主要显示语言
	primaryLanguageContent := h.selectPrimaryDisplayLanguage(targetLanguages, multiLanguageNameList, sourceTitle)

	skc := product.SKC{
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
	return skc
}

// findBestSourceTitle 查找最佳的源标题作为翻译源
func (h *SKCTranslationHandler) findBestSourceTitle(ctx *TaskContext, params SKCCreationParams) string {
	logrus.Debugf("🔍 开始查找源标题...")

	// 优先尝试根据SKU的SupplierSKU反向查找对应的ASIN，然后匹配变体标题
	if ctx.Variants != nil && len(*ctx.Variants) > 0 && len(params.SKUS) > 0 {
		// 获取当前SKC对应的SupplierSKU（从第一个SKU获取）
		if len(params.SKUS) > 0 && params.SKUS[0].SupplierSKU != "" {
			supplierSKU := params.SKUS[0].SupplierSKU
			logrus.Debugf("🎯 通过SupplierSKU %s 反向查找对应的ASIN", supplierSKU)

			// 通过AsinSkuMap反向查找ASIN
			var targetASIN string
			if ctx.AsinSkuMap != nil {
				for asin, sku := range ctx.AsinSkuMap {
					if sku == supplierSKU {
						targetASIN = asin
						logrus.Debugf("✅ 找到对应的ASIN: %s -> %s", supplierSKU, targetASIN)
						break
					}
				}
			}

			// 如果找到了ASIN，查找对应的变体标题
			if targetASIN != "" {
				for _, variant := range *ctx.Variants {
					if variant.Asin == targetASIN && variant.Title != "" {
						logrus.Infof("✅ 找到匹配变体标题: ASIN=%s, Title=%s", variant.Asin, variant.Title)
						return variant.Title
					}
				}
				logrus.Debugf("⚠️ ASIN %s 对应的变体标题为空", targetASIN)
			} else {
				logrus.Debugf("⚠️ 未找到SupplierSKU %s 对应的ASIN", supplierSKU)
			}
		}

		// 如果没有找到匹配的变体标题，尝试使用任何有效的变体标题
		for _, variant := range *ctx.Variants {
			if variant.Title != "" {
				logrus.Infof("✅ 使用其他变体标题: ASIN=%s, Title=%s", variant.Asin, variant.Title)
				return variant.Title
			}
		}
	}

	// 如果没有找到变体标题，尝试使用产品标题
	if ctx.AmazonProduct.Title != "" {
		logrus.Infof("✅ 使用产品标题: %s", ctx.AmazonProduct.Title)
		return ctx.AmazonProduct.Title
	}

	logrus.Warnf("⚠️ 未找到有效的标题")
	return ""
}

// detectTitleLanguage 检测标题的语言
func (h *SKCTranslationHandler) detectTitleLanguage(title string) string {
	title = strings.TrimSpace(title)

	if title == "" {
		return "en" // 默认返回英文
	}

	// 简单的语言检测：统计不同字符集的字符数量
	var japaneseCount, chineseCount, englishCount int

	for _, r := range title {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF): // 平假名和片假名
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF: // 中日韩统一表意文字
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	// 判断主要语言
	if japaneseCount > chineseCount && japaneseCount > englishCount {
		logrus.Infof("🔍 检测到标题语言: 日语")
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		logrus.Infof("🔍 检测到标题语言: 中文")
		return "zh"
	}

	logrus.Infof("🔍 检测到标题语言: 英文")
	return "en"
}

// initializeMultiLanguageContent 初始化多语言内容结构
func (h *SKCTranslationHandler) initializeMultiLanguageContent(targetLanguages []string) []product.LanguageContent {
	logrus.Debugf("🌐 初始化多语言内容结构，目标语言数量: %d", len(targetLanguages))

	multiLanguageNameList := make([]product.LanguageContent, 0, len(targetLanguages))

	for _, lang := range targetLanguages {
		multiLanguageNameList = append(multiLanguageNameList, product.LanguageContent{
			Language: lang,
			Name:     "", // 初始化为空，后续通过翻译填充
		})
		logrus.Debugf("📝 初始化语言: %s", lang)
	}

	return multiLanguageNameList
}

// translateToAllLanguages 翻译到所有目标语言
func (h *SKCTranslationHandler) translateToAllLanguages(ctx *TaskContext, sourceTitle string, sourceLang string, multiLanguageNameList *[]product.LanguageContent) {
	if ctx.ShopClient == nil || sourceTitle == "" {
		logrus.Warnf("⚠️ 跳过翻译：ShopClient为空(%v) 或 源标题为空(%v)", ctx.ShopClient == nil, sourceTitle == "")
		return
	}

	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]

		// 如果目标语言与源语言相同，直接设置原标题
		if langContent.Language == sourceLang {
			langContent.Name = sourceTitle
			logrus.Debugf("✅ 设置源语言(%s)标题: %s", sourceLang, sourceTitle)
			continue
		}

		// 翻译到目标语言
		translatedTitle, err := ctx.ShopClient.Translate(sourceTitle, sourceLang, langContent.Language)
		if err != nil {
			logrus.Warnf("❌ 翻译到语言 %s 失败: %v，使用源标题作为后备", langContent.Language, err)
			langContent.Name = sourceTitle // 翻译失败时使用源标题作为后备
			continue
		}

		langContent.Name = translatedTitle
	}
}

// optimizeMultiLanguageContent 使用AI优化多语言内容
func (h *SKCTranslationHandler) optimizeMultiLanguageContent(ctx *TaskContext, multiLanguageNameList *[]product.LanguageContent, sourceTitle string) {
	logrus.Debugf("🤖 开始AI优化多语言内容...")

	// 检查前置条件
	if multiLanguageNameList == nil || len(*multiLanguageNameList) == 0 {
		logrus.Warnf("⚠️ 跳过AI优化：多语言内容为空")
		return
	}

	// 检查是否有内容优化器
	if h.contentOptimizer == nil || h.contentOptimizer.openaiClient == nil {
		logrus.Warnf("⚠️ 跳过AI优化：OpenAI客户端未配置")
		return
	}

	// 收集所有需要优化的英文内容
	var englishContents []string
	var englishIndexes []int

	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]

		// 只收集英文内容
		if langContent.Language == "en" && langContent.Name != "" {
			englishContents = append(englishContents, langContent.Name)
			englishIndexes = append(englishIndexes, i)
		}
	}

	if len(englishContents) == 0 {
		logrus.Debugf("⚠️ 没有找到需要优化的英文内容")
		return
	}

	// 创建带超时的上下文
	aiCtx, cancel := context.WithTimeout(ctx.Context, 30*time.Second)
	defer cancel()

	// 一次性批量优化所有英文内容
	optimizedContents, err := h.batchOptimizeEnglishContent(aiCtx, englishContents, sourceTitle)
	if err != nil {
		logrus.Warnf("❌ 批量优化英文内容失败: %v，保持原内容", err)
		return
	}

	// 更新优化后的内容
	for i, optimizedContent := range optimizedContents {
		if i >= len(englishIndexes) {
			break
		}

		langContent := &(*multiLanguageNameList)[englishIndexes[i]]

		// 验证和清理优化后的内容
		cleanedName := strings.TrimSpace(optimizedContent)
		if len(cleanedName) < 10 {
			logrus.Warnf("⚠️ 优化后的英文内容太短，保持原内容")
			continue
		}

		if len(cleanedName) > 800 {
			cleanedName = h.truncateContent(cleanedName, 800)
			logrus.Debugf("✂️ 截断英文内容到800字符")
		}

		// 更新内容
		langContent.Name = cleanedName
		logrus.Infof("✅ 英文内容优化完成: %s", cleanedName)
	}

	logrus.Infof("🎉 AI批量优化多语言内容完成")
}

// batchOptimizeEnglishContent 批量优化英文内容
func (h *SKCTranslationHandler) batchOptimizeEnglishContent(ctx context.Context, contents []string, sourceTitle string) ([]string, error) {
	// 构建批量优化的系统提示词
	systemPrompt := `你是一个专业的电商产品内容优化专家。请批量优化产品标题，使其更适合SHEIN平台销售。

要求：
1. 所有标题必须是英文，长度在10-800个字符之间
2. 突出产品主要特征和卖点
3. 使用简洁、吸引人的描述
4. 避免使用品牌名称和敏感词汇
5. 符合英语语法规范
6. 返回JSON格式：{"optimized_titles": ["优化后的标题1", "优化后的标题2", ...]}`

	// 构建用户提示词，包含所有需要优化的内容
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

	// 调用OpenAI API
	return h.callOpenAIForBatchOptimization(ctx, systemPrompt, userPrompt, len(contents))
}

// callOpenAIForBatchOptimization 调用OpenAI API进行批量优化
func (h *SKCTranslationHandler) callOpenAIForBatchOptimization(ctx context.Context, systemPrompt, userPrompt string, expectedCount int) ([]string, error) {
	temperature := float32(0.7)

	// 构建消息
	messages := []openaiClient.ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	// 构建请求
	req := &openaiClient.ChatCompletionRequest{
		Model:       h.contentOptimizer.openaiClient.GetDefaultModel(),
		Messages:    messages,
		Temperature: &temperature,
	}

	// 调用OpenAI API
	resp, err := h.contentOptimizer.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用OpenAI API失败: %w", err)
	}

	// 检查响应是否有效
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API返回空响应")
	}

	// 解析响应内容
	content := resp.Choices[0].Message.Content

	// 解析批量JSON响应
	return h.parseBatchOptimizedResponse(content, expectedCount)
}

// parseBatchOptimizedResponse 解析批量优化响应
func (h *SKCTranslationHandler) parseBatchOptimizedResponse(content string, expectedCount int) ([]string, error) {
	// 清理内容
	cleanContent := strings.TrimSpace(content)
	cleanContent = strings.TrimPrefix(cleanContent, "```json")
	cleanContent = strings.TrimPrefix(cleanContent, "```")
	cleanContent = strings.TrimSuffix(cleanContent, "```")
	cleanContent = strings.TrimSpace(cleanContent)

	// 尝试解析JSON
	type BatchOptimizedResponse struct {
		OptimizedTitles []string `json:"optimized_titles"`
	}

	var response BatchOptimizedResponse
	if err := json.Unmarshal([]byte(cleanContent), &response); err != nil {
		return nil, fmt.Errorf("解析JSON响应失败: %w", err)
	}

	if len(response.OptimizedTitles) == 0 {
		return nil, fmt.Errorf("优化后的标题列表为空")
	}

	// 如果返回的数量不匹配，记录警告但继续处理
	if len(response.OptimizedTitles) != expectedCount {
		logrus.Warnf("⚠️ 返回的优化标题数量(%d)与期望数量(%d)不匹配", len(response.OptimizedTitles), expectedCount)
	}

	return response.OptimizedTitles, nil
}

// callOpenAIForOptimization 调用OpenAI API进行优化
func (h *SKCTranslationHandler) callOpenAIForOptimization(ctx context.Context, client *openaiClient.Client, systemPrompt, userPrompt string) (string, error) {
	temperature := float32(0.7)

	// 构建消息
	messages := []openaiClient.ChatCompletionMessage{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}

	// 构建请求
	req := &openaiClient.ChatCompletionRequest{
		Model:       client.GetDefaultModel(),
		Messages:    messages,
		Temperature: &temperature,
	}

	// 调用OpenAI API
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("调用OpenAI API失败: %w", err)
	}

	// 检查响应是否有效
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API返回空响应")
	}

	// 解析响应内容
	content := resp.Choices[0].Message.Content

	// 解析JSON响应
	return h.parseOptimizedResponse(content)
}

// parseOptimizedResponse 解析优化响应
func (h *SKCTranslationHandler) parseOptimizedResponse(content string) (string, error) {
	// 清理内容
	cleanContent := strings.TrimSpace(content)
	cleanContent = strings.TrimPrefix(cleanContent, "```json")
	cleanContent = strings.TrimPrefix(cleanContent, "```")
	cleanContent = strings.TrimSuffix(cleanContent, "```")
	cleanContent = strings.TrimSpace(cleanContent)

	// 尝试解析JSON
	type OptimizedResponse struct {
		OptimizedTitle string `json:"optimized_title"`
	}

	var response OptimizedResponse
	if err := json.Unmarshal([]byte(cleanContent), &response); err != nil {
		return "", fmt.Errorf("解析JSON响应失败: %w", err)
	}

	if response.OptimizedTitle == "" {
		return "", fmt.Errorf("优化后的标题为空")
	}

	return response.OptimizedTitle, nil
}

// truncateContent 截断内容到指定长度
func (h *SKCTranslationHandler) truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	// 在单词边界处截断
	truncated := content[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 && lastSpace > maxLength-50 {
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated)
}

// selectPrimaryDisplayLanguage 选择主要显示语言
func (h *SKCTranslationHandler) selectPrimaryDisplayLanguage(targetLanguages []string, multiLanguageNameList []product.LanguageContent, sourceTitle string) product.LanguageContent {
	if len(targetLanguages) == 0 {
		// 如果没有目标语言，尝试从多语言列表中选择第一个有效的
		if len(multiLanguageNameList) > 0 && multiLanguageNameList[0].Name != "" {
			logrus.Infof("📋 无目标语言，使用多语言列表第一项作为主要显示语言: %s", multiLanguageNameList[0].Language)
			return multiLanguageNameList[0]
		}
		// 最后的后备方案
		logrus.Infof("📋 无目标语言，使用源标题作为主要显示语言")
		return product.LanguageContent{
			Language: "en",
			Name:     sourceTitle,
		}
	}

	// 使用第一个目标语言作为主要显示语言
	primaryTargetLang := targetLanguages[0]
	logrus.Infof("🎯 选择主要显示语言: %s", primaryTargetLang)

	// 在多语言列表中查找对应的翻译内容
	for _, langContent := range multiLanguageNameList {
		if langContent.Language == primaryTargetLang && langContent.Name != "" {
			logrus.Infof("✅ 使用目标语言 %s 作为主要显示标题: %s", primaryTargetLang, langContent.Name)
			return langContent
		}
	}

	// 如果没有找到目标语言的翻译，使用源标题作为后备
	logrus.Warnf("⚠️ 未找到语言 %s 的翻译内容，使用源标题作为后备", primaryTargetLang)
	return product.LanguageContent{
		Language: primaryTargetLang,
		Name:     sourceTitle,
	}
}
