// Package attribute 提供SHEIN平台属性选择提示词生成功能
package attribute

import (
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/prompt"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
)

// AttributePromptGenerator 属性提示词生成器
type AttributePromptGenerator struct{}

// NewAttributePromptGenerator 创建新的属性提示词生成器
func NewAttributePromptGenerator() *AttributePromptGenerator {
	return &AttributePromptGenerator{}
}

// GenerateSystemPrompt 生成系统提示词
func (g *AttributePromptGenerator) GenerateSystemPrompt(attributeTemplates *attribute.AttributeTemplateInfo) (string, error) {
	return g.generateDynamicAttributeSystemPrompt(attributeTemplates)
}

// GenerateDefaultSystemPrompt 生成默认系统提示词
func (g *AttributePromptGenerator) GenerateDefaultSystemPrompt() string {
	prompt, _ := g.generateAttributeSystemPrompt()
	return prompt
}

// GenerateUserPrompt 生成用户提示词
func (g *AttributePromptGenerator) GenerateUserPrompt(ctx *shein.TaskContext, enhancedAttributeData []shein.EnhancedGenerateAttribute) string {
	return g.generateDynamicUserPrompt(ctx, enhancedAttributeData)
}

// generateAttributeSystemPrompt 生成基础系统提示词
func (g *AttributePromptGenerator) generateAttributeSystemPrompt() (string, error) {
	return prompt.GlobalRegistry.Get("shein.attribute_selector.system", `你是一个专业的产品属性优化专家。请将亚马逊产品属性转换为SHEIN平台适用的属性格式。

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
- 还需关注属性依赖关系，确保相关属性同时填写`), nil
}

// generateDynamicAttributeSystemPrompt 基于属性模板数据生成动态系统提示词
func (g *AttributePromptGenerator) generateDynamicAttributeSystemPrompt(attributeTemplates *attribute.AttributeTemplateInfo) (string, error) {
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
	dependencyInfo := g.analyzeAttributeDependencies(attributeTemplates)

	// 分析关键属性
	keyAttributes := g.identifyKeyAttributes(attributeTemplates)

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
func (g *AttributePromptGenerator) generateDynamicUserPrompt(ctx *shein.TaskContext, enhancedAttributeData []shein.EnhancedGenerateAttribute) string {
	// 按重要性分组属性
	var criticalAttrs []shein.EnhancedGenerateAttribute
	var importantAttrs []shein.EnhancedGenerateAttribute
	var optionalAttrs []shein.EnhancedGenerateAttribute

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
		g.formatAttributeList(criticalAttrs),
		len(importantAttrs),
		g.formatAttributeList(importantAttrs),
		len(optionalAttrs),
		g.formatAttributeList(optionalAttrs),
		string(attributeDataBytes))

	return prompt
}

// analyzeAttributeDependencies 分析属性依赖关系
func (g *AttributePromptGenerator) analyzeAttributeDependencies(attributeTemplates *attribute.AttributeTemplateInfo) string {
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
func (g *AttributePromptGenerator) identifyKeyAttributes(attributeTemplates *attribute.AttributeTemplateInfo) string {
	if len(attributeTemplates.Data) == 0 {
		return "无属性模板信息"
	}

	var keyAttributes []string
	importanceService := NewImportanceService()

	for _, attribute := range attributeTemplates.Data[0].AttributeInfos {
		// 使用统一的重要性计算函数
		importanceResult := importanceService.CalculateAttributeImportance(&attribute)

		if importanceResult.Importance >= 80 {
			keyAttributes = append(keyAttributes, fmt.Sprintf("- %s (ID:%d, 评分:%d)", attribute.AttributeName, attribute.AttributeID, importanceResult.Importance))
		}
	}

	if len(keyAttributes) == 0 {
		return "未检测到高重要性属性"
	}

	return strings.Join(keyAttributes, "\n")
}

// formatAttributeList 格式化属性列表用于显示
func (g *AttributePromptGenerator) formatAttributeList(attributes []shein.EnhancedGenerateAttribute) string {
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
