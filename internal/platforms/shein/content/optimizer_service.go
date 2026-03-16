// Package content 提供SHEIN平台的内容优化功能
package content

import (
	"context"
	"fmt"
	"strings"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/jsonutil"
)

// ContentOptimizer 内容优化器
type ContentOptimizer struct {
	openaiClient *openaiClient.Client
}

// NewContentOptimizer 创建新的内容优化器
func NewContentOptimizer(client *openaiClient.Client) *ContentOptimizer {
	return &ContentOptimizer{
		openaiClient: client,
	}
}

// IsAvailable 检查内容优化器是否可用
func (o *ContentOptimizer) IsAvailable() bool {
	return o != nil && o.openaiClient != nil
}

// CreateChatCompletion 创建聊天完成请求的公共方法
func (o *ContentOptimizer) CreateChatCompletion(ctx context.Context, req *openaiClient.ChatCompletionRequest) (*openaiClient.ChatCompletionResponse, error) {
	if !o.IsAvailable() {
		return nil, fmt.Errorf("OpenAI客户端未初始化")
	}
	return o.openaiClient.CreateChatCompletion(ctx, req)
}

// OptimizeTitleAndDescription 使用AI优化标题和描述
func (o *ContentOptimizer) OptimizeTitleAndDescription(ctx context.Context, title, description, features string) (optimizedTitle, optimizedDescription string, err error) {
	if o.openaiClient == nil {
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
		Model:       o.openaiClient.GetDefaultModel(),
		Messages:    messages,
		Temperature: &temperature,
	}

	// 调用OpenAI API
	resp, err := o.openaiClient.CreateChatCompletion(ctx, req)
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
	optimizedTitle, optimizedDescription = o.parseOptimizedContent(content, title, description)

	return optimizedTitle, optimizedDescription, nil
}

// parseOptimizedContent 解析优化后的内容
func (o *ContentOptimizer) parseOptimizedContent(content, defaultTitle, defaultDescription string) (title, description string) {
	title = defaultTitle
	description = defaultDescription

	// 尝试解析JSON格式的响应
	type OptimizedContent struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	var optimized OptimizedContent

	// 清理内容，移除可能的markdown代码块标记
	cleanContent := strings.TrimSpace(content)
	cleanContent = strings.TrimPrefix(cleanContent, "```json")
	cleanContent = strings.TrimPrefix(cleanContent, "```")
	cleanContent = strings.TrimSuffix(cleanContent, "```")
	cleanContent = strings.TrimSpace(cleanContent)

	// 尝试解析JSON
	if err := jsonutil.UnmarshalString(cleanContent, &optimized, ""); err == nil {
		if strings.TrimSpace(optimized.Title) != "" {
			title = strings.TrimSpace(optimized.Title)
		}
		if strings.TrimSpace(optimized.Description) != "" {
			description = strings.TrimSpace(optimized.Description)
		}
		return title, description
	}

	// 如果JSON解析失败，尝试查找中文标记（向后兼容）
	titlePrefix := "优化标题："
	descPrefix := "优化描述："

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, titlePrefix); ok {
			title = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, descPrefix); ok {
			description = strings.TrimSpace(after)
		}
	}

	return title, description
}

// ValidateTitle 验证标题长度和内容
func (o *ContentOptimizer) ValidateTitle(title string) error {
	const minTitleLength = 10
	const maxTitleLength = 800

	if len(title) < minTitleLength {
		return fmt.Errorf("标题长度不能少于%d个字符", minTitleLength)
	}

	if len(title) > maxTitleLength {
		return fmt.Errorf("标题长度不能超过%d个字符", maxTitleLength)
	}

	return nil
}

// ValidateDescription 验证描述长度和内容
func (o *ContentOptimizer) ValidateDescription(description string) error {
	const minDescriptionLength = 50
	const maxDescriptionLength = 5000

	if len(description) < minDescriptionLength {
		return fmt.Errorf("描述长度不能少于%d个字符", minDescriptionLength)
	}

	if len(description) > maxDescriptionLength {
		return fmt.Errorf("描述长度不能超过%d个字符", maxDescriptionLength)
	}

	return nil
}

// TruncateTitle 截断标题到指定长度
func (o *ContentOptimizer) TruncateTitle(title string, maxLength int) string {
	if len(title) <= maxLength {
		return title
	}

	// 在单词边界处截断
	truncated := title[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > 0 && lastSpace > maxLength-50 {
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated)
}

// TruncateDescription 截断描述到指定长度
func (o *ContentOptimizer) TruncateDescription(description string, maxLength int) string {
	if len(description) <= maxLength {
		return description
	}

	// 截断描述到指定长度
	truncated := description[:maxLength]

	// 尝试在最后一个完整句子处截断（查找最后一个句号、问号或感叹号）
	lastPeriod := strings.LastIndexAny(truncated, ".!?")
	if lastPeriod > 0 && lastPeriod > maxLength-200 {
		// 如果找到句号且位置合理（不会损失太多内容），在句号处截断
		truncated = truncated[:lastPeriod+1]
	}

	return strings.TrimSpace(truncated)
}
