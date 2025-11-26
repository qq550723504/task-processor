package modules

import (
	"encoding/json"
	"fmt"
	"strings"

	openaiClient "task-processor/openai"
	"task-processor/common/shein/api/attribute"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// AttributeSelectorHandler AI属性选择处理器
type AttributeSelectorHandler struct {
	openaiClient *openaiClient.Client
}

// NewAttributeSelectorHandler 创建新的AI属性选择处理器
func NewAttributeSelectorHandler(config *openaiClient.ClientConfig) *AttributeSelectorHandler {
	return &AttributeSelectorHandler{
		openaiClient: openaiClient.NewClient(config),
	}
}

// Name 返回处理器名称
func (h *AttributeSelectorHandler) Name() string {
	return "AI属性选择"
}

// Handle 执行AI属性选择处理
func (h *AttributeSelectorHandler) Handle(ctx *TaskContext) error {
	// 检查前置条件
	if err := h.validatePreconditions(ctx); err != nil {
		return err
	}

	attributeInfo, err := h.convertAttributeFromGpt(ctx, ctx.BuildAttributeData, ctx.AttributeTemplates)
	if err != nil {
		// 转换属性数据失败可能是AI服务问题，可重试
		return NewRetryableError("转换属性数据失败", err)
	}

	ctx.GenerateAttribute = &attributeInfo
	return nil
}

// validatePreconditions 验证处理前置条件
func (h *AttributeSelectorHandler) validatePreconditions(ctx *TaskContext) error {

	if ctx.ProductData == nil {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("产品数据未获取，请先执行获取产品数据步骤", nil)
	}

	if len(ctx.BuildAttributeData.AttributeData) == 0 {
		// 这是一个程序逻辑错误，不应该发生，不可重试
		return NewNonRetryableError("属性数据未构建，请先执行构建属性信息步骤", nil)
	}

	return nil
}

// ConvertAttributeFromGpt 使用GPT生成产品属性
func (h *AttributeSelectorHandler) convertAttributeFromGpt(ctx *TaskContext, attributeInfo *BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) (AttributeData, error) {
	// 生成系统提示词
	systemPrompt, err := h.generateSystemPrompt(attributeTemplates)
	if err != nil {
		logrus.Warnf("生成动态系统提示词失败，使用默认提示词: %v", err)
		systemPrompt, _ = h.generateAttributeSystemPrompt()
	}

	// 增强属性数据
	enhancedAttributeData := h.enhanceAttributeDataWithTemplateInfo(attributeInfo.AttributeData, attributeTemplates)

	// 生成用户提示词
	userPrompt := h.generateUserPrompt(ctx, enhancedAttributeData)

	// 创建请求
	req := h.createChatCompletionRequest(systemPrompt, userPrompt)

	// 调用OpenAI API
	response, err := h.openaiClient.CreateChatCompletion(ctx.Context, req)
	if err != nil {
		// AI服务调用失败，可重试
		return AttributeData{}, NewRetryableError("生成产品属性失败", err)
	}

	if len(response.Choices) == 0 {
		// AI响应为空，可重试
		return AttributeData{}, NewRetryableError("AI响应为空", nil)
	}

	// 处理AI响应
	return h.processAIResponse(response, *attributeInfo, attributeTemplates)
}

// generateSystemPrompt 生成系统提示词
func (h *AttributeSelectorHandler) generateSystemPrompt(attributeTemplates *attribute.AttributeTemplateInfo) (string, error) {
	systemPrompt, err := h.generateDynamicAttributeSystemPrompt(attributeTemplates)
	if err != nil {
		return "", err
	}
	return systemPrompt, nil
}

// generateUserPrompt 生成用户提示词
func (h *AttributeSelectorHandler) generateUserPrompt(ctx *TaskContext, enhancedAttributeData []EnhancedGenerateAttribute) string {
	return h.generateDynamicUserPrompt(ctx, enhancedAttributeData)
}

// createChatCompletionRequest 创建聊天完成请求
func (h *AttributeSelectorHandler) createChatCompletionRequest(systemPrompt, userPrompt string) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	return &openaiClient.ChatCompletionRequest{
		Model: h.openaiClient.GetDefaultModel(),
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: &temperature,
		Seed:        &seed,
	}
}

