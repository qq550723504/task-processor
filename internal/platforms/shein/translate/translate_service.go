// Package modules 提供SHEIN平台的翻译处理功能，包括产品标题和描述的多语言翻译
package translate

import (
	"fmt"
	"strings"
	"task-processor/internal/domain/model"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/contextutil"
	"task-processor/internal/platforms/shein/api/product"
	shein "task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/content"
)

// TranslateHandler 翻译处理器
type TranslateHandler struct {
	openaiClient     *openaiClient.Client
	languageDetector *LanguageDetector
	contentOptimizer *content.ContentOptimizer
	textCleaner      *content.TextCleaner
}

// NewTranslateHandler 创建新的翻译处理器
func NewTranslateHandler(config *openaiClient.ClientConfig) *TranslateHandler {
	return &TranslateHandler{
		openaiClient:     openaiClient.NewClient(config),
		languageDetector: NewLanguageDetector(),
		contentOptimizer: content.NewContentOptimizer(openaiClient.NewClient(config)),
		textCleaner:      content.NewTextCleaner(),
	}
}

// Name 返回处理器名称
func (h *TranslateHandler) Name() string {
	return "翻译产品信息"
}

// Handle 执行翻译处理
func (h *TranslateHandler) Handle(ctx *shein.TaskContext) error {

	features := strings.Join(ctx.AmazonProduct.Features, ", ")

	// 去除标题和描述中的品牌词
	cleanedTitle := h.textCleaner.RemoveBrandFromText(ctx.AmazonProduct.Title, ctx.AmazonProduct.Brand)
	cleanedDescription := h.textCleaner.RemoveBrandFromText(ctx.AmazonProduct.Description, ctx.AmazonProduct.Brand)

	// 检测源文本的实际语言
	detectedLang := h.languageDetector.DetectLanguage(cleanedTitle, cleanedDescription)

	// 使用AI重构标题与描述（仅当检测到的语言是英文时）
	var optimizedTitle, optimizedDescription string
	var err error
	if detectedLang == "en" {
		// 创建带超时的context用于AI调用 - 使用传入的context
		aiCtx, cancel := contextutil.WithAIShortTimeout(ctx.Context)
		defer cancel()

		optimizedTitle, optimizedDescription, err = h.contentOptimizer.OptimizeTitleAndDescription(aiCtx, cleanedTitle, cleanedDescription, features)
		if err != nil {
			optimizedTitle = cleanedTitle
			optimizedDescription = cleanedDescription
		}
	} else {
		// 如果不是英文，直接使用清理后的文本
		optimizedTitle = cleanedTitle
		optimizedDescription = cleanedDescription
	}

	// 确保优化后的标题和描述不为空
	optimizedTitle = h.ensureValidTitle(optimizedTitle, cleanedTitle, ctx.AmazonProduct)
	optimizedDescription = h.ensureValidDescription(optimizedDescription, cleanedDescription, ctx.AmazonProduct)

	// 初始化多语言描述列表
	if ctx.ProductData.MultiLanguageDescList == nil {
		ctx.ProductData.MultiLanguageDescList = []product.LanguageContent{}
	}

	// 根据站点信息确定需要翻译的目标语言
	targetLanguages := GetTargetLanguagesByRegion(ctx.Task.Region)
	if len(targetLanguages) == 0 {
		return fmt.Errorf("不支持的区域: %s", ctx.Task.Region)
	}

	// 翻译产品名称（传入检测到的源语言）
	if err := h.translateProductName(ctx, targetLanguages, optimizedTitle, detectedLang); err != nil {
		return fmt.Errorf("翻译产品名称失败: %w", err)
	}

	// 翻译产品描述（传入检测到的源语言）
	if err := h.translateProductDescription(ctx, targetLanguages, optimizedDescription, detectedLang); err != nil {
		// 翻译Description失败时，使用Features作为备选

		// 使用Features作为描述
		fallbackDescription := features
		if strings.TrimSpace(fallbackDescription) == "" {
			fallbackDescription = "High quality product with excellent features and design."
		}

		// 重新尝试翻译Features
		if err := h.translateProductDescription(ctx, targetLanguages, fallbackDescription, detectedLang); err != nil {
			return fmt.Errorf("翻译产品描述失败: %w", err)
		}
	}

	return nil
}

// ensureValidTitle 确保标题有效
func (h *TranslateHandler) ensureValidTitle(optimizedTitle, cleanedTitle string, amazonProduct *model.Product) string {
	if strings.TrimSpace(optimizedTitle) == "" {
		if strings.TrimSpace(cleanedTitle) != "" {
			return cleanedTitle
		} else if amazonProduct != nil && strings.TrimSpace(amazonProduct.Title) != "" {
			return amazonProduct.Title
		} else {
			return "Quality Product"
		}
	}
	return optimizedTitle
}

// ensureValidDescription 确保描述有效
func (h *TranslateHandler) ensureValidDescription(optimizedDescription, cleanedDescription string, amazonProduct *model.Product) string {
	if strings.TrimSpace(optimizedDescription) == "" {
		if strings.TrimSpace(cleanedDescription) != "" {
			return cleanedDescription
		} else if amazonProduct != nil && strings.TrimSpace(amazonProduct.Description) != "" {
			return amazonProduct.Description
		} else {
			return "High quality product with excellent features and design."
		}
	}
	return optimizedDescription
}

