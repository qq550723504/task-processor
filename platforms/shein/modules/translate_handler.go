package modules

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"task-processor/common/shein/api/product"
	openaiClient "task-processor/openai"
	"time"
)

// GetTargetLanguagesByRegion 根据区域获取目标语言列表
func GetTargetLanguagesByRegion(region string) []string {
	// 根据不同的区域返回不同的目标语言
	switch region {
	case "US", "MX":
		// 美国和墨西哥站点需要翻译为西班牙语
		return []string{"en", "es"}
	case "FR", "DE", "IT", "ES":
		// 欧洲站点需要翻译为德语、西班牙语、法语、意大利语
		return []string{"de", "es", "fr", "it", "en"}
	case "JP":
		// 日本站点需要日语
		return []string{"ja", "en"}
	case "SA", "AE":
		// 沙特和阿联酋站点需要阿拉伯语
		return []string{"ar", "en"}
	default:
		// 默认返回空列表，表示不支持的区域
		return []string{"en"}
	}
}

// TranslateHandler 翻译处理器
type TranslateHandler struct {
	openaiClient *openaiClient.Client
}

// NewTranslateHandler 创建新的翻译处理器
func NewTranslateHandler(config *openaiClient.ClientConfig) *TranslateHandler {
	return &TranslateHandler{
		openaiClient: openaiClient.NewClient(config),
	}
}

// Name 返回处理器名称
func (h *TranslateHandler) Name() string {
	return "翻译产品信息"
}

// Handle 执行翻译处理
func (h *TranslateHandler) Handle(ctx *TaskContext) error {

	// 检查是否已获取店铺客户端
	if ctx.ShopClient == nil {
		return fmt.Errorf("店铺客户端未获取，请先执行获取店铺API客户端步骤")
	}

	features := strings.Join(ctx.AmazonProduct.Features, ", ")

	// 去除标题和描述中的品牌词
	cleanedTitle := h.removeBrandFromText(ctx.AmazonProduct.Title, ctx.AmazonProduct.Brand)
	cleanedDescription := h.removeBrandFromText(ctx.AmazonProduct.Description, ctx.AmazonProduct.Brand)

	// 检测源文本的实际语言
	detectedLang := h.detectLanguage(cleanedTitle, cleanedDescription)

	// 使用AI重构标题与描述（仅当检测到的语言是英文时）
	var optimizedTitle, optimizedDescription string
	var err error
	if detectedLang == "en" {
		optimizedTitle, optimizedDescription, err = h.optimizeTitleAndDescriptionWithAI(cleanedTitle, cleanedDescription, features)
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
	if strings.TrimSpace(optimizedTitle) == "" {
		if strings.TrimSpace(cleanedTitle) != "" {
			optimizedTitle = cleanedTitle
		} else if ctx.AmazonProduct != nil && strings.TrimSpace(ctx.AmazonProduct.Title) != "" {
			optimizedTitle = ctx.AmazonProduct.Title
		} else {
			optimizedTitle = "Quality Product"
		}
	}

	if strings.TrimSpace(optimizedDescription) == "" {
		if strings.TrimSpace(cleanedDescription) != "" {
			optimizedDescription = cleanedDescription
		} else if ctx.AmazonProduct != nil && strings.TrimSpace(ctx.AmazonProduct.Description) != "" {
			optimizedDescription = ctx.AmazonProduct.Description
		} else {
			optimizedDescription = "High quality product with excellent features and design."
		}
	}

	// 初始化多语言描述列表
	if ctx.ProductData.MultiLanguageDescList == nil {
		ctx.ProductData.MultiLanguageDescList = []product.LanguageContent{}
	}

	// 根据站点信息确定需要翻译的目标语言
	targetLanguages := h.getTargetLanguagesByRegion(ctx.Task.Region)
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
		h.logFailedASIN(ctx, fmt.Sprintf("翻译产品描述失败，尝试使用Features: %v", err))

		// 使用Features作为描述
		fallbackDescription := features
		if strings.TrimSpace(fallbackDescription) == "" {
			fallbackDescription = "High quality product with excellent features and design."
		}

		// 重新尝试翻译Features
		if err := h.translateProductDescription(ctx, targetLanguages, fallbackDescription, detectedLang); err != nil {
			h.logFailedASIN(ctx, fmt.Sprintf("使用Features翻译也失败: %v", err))
			return fmt.Errorf("翻译产品描述失败: %w", err)
		}
	}

	return nil
}

// detectLanguage 检测文本的语言
func (h *TranslateHandler) detectLanguage(title, description string) string {
	// 合并标题和描述进行检测
	text := title + " " + description
	text = strings.TrimSpace(text)

	if text == "" {
		return "en" // 默认返回英文
	}

	// 简单的语言检测：统计不同字符集的字符数量
	var japaneseCount, chineseCount, englishCount int

	for _, r := range text {
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
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		return "zh" // 中文
	}

	return "en" // 默认英文
}