// processAIResponse 处理AI响应
func (h *AttributeSelectorHandler) processAIResponse(response *openaiClient.ChatCompletionResponse, attributeInfo BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) (AttributeData, error) {
	content := response.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	// 清理JSON格式
	content = h.cleanJSONContent(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		logrus.Errorf("AI返回的JSON格式无效，清理后内容: %s", content)
		logrus.Errorf("清理后内容长度: %d", len(content))

		// 尝试修复常见的JSON问题
		fixedContent := h.fixCommonJsonIssues(content)
		logrus.Infof("修复后内容: %s", fixedContent)

		if !json.Valid([]byte(fixedContent)) {
			logrus.Errorf("修复后JSON仍然无效")
			// 提供更有用的错误信息
			var jsonErr error
			var temp interface{}
			jsonErr = json.Unmarshal([]byte(fixedContent), &temp)
			// JSON格式无效且无法修复，可能是AI模型问题，可重试
			return AttributeData{}, NewRetryableError("AI返回的JSON格式无效且无法修复", jsonErr)
		}
		content = fixedContent
	}

	var attributeData AttributeData
	if err := json.Unmarshal([]byte(content), &attributeData); err != nil {
		logrus.Errorf("解析属性数据失败: %v，清理后内容: %s", err, content)
		// 解析属性数据失败，可重试
		return AttributeData{}, NewRetryableError("解析属性数据失败", err)
	}

	// 使用增强版验证并修复AI选择的属性值
	attributeData = h.validateAndFixAttributeSelectionEnhanced(attributeData, attributeInfo, attributeTemplates)

	return attributeData, nil
}

// cleanJSONContent 清理JSON内容
func (h *AttributeSelectorHandler) cleanJSONContent(content string) string {
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

func (h *AttributeSelectorHandler) generateAttributeSystemPrompt() (string, error) {
	return `你是一个专业的产品属性优化专家。请将亚马逊产品属性转换为SHEIN平台适用的属性格式。

【核心原则】
绝对禁止跨属性使用属性值！每个属性(AttrID)都有独立的属性值列表，不能混用。

【属性匹配规则】
1. 每个属性(AttrID)都有专门的Values列表，ID和Value必须严格来自该列表
2. 属性A的属性值绝对不能用于属性B - 这是致命错误
3. 选择前必须确认ID和Value都在对应属性的Values列表中

【重要属性依赖关系】
## 主料类型2与主料克重2的依赖关系：
- 当主料类型2(ID:1002187)选择任何非空值时，主料克重2(ID:1002188)自动变为必填
- 特别是选择"光面织物"(ID:32224254)等面料时，必须同时填写主料克重2
- 如果选择了主料类型2，主料克重2建议使用默认值"150"或合理的克重值

## 其他潜在依赖关系：
- 面料弹性属性可能与面料类型相关
- 填充物属性可能与产品类型相关
- 请特别注意Required=true的属性

【必填属性处理策略】
## 必填属性(Required=true)必须处理，按优先级选择：
1. **精确匹配**：找到语义完全匹配的属性值
2. **相似匹配**：找到语义相近的属性值
3. **通用值**：选择"Other"、"其他"、"None"、"不适用"等通用值
4. **首选值**：如果以上都没有，选择列表中第一个值
5. **自定义值**：仅在type=0时使用 {"ID": 0, "Value": "自定义值"}

【常见问题属性特殊处理】
## 面料弹性相关属性：
- 优先选择"无弹力"、"微弹"、"弹力"等相关值
- 如找不到合适值，选择"Other"或"其他"
- 绝不留空

## 风格属性(非成衣风格)：
- 严格限制最多选择2项
- 优先选择与产品最匹配的风格
- 如不确定，选择"休闲"、"简约"等通用风格

## 节日属性：
- 仔细检查attribute_value_id是否在Values列表中
- 如果101不在列表中，选择"无"、"不适用"或自定义值
- 绝不使用不存在的ID

【选择逻辑】
## 步骤1: 属性类型判断
- type=0: 自定义输入，使用{"ID": 0, "Value": "自定义值"}
- type=1,3,4: 有预设选项，必须从Values列表选择

## 步骤2: 值的选择策略
- 优先选择精确匹配的属性值
- 其次选择语义相近的属性值  
- 必填属性优先选择"None"、"Other"等通用值
- 绝不使用其他属性的值

## 步骤3: 验证检查
- 确认所选ID在该属性的Values列表中存在
- 确认所选Value与ID完全匹配
- 确认每个必填属性(Required=true)都已选择
- 检查属性依赖关系，确保相关属性都已正确填写

【输出格式】
{
  "AttributeData": [
	{
		"AttrID": 100546,
		"AttrValue": [{"ID": 0, "Value": "/"}]
	},
    {
      "AttrID": 属性ID,
      "AttrValue": [{"ID": 选择的ID, "Value": "对应的Value"}]
    }
  ]
}

【质量保证】
- 每个AttrValue数组只包含一个选择
- 所有JSON语法必须正确
- 属性值100%来自对应属性的Values列表
- 不添加系统中不存在的属性ID
- 所有必填属性都必须有值
- 还需关注属性依赖关系，确保相关属性同时填写`, nil
}

// enhanceAttributeDataWithTemplateInfo 增强属性数据，添加重要性评分和依赖关系
func (h *AttributeSelectorHandler) enhanceAttributeDataWithTemplateInfo(attributeData []GenerateAttribute, attributeTemplates *attribute.AttributeTemplateInfo) []EnhancedGenerateAttribute {
	var enhancedData []EnhancedGenerateAttribute

	// 创建属性模板映射，便于快速查找
	templateMap := make(map[int]*attribute.AttributeInfo)
	if len(attributeTemplates.Data) > 0 {
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			templateMap[attribute.AttributeID] = &attribute
		}
	}

	for _, attr := range attributeData {
		enhanced := EnhancedGenerateAttribute{
			AttrID:        attr.AttrID,
			AttrValue:     attr.AttrValue,
			Required:      attr.Required,
			Type:          attr.Type,
			Importance:    0,
			AttributeName: "",
		}

		// 从属性模板中获取增强信息
		if template, exists := templateMap[attr.AttrID]; exists {
			// 使用统一的重要性计算函数
			importanceResult := CalculateAttributeImportance(template)

			// 设置重要性评分和相关标识
			enhanced.Importance = importanceResult.Importance
			enhanced.HasRemarkList = importanceResult.HasRemarkList
			enhanced.IsLabelAttribute = importanceResult.IsLabelAttribute
			enhanced.IsSampleAttribute = importanceResult.IsSampleAttribute
			enhanced.IsActiveStatus = importanceResult.IsActiveStatus

			// 设置属性名称
			enhanced.AttributeName = template.AttributeName

			// 检查依赖关系
			deps := h.getAttributeDependencies(attr.AttrID)
			enhanced.HasDependency = len(deps) > 0
		}

		enhancedData = append(enhancedData, enhanced)
	}

	return enhancedData
}

