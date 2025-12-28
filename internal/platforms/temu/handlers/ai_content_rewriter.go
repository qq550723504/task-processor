// Package handlers 提供TEMU平台的各种处理器，包括AI内容重构等功能
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openaiClient "task-processor/internal/clients/openai"
	"task-processor/internal/pipeline"
	temucontext "task-processor/internal/platforms/temu/context"

	"github.com/sirupsen/logrus"
)

// AIContentRewriter AI内容重构器
type AIContentRewriter struct {
	logger       *logrus.Entry
	openaiClient *openaiClient.Client
}

// RewriteResult 重构结果
type RewriteResult struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	BulletPoints []string `json:"bullet_points"`
}

// NewAIContentRewriter 创建新的AI内容重构器
func NewAIContentRewriter(logger *logrus.Entry, openaiClient *openaiClient.Client) *AIContentRewriter {
	return &AIContentRewriter{
		logger:       logger,
		openaiClient: openaiClient,
	}
}

// NewAIContentRewriterHandler 创建新的AI内容重构处理器（用于pipeline）
func NewAIContentRewriterHandler(openaiConfig *openaiClient.ClientConfig) *AIContentRewriter {
	logger := logrus.WithField("handler", "ai_content_rewriter")

	var aiClient *openaiClient.Client
	if openaiConfig != nil {
		aiClient = openaiClient.NewClient(openaiConfig)
	}

	return NewAIContentRewriter(logger, aiClient)
}

// Name 返回处理器名称
func (r *AIContentRewriter) Name() string {
	return "AI内容重构处理器"
}

// HandleTemu 处理TEMU任务（实现TemuHandler接口）
func (r *AIContentRewriter) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	r.logger.Info("🤖 开始使用AI重构产品标题和描述")

	// 检查AI客户端是否可用
	if r.openaiClient == nil {
		r.logger.Warn("⚠️ OpenAI客户端未配置，跳过AI重构")
		return nil
	}

	// 构建提示词
	systemPrompt := r.buildSystemPrompt()
	userPrompt := r.buildUserPrompt(temuCtx)

	r.logger.Infof("📝 系统提示词长度: %d", len(systemPrompt))
	r.logger.Infof("📝 用户提示词长度: %d", len(userPrompt))

	// 调用AI进行重构 - 使用传入的context，添加超时控制
	aiCtx, cancel := context.WithTimeout(temuCtx.GetContext(), 60*time.Second)
	defer cancel()

	result, err := r.callAIForRewrite(aiCtx, systemPrompt, userPrompt)
	if err != nil {
		r.logger.WithError(err).Warn("❌ AI重构失败，保持原内容")
		return nil // 不返回错误，继续使用原内容
	}

	// 应用重构结果
	r.applyRewriteResult(temuCtx, result)

	r.logger.Info("✅ AI内容重构完成")
	return nil
}

// Handle 兼容原有的Handler接口（用于pipeline.AddHandler）
func (r *AIContentRewriter) Handle(ctx pipeline.TaskContext) error {
	// 尝试类型断言为TemuTaskContext
	if temuCtx, ok := ctx.(*temucontext.TemuTaskContext); ok {
		return r.HandleTemu(temuCtx)
	}
	// 如果不是TemuTaskContext，返回错误
	return fmt.Errorf("上下文类型错误，期望*temucontext.TemuTaskContext，实际类型: %T", ctx)
}