// translateProductName 翻译产品名称
func (h *TranslateHandler) translateProductName(ctx *shein.TaskContext, targetLanguages []string, productName string, sourceLang string) error {
	// 获取原始产品名称，如果为空则使用默认值
	originalName := strings.TrimSpace(productName)
	if originalName == "" {
		// 尝试从 Amazon 产品数据中获取标题
		if ctx.AmazonProduct != nil && strings.TrimSpace(ctx.AmazonProduct.Title) != "" {
			originalName = strings.TrimSpace(ctx.AmazonProduct.Title)
		} else {
			// 如果仍然为空，使用默认产品名称
			originalName = "Quality Product"
		}
	}

	// 如果多语言名称列表为空，则初始化
	if ctx.ProductData.MultiLanguageNameList == nil {
		ctx.ProductData.MultiLanguageNameList = []product.LanguageContent{}
	}

	// 添加源语言的原始名称，避免重复
	h.addLanguageContentIfNotExists(&ctx.ProductData.MultiLanguageNameList, sourceLang, originalName)

	// 为每种目标语言进行翻译
	for _, targetLang := range targetLanguages {
		// 如果目标语言与源语言相同，跳过翻译
		if targetLang == sourceLang {
			continue
		}

		translatedName, err := ctx.TranslateAPI.Translate(originalName, sourceLang, targetLang)
		if err != nil {
			return fmt.Errorf("翻译产品名称为目标语言 %s 失败: %w", targetLang, err)
		}

		// 添加翻译后的名称，避免重复
		h.addLanguageContentIfNotExists(&ctx.ProductData.MultiLanguageNameList, targetLang, translatedName)
	}

	return nil
}

// translateProductDescription 翻译产品描述
func (h *TranslateHandler) translateProductDescription(ctx *shein.TaskContext, targetLanguages []string, productDescription string, sourceLang string) error {
	// 检查产品描述是否为空
	if strings.TrimSpace(productDescription) == "" {
		// 如果描述为空，使用产品标题作为默认描述
		if ctx.AmazonProduct != nil && strings.TrimSpace(ctx.AmazonProduct.Title) != "" {
			productDescription = ctx.AmazonProduct.Title
		} else {
			// 如果标题也为空，使用默认描述
			productDescription = "High quality product with excellent features and design."
		}
	}

	// 验证并截断描述长度（SHEIN限制为5000个字符）
	productDescription = h.validateAndTruncateDescription(productDescription)

	// 添加源语言的原始描述，避免重复
	h.addLanguageContentIfNotExists(&ctx.ProductData.MultiLanguageDescList, sourceLang, productDescription)

	// 为每种目标语言进行翻译
	for _, targetLang := range targetLanguages {
		// 如果目标语言与源语言相同，跳过翻译
		if targetLang == sourceLang {
			continue
		}

		// 检查是否已经存在该语言的翻译
		if h.existsLanguage(ctx.ProductData.MultiLanguageDescList, targetLang) {
			continue
		}

		translatedDesc, err := ctx.TranslateAPI.Translate(productDescription, sourceLang, targetLang)
		if err != nil {
			return fmt.Errorf("翻译产品描述为目标语言 %s 失败: %w", targetLang, err)
		}

		// 验证并截断翻译后的描述长度
		translatedDesc = h.validateAndTruncateDescription(translatedDesc)

		// 添加翻译后的描述，避免重复
		h.addLanguageContentIfNotExists(&ctx.ProductData.MultiLanguageDescList, targetLang, translatedDesc)
	}

	return nil
}

// validateAndTruncateDescription 验证并截断描述长度
// SHEIN平台限制商品描述不能超过5000个字符
func (h *TranslateHandler) validateAndTruncateDescription(description string) string {
	const maxDescriptionLength = 5000

	// 如果描述长度在限制范围内，直接返回
	if len(description) <= maxDescriptionLength {
		return description
	}

	// 截断描述到5000个字符
	truncated := description[:maxDescriptionLength]

	// 尝试在最后一个完整句子处截断（查找最后一个句号、问号或感叹号）
	lastPeriod := strings.LastIndexAny(truncated, ".!?")
	if lastPeriod > 0 && lastPeriod > maxDescriptionLength-200 {
		// 如果找到句号且位置合理（不会损失太多内容），在句号处截断
		truncated = truncated[:lastPeriod+1]
	}

	return strings.TrimSpace(truncated)
}

// existsLanguage 检查是否存在指定语言的翻译
func (h *TranslateHandler) existsLanguage(languageList []product.LanguageContent, language string) bool {
	for _, item := range languageList {
		if item.Language == language {
			return true
		}
	}
	return false
}

// addLanguageContentIfNotExists 如果不存在则添加语言内容
func (h *TranslateHandler) addLanguageContentIfNotExists(languageList *[]product.LanguageContent, language, content string) {
	if !h.existsLanguage(*languageList, language) {
		*languageList = append(*languageList, product.LanguageContent{
			Language: language,
			Name:     content,
		})
	}
}