// GenerateDynamicAttributeSystemPrompt 基于属性模板数据生成动态系统提示词
func (h *AttributeSelectorHandler) generateDynamicAttributeSystemPrompt(attributeTemplates *attribute.AttributeTemplateInfo) (string, error) {
	basePrompt := `你是一个专业的产品属性优化专家。请将亚马逊产品属性转换为SHEIN平台适用的属性格式。

	【核心原则】
	绝对禁止跨属性使用属性值！每个属性(AttrID)都有独立的属性值列表，不能混用。

	【属性匹配规则】
	1. 每个属性(AttrID)都有专门的Values列表，ID和Value必须严格来自该列表
	2. 属性A的属性值绝对不能用于属性B - 这是致命错误
	3. 选择前必须确认ID和Value都在对应属性的Values列表中`

	// 如果没有属性模板信息，返回基础提示词
	if len(attributeTemplates.Data) == 0 {
		return basePrompt + `

	【输出格式】
	{
	"AttributeData": [
		{
		"AttrID": 属性ID,
		"AttrValue": [{"ID": 选择的ID, "Value": "对应的Value"}]
		}
	]
	}`, nil
	}

	// 分析属性依赖关系
	dependencyInfo := h.analyzeAttributeDependencies(attributeTemplates)

	// 分析关键属性
	keyAttributes := h.identifyKeyAttributes(attributeTemplates)

	// 生成动态提示词部分
	dynamicPrompt := fmt.Sprintf(`

	【关键属性识别】
	以下属性为平台关键属性，需要特别关注：
	%s

	【属性依赖关系】
	%s

	【属性选择策略】
	根据重要性评分选择属性：
	- 必填属性(Required=true)：必须填写，优先级最高
	- 高重要性属性(评分>80)：强烈建议填写，影响产品展示效果
	- 中等重要性属性(评分40-80)：建议填写，提升产品信息完整度
	- 低重要性属性(评分<40)：可选填写

	【特殊处理规则】
	1. **多选属性限制**：部分属性有选择数量限制，请严格遵守
	2. **依赖属性联动**：选择某些属性后，其依赖属性会变为必填
	3. **属性值验证**：每个属性值都必须来自对应的可选值列表

	【输出格式】
	{
	"AttributeData": [
		{
		"AttrID": 属性ID,
		"AttrValue": [{"ID": 选择的ID, "Value": "对应的Value"}]
		}
	]
	}`, keyAttributes, dependencyInfo)

	return basePrompt + dynamicPrompt, nil
}