// buildSystemPrompt 构建系统提示词
func (r *AIContentRewriter) buildSystemPrompt() string {
	return `You are a professional product copywriting expert for the TEMU platform. Your task is to rewrite Amazon product information into titles, descriptions, and bullet points suitable for the TEMU platform.

【CRITICAL LANGUAGE REQUIREMENT】
🚨 ALL OUTPUT MUST BE IN ENGLISH ONLY
🚨 DO NOT use Chinese, Japanese, Korean, or any other non-English characters
🚨 Use only English letters (A-Z, a-z), numbers (0-9), and basic punctuation

【Core Principles】
1. Maintain accuracy and authenticity of product information
2. Use attractive language without exaggeration
3. Highlight core selling points and advantages
4. Comply with TEMU platform content standards
5. Use concise, clear, and easy-to-understand expressions

【Title Requirements】
- Length: 20-200 characters
- Include core keywords and product type
- Highlight main features or selling points
- Avoid special symbols and decorative characters
- Do not include brand names (will be removed in post-processing)

【Description Requirements】
- Length: 200-2000 characters
- Clear structure with reasonable paragraphs
- Include product features, uses, and advantages
- Use specific descriptions rather than empty adjectives
- Avoid HTML tags and rich text formatting

【Bullet Points Requirements】
- Quantity: 3-6 points
- Each point: 15-120 characters
- Highlight different product features
- Sort by importance
- Avoid duplicate content

【Product Positioning】
✅ Focus on:
- Product practicality and functionality
- Application scenarios and uses
- Materials and craftsmanship
- Value for money and quality
- User experience

⚠️ Important Constraints:
- Do NOT add children-related descriptions
- Do NOT mention "for children", "kids", "baby", "toddler", etc.
- Focus on adult or general use scenarios
- Use professional and mature expressions
- 🚨 CRITICAL: Do NOT use any environmentally-friendly related terms
- FORBIDDEN TERMS: "environmentally friendly", "eco-friendly", "sustainable", "biodegradable", "recyclable", "organic", "green product", "earth friendly", "planet friendly", "carbon neutral", "zero waste", "eco-conscious", "environmental protection", etc.
- 🚨 CRITICAL: Do NOT include any certification or compliance claims
- FORBIDDEN CERTIFICATION TERMS: "FSC certified", "FSC-certified", "FSC", "CE certified", "FDA approved", "ISO certified", "RoHS compliant", "REACH compliant", "UL listed", "ETL certified", "GS certified", "TÜV certified", "OEKO-TEX", "GOTS certified", "Fair Trade certified", "Energy Star", "USDA certified", "Non-GMO verified", "Kosher certified", "Halal certified", "BPA-free certified", "Phthalate-free certified", or any other certification/compliance statements
- Remove all certificate numbers, compliance codes, and regulatory approval references
- Focus on product quality, functionality, and practical features instead

【Output Format】
Return JSON format (IN ENGLISH ONLY):
{
  "title": "Rewritten title in English",
  "description": "Rewritten description in English",
  "bullet_points": ["Point 1 in English", "Point 2 in English", "Point 3 in English", ...]
}

【Quality Standards】
✅ Accurate information, no false advertising
✅ Fluent language, easy to understand
✅ Highlight selling points, attract users
✅ Comply with platform standards
✅ Appropriate length, reasonable structure
✅ No children-related descriptions
✅ ENGLISH ONLY - No Chinese or other languages`
}

// buildUserPrompt 构建用户提示词
func (r *AIContentRewriter) buildUserPrompt(temuCtx *temucontext.TemuTaskContext) string {
	// 直接从强类型上下文获取Amazon产品信息
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		r.logger.Warn("Amazon产品数据为空")
		return ""
	}

	title := amazonProduct.Title
	brand := amazonProduct.Brand

	prompt := fmt.Sprintf(`【原始产品信息】

标题: %s
品牌: %s
描述: %s

特性:
`, title, brand, "基于Amazon产品信息生成")

	// 使用标题作为特性
	prompt += fmt.Sprintf("1. %s\n", title)

	if amazonProduct.ProductDimensions != "" {
		prompt += fmt.Sprintf("\n尺寸: %s", amazonProduct.ProductDimensions)
	}
	if amazonProduct.ItemWeight != "" {
		prompt += fmt.Sprintf("\n重量: %s", amazonProduct.ItemWeight)
	}
	if amazonProduct.ModelNumber != "" {
		prompt += fmt.Sprintf("\n型号: %s", amazonProduct.ModelNumber)
	}
	if amazonProduct.Department != "" {
		prompt += fmt.Sprintf("\n部门: %s", amazonProduct.Department)
	}

	if len(amazonProduct.ProductDetails) > 0 {
		prompt += "\n\n产品详情:\n"
		for _, detail := range amazonProduct.ProductDetails {
			prompt += fmt.Sprintf("- %s: %s\n", detail.Type, detail.Value)
		}
	}

	if len(amazonProduct.Categories) > 0 {
		prompt += fmt.Sprintf("\n类别: %v", amazonProduct.Categories)
	}

	prompt += `

【Task】
Based on the above information, rewrite the title, description, and bullet points suitable for the TEMU platform.

⚠️ CRITICAL REQUIREMENTS:
- 🚨 OUTPUT MUST BE IN ENGLISH ONLY - No Chinese characters allowed
- Do NOT add any children-related words in title, description, or bullet points
- Even if the original product mentions children, rewrite for general or adult use scenarios
- 🚨 REMOVE ALL CERTIFICATION CLAIMS - Do NOT include FSC, CE, FDA, ISO, or any other certifications
- Remove any certificate numbers, compliance codes, or regulatory approval statements
- If the original mentions certifications, focus on the actual product features instead
- Focus on product functionality, practicality, and quality
- Use only English letters, numbers, and basic punctuation

Ensure the content is accurate, attractive, and complies with platform standards.

REMEMBER: Your entire response must be in English!`

	return prompt
}

