package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"task-processor/internal/pkg/jsonx"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// LLMAttributeMapper 基于LLM的属性映射器
type LLMAttributeMapper struct {
	llmClient LLMClient
	logger    *logrus.Entry
}

// LLMClient LLM客户端接口
type LLMClient interface {
	Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// ChatResponse LLM响应
type ChatResponse struct {
	Content string `json:"content"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// AttributeMappingRequest 属性映射请求
type AttributeMappingRequest struct {
	SourcePlatform string         `json:"source_platform"`
	TargetPlatform string         `json:"target_platform"`
	ProductData    map[string]any `json:"product_data"`
	ProductType    string         `json:"product_type"`
}

// AttributeMappingResponse 属性映射响应
type AttributeMappingResponse struct {
	MappedAttributes map[string]any `json:"mapped_attributes"`
	ProductType      string         `json:"product_type"`
	Confidence       float64        `json:"confidence"`
	Reasoning        string         `json:"reasoning"`
}

// NewLLMAttributeMapper 创建LLM属性映射器
func NewLLMAttributeMapper(llmClient LLMClient) *LLMAttributeMapper {
	return &LLMAttributeMapper{
		llmClient: llmClient,
		logger:    logger.GetGlobalLogger("LLMAttributeMapper"),
	}
}

// MapAttributes 使用LLM映射产品属性
func (m *LLMAttributeMapper) MapAttributes(ctx context.Context, req *AttributeMappingRequest) (*AttributeMappingResponse, error) {
	m.logger.WithFields(logrus.Fields{
		"source_platform": req.SourcePlatform,
		"target_platform": req.TargetPlatform,
		"product_type":    req.ProductType,
	}).Info("开始LLM属性映射")

	// 构建LLM提示词
	prompt := m.buildMappingPrompt(req)

	// 调用LLM
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: m.getSystemPrompt(),
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := m.llmClient.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}

	// 解析LLM响应
	mappingResult, err := m.parseLLMResponse(response.Content)
	if err != nil {
		return nil, fmt.Errorf("解析LLM响应失败: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"mapped_count": len(mappingResult.MappedAttributes),
		"confidence":   mappingResult.Confidence,
		"tokens_used":  response.Usage.TotalTokens,
	}).Info("LLM属性映射完成")

	return mappingResult, nil
}

// getSystemPrompt 获取系统提示词
func (m *LLMAttributeMapper) getSystemPrompt() string {
	return `你是一个专业的跨境电商产品属性映射专家。你的任务是将1688平台的中文产品信息智能转换为Amazon平台的英文标准属性。

核心要求：
1. 准确翻译产品信息，保持商业价值
2. 遵循Amazon平台的属性规范
3. 移除营销性词汇，使用标准描述
4. 智能提取变体信息（颜色、尺寸等）
5. 确保属性完整性和合规性

输出格式必须是有效的JSON，包含：
- mapped_attributes: 映射后的属性对象
- product_type: 推荐的Amazon产品类型
- confidence: 映射置信度(0-1)
- reasoning: 映射推理过程

请始终以专业、准确、高效的方式完成映射任务。`
}

// buildMappingPrompt 构建映射提示词
func (m *LLMAttributeMapper) buildMappingPrompt(req *AttributeMappingRequest) string {
	// 将产品数据转换为JSON字符串
	productDataJSON, _ := json.MarshalIndent(req.ProductData, "", "  ")

	prompt := fmt.Sprintf(`请将以下1688产品信息映射为Amazon平台标准属性：

源平台：%s
目标平台：%s
产品类型：%s

原始产品数据：
%s

映射要求：
1. item_name: 产品标题（英文，去除营销词汇）
2. brand: 品牌名称
3. manufacturer: 制造商
4. product_description: 产品描述（英文，专业化）
5. color_name: 颜色（如果有变体）
6. size_name: 尺寸（如果有变体）
7. material_type: 材质信息
8. target_audience: 目标人群
9. style: 风格类型
10. 其他相关属性

请分析产品信息并返回标准的JSON格式映射结果。`,
		req.SourcePlatform,
		req.TargetPlatform,
		req.ProductType,
		string(productDataJSON))

	return prompt
}

// parseLLMResponse 解析LLM响应
func (m *LLMAttributeMapper) parseLLMResponse(content string) (*AttributeMappingResponse, error) {
	// 清理响应内容，提取JSON部分
	content = strings.TrimSpace(content)

	// 查找JSON开始和结束位置
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("响应中未找到有效的JSON格式")
	}

	jsonContent := content[start : end+1]

	var result AttributeMappingResponse
	if err := jsonx.UnmarshalString(jsonContent, &result, "JSON解析失败"); err != nil {
		return nil, err
	}

	// 验证必要字段
	if result.MappedAttributes == nil {
		return nil, fmt.Errorf("映射结果缺少mapped_attributes字段")
	}

	// 设置默认值
	if result.Confidence == 0 {
		result.Confidence = 0.8 // 默认置信度
	}
	if result.ProductType == "" {
		result.ProductType = "PRODUCT" // 默认产品类型
	}

	return &result, nil
}

// ValidateMapping 验证映射结果
func (m *LLMAttributeMapper) ValidateMapping(attributes map[string]any) error {
	// 检查必需属性
	requiredFields := []string{"item_name"}

	for _, field := range requiredFields {
		if _, exists := attributes[field]; !exists {
			return fmt.Errorf("缺少必需属性: %s", field)
		}
	}

	// 验证属性值格式
	if itemName, ok := attributes["item_name"].(string); ok {
		if len(itemName) == 0 {
			return fmt.Errorf("item_name不能为空")
		}
		if len(itemName) > 200 {
			return fmt.Errorf("item_name长度不能超过200字符")
		}
	}

	return nil
}

// GetMappingStats 获取映射统计信息
func (m *LLMAttributeMapper) GetMappingStats() map[string]any {
	return map[string]any{
		"service_name":    "LLMAttributeMapper",
		"version":         "1.0.0",
		"supported_pairs": []string{"1688->Amazon", "Taobao->Amazon"},
	}
}