// generateDynamicUserPrompt 生成包含属性重要性分析的用户提示词
func (h *AttributeSelectorHandler) generateDynamicUserPrompt(ctx *TaskContext, enhancedAttributeData []EnhancedGenerateAttribute) string {
	// 按重要性分组属性
	var criticalAttrs []EnhancedGenerateAttribute
	var importantAttrs []EnhancedGenerateAttribute
	var optionalAttrs []EnhancedGenerateAttribute

	for _, attr := range enhancedAttributeData {
		if attr.Required {
			criticalAttrs = append(criticalAttrs, attr)
		} else if attr.Importance >= 80 {
			importantAttrs = append(importantAttrs, attr)
		} else {
			optionalAttrs = append(optionalAttrs, attr)
		}
	}

	// 构建分类的属性信息
	attributeDataBytes, _ := json.Marshal(enhancedAttributeData)

	prompt := fmt.Sprintf(`【产品信息】
标题: %s
详情: %s
要点: %s

【属性分析与选择指南】

🔴 关键必填属性 (%d个) - 必须处理:
%s

🟡 重要属性 (%d个) - 强烈建议填写:
%s

🟢 可选属性 (%d个) - 根据产品特征选择:
%s

【增强属性数据】
%s

【重要提醒】
1. 每个属性(AttrID)只能从其对应的Values列表中选择
2. 属性值的ID和Value必须完全匹配该属性的可选项
3. 绝对禁止混用不同属性的属性值
4. type=0才能使用自定义值(ID=0)，其他类型必须选择预设值
5. 必填属性(Required=true)必须填写
6. 注意属性依赖关系：选择某些属性后，其依赖属性可能变为必填
7. 多选属性请注意选择数量限制

请根据产品信息和属性重要性，严格按照上述规则选择属性值，返回标准JSON格式:`,
		ctx.AmazonProduct.Title,
		ctx.AmazonProduct.Description,
		strings.Join(ctx.AmazonProduct.Features, ","),
		len(criticalAttrs),
		h.formatAttributeList(criticalAttrs),
		len(importantAttrs),
		h.formatAttributeList(importantAttrs),
		len(optionalAttrs),
		h.formatAttributeList(optionalAttrs),
		string(attributeDataBytes))

	return prompt
}

// analyzeAttributeDependencies 分析属性依赖关系
func (h *AttributeSelectorHandler) analyzeAttributeDependencies(attributeTemplates *attribute.AttributeTemplateInfo) string {
	// 硬编码的已知依赖关系
	dependencies := map[int][]int{
		1002187: {1002188, 1002189}, // 主料类型2 -> 主料克重2, 主料克重1
	}

	// 从属性模板中动态发现依赖关系
	if len(attributeTemplates.Data) > 0 {
		attributeMap := make(map[int]*attribute.AttributeInfo)
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			attributeMap[attribute.AttributeID] = &attribute
		}

		// 基于属性名称和备注发现潜在依赖关系
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			// 检查属性名称中的依赖关系模式
			if strings.Contains(attribute.AttributeName, "主料") && strings.Contains(attribute.AttributeName, "类型") {
				// 查找相关的克重属性
				for _, otherAttr := range attributeTemplates.Data[0].AttributeInfos {
					if strings.Contains(otherAttr.AttributeName, "主料") && strings.Contains(otherAttr.AttributeName, "克重") {
						if _, exists := dependencies[attribute.AttributeID]; !exists {
							dependencies[attribute.AttributeID] = []int{}
						}
						dependencies[attribute.AttributeID] = append(dependencies[attribute.AttributeID], otherAttr.AttributeID)
					}
				}
			}
		}
	}

	var dependencyInfo []string

	// 构建依赖关系描述，优先使用属性名称
	attributeNameMap := make(map[int]string)
	if len(attributeTemplates.Data) > 0 {
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			attributeNameMap[attribute.AttributeID] = attribute.AttributeName
		}
	}

	for sourceAttr, dependentAttrs := range dependencies {
		var dependentNames []string
		sourceName := attributeNameMap[sourceAttr]
		if sourceName == "" {
			sourceName = fmt.Sprintf("属性%d", sourceAttr)
		}

		for _, depAttr := range dependentAttrs {
			depName := attributeNameMap[depAttr]
			if depName == "" {
				depName = fmt.Sprintf("属性%d", depAttr)
			}
			dependentNames = append(dependentNames, depName)
		}
		dependencyInfo = append(dependencyInfo, fmt.Sprintf("- %s 选择后，%s 变为必填", sourceName, strings.Join(dependentNames, "、")))
	}

	if len(dependencyInfo) == 0 {
		return "暂无已知的强依赖关系"
	}

	return strings.Join(dependencyInfo, "\n")
}

