// Package ai 提供TEMU平台的各种处理器，包括AI内容重构等功能
package ai

import (
	"context"
	"fmt"
	"strings"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pipeline"
	"task-processor/internal/pkg/timeout"
	"task-processor/internal/pkg/jsonx"
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
	aiCtx, cancel := timeout.WithAITimeout(temuCtx.GetContext())
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
	return `你是TEMU平台的专业产品文案专家。你的任务是将Amazon产品信息重写为适合TEMU平台的标题、描述和要点。

【关键语言要求】
🚨 所有输出内容必须是纯英文
🚨 不要使用中文、日文、韩文或任何其他非英文字符
🚨 只使用英文字母(A-Z, a-z)、数字(0-9)和基本标点符号

【核心原则】
1. 保持产品信息的准确性和真实性
2. 使用吸引人的语言，但不夸大
3. 突出核心卖点和优势
4. 符合TEMU平台内容标准
5. 使用简洁、清晰、易懂的表达

【标题要求】
- 长度：20-200个字符
- 包含核心关键词和产品类型
- 突出主要特性或卖点
- 避免特殊符号和装饰性字符
- 🚨 必须移除所有品牌名称和商标词
- 不要包含任何品牌标识、商标符号或专有名称
- 专注于产品功能和通用描述

【描述要求】
- 长度：200-2000个字符
- 结构清晰，段落合理
- 包含产品特性、用途和优势
- 使用具体描述而非空洞形容词
- 避免HTML标签和富文本格式
- 🚨 完全移除所有品牌名称、商标词和专有名称
- 使用通用产品描述，避免品牌相关术语

【要点要求】
- 数量：3-6个要点
- 每个要点：15-120个字符
- 突出不同的产品特性
- 按重要性排序
- 避免重复内容

【产品定位】
✅ 重点关注：
- 产品的实用性和功能性
- 应用场景和用途
- 材料和工艺
- 性价比和质量
- 用户体验

⚠️ 重要限制：
- 🚨 完全移除所有品牌名称、商标词和专有名称
- 不要使用任何品牌标识、商标符号(™, ®, ©)或公司名称
- 将品牌特定术语替换为通用产品描述
- 专注于产品功能而非品牌身份
- 不要添加儿童相关描述
- 不要提及"for children"、"kids"、"baby"、"toddler"等
- 专注于成人或通用使用场景
- 使用专业和成熟的表达
- 🚨 关键：不要使用任何环保相关术语
- 禁用术语："environmentally friendly"、"eco-friendly"、"sustainable"、"biodegradable"、"recyclable"、"organic"、"green product"、"earth friendly"、"planet friendly"、"carbon neutral"、"zero waste"、"eco-conscious"、"environmental protection"等
- 🚨 关键：不要包含任何认证或合规声明
- 禁用认证术语："FSC certified"、"FSC-certified"、"FSC"、"CE certified"、"FDA approved"、"ISO certified"、"RoHS compliant"、"REACH compliant"、"UL listed"、"ETL certified"、"GS certified"、"TÜV certified"、"OEKO-TEX"、"GOTS certified"、"Fair Trade certified"、"Energy Star"、"USDA certified"、"Non-GMO verified"、"Kosher certified"、"Halal certified"、"BPA-free certified"、"Phthalate-free certified"或任何其他认证/合规声明
- 移除所有证书编号、合规代码和监管批准引用
- 专注于产品质量、功能性和实用特性

【输出格式】
返回JSON格式（纯英文）：
{
  "title": "重写的英文标题",
  "description": "重写的英文描述",
  "bullet_points": ["英文要点1", "英文要点2", "英文要点3", ...]
}

【质量标准】
✅ 信息准确，无虚假宣传
✅ 语言流畅，易于理解
✅ 突出卖点，吸引用户
✅ 符合平台标准
✅ 长度适当，结构合理
✅ 无儿童相关描述
✅ 纯英文输出 - 不使用中文或其他语言`
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

【任务】
基于以上信息，重写适合TEMU平台的标题、描述和要点。

⚠️ 关键要求：
- 🚨 输出必须是纯英文 - 不允许使用中文字符
- 🚨 完全移除所有品牌名称、商标词和专有名称
- 将品牌特定术语替换为通用产品描述词汇
- 移除商标符号(™, ®, ©)和公司名称
- 专注于产品功能而非品牌身份
- 不要在标题、描述或要点中添加任何儿童相关词汇
- 即使原产品提到儿童，也要重写为通用或成人使用场景
- 🚨 移除所有认证声明 - 不要包含FSC、CE、FDA、ISO或任何其他认证
- 移除任何证书编号、合规代码或监管批准声明
- 如果原文提到认证，专注于实际产品特性
- 专注于产品功能性、实用性和质量
- 只使用英文字母、数字和基本标点符号

确保内容准确、有吸引力并符合平台标准。

记住：你的整个回复必须是英文！`

	return prompt
}

// callAIForRewrite 调用AI进行重构
func (r *AIContentRewriter) callAIForRewrite(ctx context.Context, systemPrompt, userPrompt string) (*RewriteResult, error) {
	r.logger.Info("调用AI进行内容重构")

	// 创建请求
	temperature := float32(0.8) // 使用较高的temperature以获得更有创意和多样化的输出

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
		// 不设置Seed，让每次调用产生不同的随机输出
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
	if err := jsonx.UnmarshalBytes([]byte(content), &result, "解析AI响应失败"); err != nil {
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