// callAIForRewrite 调用AI进行重构
func (r *AIContentRewriter) callAIForRewrite(ctx context.Context, systemPrompt, userPrompt string) (*RewriteResult, error) {
	r.logger.Info("调用AI进行内容重构")

	// 创建请求
	seed := 42
	temperature := float32(0.7) // 使用较高的temperature以获得更有创意的输出

	req := &openaiClient.ChatCompletionRequest{
		Model: r.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}

	// 调用API
	response, err := r.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用AI服务失败: %w", err)
	}

	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("AI响应为空")
	}

	// 解析响应
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	content = r.cleanJSONContent(content)

	var result RewriteResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		r.logger.WithError(err).Errorf("解析AI响应失败: %s", content)
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	r.logger.Infof("AI重构成功 - 标题长度: %d, 描述长度: %d, 要点数量: %d",
		len(result.Title), len(result.Description), len(result.BulletPoints))

	return &result, nil
}

// cleanJSONContent 清理JSON内容
func (r *AIContentRewriter) cleanJSONContent(content string) string {
	// 移除markdown代码块标记
	if after, found := strings.CutPrefix(content, "```json"); found {
		content = strings.TrimSuffix(after, "```")
	} else if after, found := strings.CutPrefix(content, "```"); found {
		content = strings.TrimSuffix(after, "```")
	}

	return strings.TrimSpace(content)
}

// applyRewriteResult 应用重构结果
func (r *AIContentRewriter) applyRewriteResult(temuCtx *temucontext.TemuTaskContext, result *RewriteResult) {
	if result == nil {
		r.logger.Warn("⚠️ AI重构结果为nil，无法应用")
		return
	}

	r.logger.Infof("📝 AI重构结果: 标题长度=%d, 描述长度=%d, 要点数量=%d",
		len(result.Title), len(result.Description), len(result.BulletPoints))

	// 更新标题
	if result.Title != "" {
		temuCtx.TemuProduct.GoodsBasic.GoodsName = result.Title
	} else {
		r.logger.Warnf("⚠️ AI返回的标题为空，保持原标题: %s", temuCtx.TemuProduct.GoodsBasic.GoodsName)
	}

	// 更新描述
	if result.Description != "" {
		temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc = result.Description
	} else {
		r.logger.Warnf("⚠️ AI返回的描述为空，保持原描述长度: %d",
			len(temuCtx.TemuProduct.GoodsExtensionInfo.GoodsDesc))
	}

	// 更新要点
	if len(result.BulletPoints) > 0 {
		originalCount := len(temuCtx.TemuProduct.GoodsExtensionInfo.BulletPoints)
		temuCtx.TemuProduct.GoodsExtensionInfo.BulletPoints = result.BulletPoints
		r.logger.Infof("✅ 要点已更新 (原始数量: %d, 重构数量: %d)",
			originalCount, len(result.BulletPoints))
	} else {
		r.logger.Warnf("⚠️ AI返回的要点为空，保持原要点数量: %d",
			len(temuCtx.TemuProduct.GoodsExtensionInfo.BulletPoints))
	}

}