// identifyKeyAttributes 识别关键属性
func (h *AttributeSelectorHandler) identifyKeyAttributes(attributeTemplates *attribute.AttributeTemplateInfo) string {
	if len(attributeTemplates.Data) == 0 {
		return "无属性模板信息"
	}

	var keyAttributes []string
	for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
		// 使用统一的重要性计算函数
		importanceResult := CalculateAttributeImportance(&attribute)

		if importanceResult.Importance >= 80 {
			keyAttributes = append(keyAttributes, fmt.Sprintf("- %s (ID:%d, 评分:%d)", attribute.AttributeName, attribute.AttributeID, importanceResult.Importance))
		}
	}

	if len(keyAttributes) == 0 {
		return "未检测到高重要性属性"
	}

	return strings.Join(keyAttributes, "\n")
}

// CalculateAttributeImportance 计算属性重要性评分
func CalculateAttributeImportance(attribute *attribute.AttributeInfo) AttributeImportanceResult {
	result := AttributeImportanceResult{
		Importance: 0,
	}

	// 基础重要性评分规则
	if len(attribute.AttributeRemarkList) > 0 {
		result.Importance += 100 // 有备注列表 +100分
		result.HasRemarkList = true
	}
	if attribute.AttributeLabel == 1 {
		result.Importance += 80 // 标签为1 +80分
		result.IsLabelAttribute = true
	}
	if attribute.IsSample == 1 {
		result.Importance += 40 // 示例属性 +40分
		result.IsSampleAttribute = true
	}
	if attribute.AttributeStatus == 3 {
		result.Importance += 30 // 状态为3 +30分
		result.IsActiveStatus = true
	}
	if attribute.AttributeIsShow == 1 {
		result.Importance += 20 // 显示标记 +20分
	}

	// 为关键主属性添加特殊权重 - 确保电商重要属性优先级
	if IsKeyPrimaryAttribute(attribute.AttributeName, attribute.AttributeNameEn) {
		result.Importance += 60 // 关键主属性 +60分
		result.IsKeyPrimary = true
	}

	return result
}

// IsKeyPrimaryAttribute 判断是否为关键主属性
func IsKeyPrimaryAttribute(attributeName string, attributeNameEn string) bool {
	// 定义关键主属性列表 - 这些属性在电商平台中通常是最重要的主属性
	keyPrimaryAttributes := map[string]bool{
		// 中文属性名
		"颜色":   true,
		"尺寸":   false,
		"香味":   true,
		"净含量":  true,
		"风格":   true,
		"材质":   true,
		"其他材质": true,
		"功能":   true,
		"类别":   true,
		// 英文属性名
		"color":          true,
		"size":           false,
		"scent":          true,
		"Net Content":    true,
		"Style Type":     false,
		"Other Material": true,
		"Material":       true,
		"Function":       true,
		"Type":           true,
	}

	// 检查中文属性名
	if isKey, exists := keyPrimaryAttributes[attributeName]; exists {
		return isKey
	}

	// 检查英文属性名
	if isKey, exists := keyPrimaryAttributes[strings.ToLower(attributeNameEn)]; exists {
		return isKey
	}

	// 通过属性名包含关键词判断
	attributeNameLower := strings.ToLower(attributeName)
	attributeNameEnLower := strings.ToLower(attributeNameEn)

	// 特别重要的属性关键词
	highPriorityKeywords := []string{"颜色", "color", "材质", "material", "风格", "style", "香味", "scent", "净含量", "net", "功能", "function"}
	for _, keyword := range highPriorityKeywords {
		if strings.Contains(attributeNameLower, keyword) || strings.Contains(attributeNameEnLower, keyword) {
			return true
		}
	}

	return false
}