// removeBrandFromText 从文本中移除品牌词
func (h *TranslateHandler) removeBrandFromText(text, brand string) string {
	if text == "" {
		return text
	}

	// 如果品牌为空，直接返回原文本
	if brand == "" {
		return text
	}

	// 使用正则表达式移除品牌词（不区分大小写）
	brandPattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(brand))
	re := regexp.MustCompile(brandPattern)
	cleanedText := re.ReplaceAllString(text, "")

	// 清理多余的空格
	cleanedText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanedText, " ")
	cleanedText = strings.TrimSpace(cleanedText)

	// 如果清理后的文本为空，返回原始文本
	if cleanedText == "" {
		return text
	}

	return cleanedText
}

// optimizeTitleAndDescriptionWithAI 使用AI优化标题和描述
func (h *TranslateHandler) optimizeTitleAndDescriptionWithAI(title, description, features string) (optimizedTitle, optimizedDescription string, err error) {
	if h.openaiClient == nil {
		return title, description, fmt.Errorf("OpenAI客户端未初始化")
	}

	// 构建系统提示词
	systemPrompt := `你是一个专业的电商产品内容生成专家。请为Amazon产品生成适合SHEIN平台的英文标题和描述。

要求：
1. 标题和描述都必须是英文,不要有表情符号
2. 标题长度在80-800个字符之间，突出产品主要特征和卖点
3. 描述长度在100-2000个字符之间，详细介绍产品特征、材质、用途（注意：描述最多不能超过5000个字符）
4. 避免使用品牌名称
5. 使用简洁、吸引人的描述
6. 避免敏感词汇
7. 返回JSON格式，包含title和description字段

JSON格式示例：
{
  "title": "Women's Casual Cotton T-Shirt with Round Neck",
  "description": "This comfortable cotton blend t-shirt features a classic design perfect for casual wear. Soft fabric ensures all-day comfort while maintaining shape after washing. Available in multiple colors and sizes."
}`

	userPrompt := fmt.Sprintf(`Amazon产品信息：
标题：%s
详情：%s
要点：%s

请生成SHEIN平台的英文标题和描述（JSON格式）：`,
		title,
		description,
		features)

	// 设置参数
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
		Model:       h.openaiClient.GetDefaultModel(),
		Messages:    messages,
		Temperature: &temperature,
	}

	// 调用OpenAI API
	resp, err := h.openaiClient.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return title, description, fmt.Errorf("调用OpenAI API失败: %w", err)
	}

	// 检查响应是否有效
	if len(resp.Choices) == 0 {
		return title, description, fmt.Errorf("OpenAI API返回空响应")
	}

	// 解析响应内容
	content := resp.Choices[0].Message.Content

	// 提取优化后的标题和描述
	optimizedTitle, optimizedDescription = h.parseOptimizedContent(content, title, description)

	return optimizedTitle, optimizedDescription, nil
}

// parseOptimizedContent 解析优化后的内容
func (h *TranslateHandler) parseOptimizedContent(content, defaultTitle, defaultDescription string) (title, description string) {
	title = defaultTitle
	description = defaultDescription

	// 查找"优化标题："和"优化描述："标记
	titlePrefix := "优化标题："
	descPrefix := "优化描述："

	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, titlePrefix); ok {
			title = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, descPrefix); ok {
			description = strings.TrimSpace(after)
		}
	}

	// 如果解析失败，返回原始内容
	if title == "" {
		title = defaultTitle
	}
	if description == "" {
		description = defaultDescription
	}

	return title, description
}

// getTargetLanguagesByRegion 根据区域获取目标语言列表
func (h *TranslateHandler) getTargetLanguagesByRegion(region string) []string {
	return GetTargetLanguagesByRegion(region)
}

// translateProductName 翻译产品名称
func (h *TranslateHandler) translateProductName(ctx *TaskContext, targetLanguages []string, productName string, sourceLang string) error {
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

		translatedName, err := ctx.ShopClient.Translate(originalName, sourceLang, targetLang)
		if err != nil {
			return fmt.Errorf("翻译产品名称为目标语言 %s 失败: %w", targetLang, err)
		}

		// 添加翻译后的名称，避免重复
		h.addLanguageContentIfNotExists(&ctx.ProductData.MultiLanguageNameList, targetLang, translatedName)
	}

	return nil
}

// translateProductDescription 翻译产品描述
func (h *TranslateHandler) translateProductDescription(ctx *TaskContext, targetLanguages []string, productDescription string, sourceLang string) error {
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

		translatedDesc, err := ctx.ShopClient.Translate(productDescription, sourceLang, targetLang)
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

// logFailedASIN 记录翻译失败的ASIN到文件
func (h *TranslateHandler) logFailedASIN(ctx *TaskContext, reason string) {
	if ctx == nil || ctx.AmazonProduct == nil {
		return
	}

	asin := ctx.AmazonProduct.Asin
	if asin == "" {
		return
	}

	// 创建日志文件路径
	logFile := "failed_translations.log"

	// 打开文件（追加模式）
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// 写入日志
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] ASIN: %s, Region: %s, Reason: %s\n",
		timestamp, asin, ctx.Task.Region, reason)
	f.WriteString(logEntry)
}