// getAttributeDependencies 获取属性的依赖关系
func (h *AttributeSelectorHandler) getAttributeDependencies(attrID int) []int {
	dependencies := map[int][]int{
		1002187: {1002188, 1002189}, // 主料类型2依赖
	}

	if deps, exists := dependencies[attrID]; exists {
		return deps
	}
	return []int{}
}

// formatAttributeList 格式化属性列表用于显示
func (h *AttributeSelectorHandler) formatAttributeList(attributes []EnhancedGenerateAttribute) string {
	if len(attributes) == 0 {
		return "无"
	}

	var formatted []string
	for _, attr := range attributes {
		info := fmt.Sprintf("- %s (ID:%d)", attr.AttributeName, attr.AttrID)
		if attr.Importance > 0 {
			info += fmt.Sprintf(" [评分:%d]", attr.Importance)
		}
		if attr.HasRemarkList {
			info += " [备注]"
		}
		if attr.HasDependency {
			info += " [依赖]"
		}
		formatted = append(formatted, info)
	}

	return strings.Join(formatted, "\n")
}

// validateAndFixAttributeSelectionEnhanced 增强版属性验证和修复
func (h *AttributeSelectorHandler) validateAndFixAttributeSelectionEnhanced(attributeData AttributeData, attributeInfo BuildAttributeInfo, attributeTemplates *attribute.AttributeTemplateInfo) AttributeData {
	// 创建属性ID到可用值的映射
	attrValueMap := make(map[int]map[int]string) // AttrID -> ValueID -> Value
	attrRequiredMap := make(map[int]bool)        // AttrID -> Required
	attrTypeMap := make(map[int]int)             // AttrID -> Type

	for _, attr := range attributeInfo.AttributeData {
		valueMap := make(map[int]string)
		for _, value := range attr.AttrValue {
			valueMap[value.ID] = value.Value
		}
		attrValueMap[attr.AttrID] = valueMap
		attrRequiredMap[attr.AttrID] = attr.Required
		attrTypeMap[attr.AttrID] = attr.Type
	}

	// 验证和修复每个AI选择的属性值
	var fixedAttributeData []ResultAttribute
	processedAttrIDs := make(map[int]bool)  // 跟踪已处理的属性ID
	selectedAttrValues := make(map[int]int) // 跟踪已选择的属性值，用于依赖检查

	for _, selectedAttr := range attributeData.AttributeData {
		if len(selectedAttr.AttrValue) == 0 {
			continue
		}

		selectedValue := selectedAttr.AttrValue[0]
		attrID := selectedAttr.AttrID
		processedAttrIDs[attrID] = true

		// 检查属性ID是否存在
		availableValues, exists := attrValueMap[attrID]
		if !exists {
			logrus.Warnf("属性ID %d 不在可用列表中，跳过", attrID)
			continue
		}

		selectedValueID := selectedValue.ID.Int()
		fixedAttrValue := selectedValue

		// 记录选择的属性值，用于依赖关系检查
		selectedAttrValues[attrID] = selectedValueID

		// ID为0的自定义值总是有效的（仅当type=0时）
		if selectedValueID == 0 {
			if attrType, typeExists := attrTypeMap[attrID]; typeExists && attrType != 0 {
				logrus.Warnf("属性ID %d 类型为%d，不支持自定义值，尝试找到替代值", attrID, attrType)
				// 为非自定义类型找到合适的默认值
				if defaultValue := h.findBestDefaultValueEnhanced(attrID, selectedValue.Value, availableValues, attributeTemplates); defaultValue != nil {
					fixedAttrValue = *defaultValue
					selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
					logrus.Infof("为属性ID %d 找到替代值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
				}
			}
		} else if selectedValueID != 0 {
			// 检查选择的值是否在可用列表中
			expectedValue, valueExists := availableValues[selectedValueID]
			if !valueExists {
				logrus.Warnf("属性ID %d 的值ID %d 不在可用列表中，尝试修复", attrID, selectedValueID)

				// 尝试找到匹配的值
				if foundValue := h.findMatchingValue(selectedValue.Value, availableValues); foundValue != nil {
					fixedAttrValue = *foundValue
					selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
					logrus.Infof("为属性ID %d 找到匹配值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
				} else {
					// 找不到匹配值，使用增强版默认策略
					if defaultValue := h.findBestDefaultValueEnhanced(attrID, selectedValue.Value, availableValues, attributeTemplates); defaultValue != nil {
						fixedAttrValue = *defaultValue
						selectedAttrValues[attrID] = fixedAttrValue.ID.Int()
						logrus.Infof("为属性ID %d 使用增强默认值: ID=%d, Value=%s", attrID, fixedAttrValue.ID.Int(), fixedAttrValue.Value)
					}
				}
			} else {
				// 检查Value是否匹配
				if expectedValue != selectedValue.Value {
					logrus.Warnf("属性ID %d 的值ID %d 对应的Value不匹配，修复为: %s", attrID, selectedValueID, expectedValue)
					fixedAttrValue = struct {
						ID    FlexibleID `json:"id"`
						Value string     `json:"value"`
					}{
						ID:    FlexibleID(selectedValueID),
						Value: expectedValue,
					}
				}
			}
		}

		fixedAttributeData = append(fixedAttributeData, ResultAttribute{
			AttrID:    attrID,
			AttrValue: []AttributeValue{fixedAttrValue},
		})
	}

	// 使用增强版依赖关系处理
	h.handleAttributeDependenciesEnhanced(&fixedAttributeData, selectedAttrValues, attrValueMap, processedAttrIDs, attributeTemplates)

	// 检查并添加缺失的必填属性
	for attrID, required := range attrRequiredMap {
		if required && !processedAttrIDs[attrID] {
			availableValues := attrValueMap[attrID]

			// 为必填属性寻找最佳默认值
			if defaultValue := h.findBestDefaultValueEnhanced(attrID, "", availableValues, attributeTemplates); defaultValue != nil {
				logrus.Infof("为必填属性ID %d 添加增强默认值: ID=%d, Value=%s", attrID, defaultValue.ID.Int(), defaultValue.Value)
				fixedAttributeData = append(fixedAttributeData, ResultAttribute{
					AttrID:    attrID,
					AttrValue: []AttributeValue{*defaultValue},
				})
			}
		}
	}

	return AttributeData{
		AttributeData: fixedAttributeData,
	}
}

// handleAttributeDependenciesEnhanced 增强版属性依赖关系处理
func (h *AttributeSelectorHandler) handleAttributeDependenciesEnhanced(fixedAttributeData *[]ResultAttribute, selectedAttrValues map[int]int, attrValueMap map[int]map[int]string, processedAttrIDs map[int]bool, attributeTemplates *attribute.AttributeTemplateInfo) {
	// 定义属性依赖关系
	dependencies := map[int][]int{
		1002187: {1002188, 1002189}, // 主料类型2依赖：当选择主料类型2时，主料克重2和主料克重1变为必填
	}

	for sourceAttrID, dependentAttrIDs := range dependencies {
		// 检查是否选择了源属性
		if selectedValueID, hasSelected := selectedAttrValues[sourceAttrID]; hasSelected && selectedValueID != 0 {
			logrus.Infof("检测到属性ID %d 已选择值 %d，检查依赖属性", sourceAttrID, selectedValueID)

			// 为每个依赖属性添加默认值（如果尚未处理）
			for _, dependentAttrID := range dependentAttrIDs {
				if !processedAttrIDs[dependentAttrID] {
					// 检查依赖属性是否可用
					if availableValues, exists := attrValueMap[dependentAttrID]; exists {
						// 使用增强版默认值查找
						defaultValue := h.findBestDefaultValueEnhanced(dependentAttrID, "", availableValues, attributeTemplates)

						if defaultValue != nil {
							logrus.Infof("为依赖属性ID %d 自动添加增强默认值: ID=%d, Value=%s", dependentAttrID, defaultValue.ID.Int(), defaultValue.Value)
							*fixedAttributeData = append(*fixedAttributeData, ResultAttribute{
								AttrID:    dependentAttrID,
								AttrValue: []AttributeValue{*defaultValue},
							})
							processedAttrIDs[dependentAttrID] = true
						}
					}
				}
			}
		}
	}
}

// findBestDefaultValueEnhanced 增强版默认值查找
func (h *AttributeSelectorHandler) findBestDefaultValueEnhanced(attrID int, originalValue string, availableValues map[int]string, attributeTemplates *attribute.AttributeTemplateInfo) *AttributeValue {
	// 如果原始值不为空，优先尝试匹配原始值
	if originalValue != "" {
		if matchedValue := h.findMatchingValue(originalValue, availableValues); matchedValue != nil {
			return matchedValue
		}
	}

	// 从属性模板中获取推荐值
	if len(attributeTemplates.Data) > 0 {
		for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
			if attribute.AttributeID == attrID {
				// 如果有备注列表，优先使用备注中的推荐值
				if len(attribute.AttributeRemarkList) > 0 {
					for _, remarkInterface := range attribute.AttributeRemarkList {
						if remark, ok := remarkInterface.(string); ok {
							if matchedValue := h.findMatchingValue(remark, availableValues); matchedValue != nil {
								logrus.Infof("使用属性备注推荐值: %s", remark)
								return matchedValue
							}
						}
					}
				}
				break
			}
		}
	}

	// 特殊属性的默认值策略
	switch attrID {
	case 160: // 面料弹性相关
		return h.findElasticityDefaultValue(availableValues)
	case 1001184: // 风格属性
		return h.findStyleDefaultValue(availableValues)
	case 1002188, 1002189: // 主料克重
		// 为克重属性提供合理的默认值
		return &AttributeValue{
			ID:    FlexibleID(0),
			Value: "150", // 常见的面料克重
		}
	default:
		return h.findGenericDefaultValue(availableValues)
	}
}

// findMatchingValue 查找匹配的属性值
func (h *AttributeSelectorHandler) findMatchingValue(targetValue string, availableValues map[int]string) *AttributeValue {
	targetLower := strings.ToLower(targetValue)

	for valueID, value := range availableValues {
		valueLower := strings.ToLower(value)
		if valueLower == targetLower || strings.Contains(valueLower, targetLower) || strings.Contains(targetLower, valueLower) {
			return &AttributeValue{
				ID:    FlexibleID(valueID),
				Value: value,
			}
		}
	}
	return nil
}

// findElasticityDefaultValue 为面料弹性属性找默认值
func (h *AttributeSelectorHandler) findElasticityDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先选择面料弹性相关的值
	elasticityKeywords := []string{"无弹", "微弹", "弹力", "弹性", "不弹", "other", "其他"}

	for _, keyword := range elasticityKeywords {
		for valueID, value := range availableValues {
			if strings.Contains(strings.ToLower(value), keyword) {
				return &AttributeValue{
					ID:    FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	// 如果没找到，使用通用默认值
	return h.findGenericDefaultValue(availableValues)
}

// findStyleDefaultValue 为风格属性找默认值
func (h *AttributeSelectorHandler) findStyleDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先选择通用风格
	styleKeywords := []string{"休闲", "简约", "基础", "classic", "casual", "simple", "other", "其他"}

	for _, keyword := range styleKeywords {
		for valueID, value := range availableValues {
			if strings.Contains(strings.ToLower(value), keyword) {
				return &AttributeValue{
					ID:    FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	return h.findGenericDefaultValue(availableValues)
}

// findGenericDefaultValue 查找通用默认值
func (h *AttributeSelectorHandler) findGenericDefaultValue(availableValues map[int]string) *AttributeValue {
	// 优先查找通用默认值
	genericKeywords := []string{"other", "none", "其他", "不适用", "无", "默认"}

	for _, keyword := range genericKeywords {
		for valueID, value := range availableValues {
			lowerValue := strings.ToLower(value)
			if strings.Contains(lowerValue, keyword) {
				return &AttributeValue{
					ID:    FlexibleID(valueID),
					Value: value,
				}
			}
		}
	}

	// 如果没找到通用值，使用第一个可用值
	if len(availableValues) > 0 {
		for valueID, value := range availableValues {
			return &AttributeValue{
				ID:    FlexibleID(valueID),
				Value: value,
			}
		}
	}

	// 最后选择自定义值
	return &AttributeValue{
		ID:    FlexibleID(0),
		Value: "/",
	}
}

// truncateString 截断字符串到指定长度
func (h *AttributeSelectorHandler) truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen] + "..."
}

// fixCommonJsonIssues 修复常见的JSON问题
func (h *AttributeSelectorHandler) fixCommonJsonIssues(jsonStr string) string {
	// 这里可以添加具体的JSON修复逻辑
	// 目前只是一个占位符，实际实现可能需要根据具体问题来处理
	return jsonStr
}
