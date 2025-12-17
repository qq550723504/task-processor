package handlers

import (
	"fmt"
)

// PromptBuilder AI提示词构建器
type PromptBuilder struct{}

// NewPromptBuilder 创建新的提示词构建器
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildSystemPrompt 构建系统提示词
func (p *PromptBuilder) BuildSystemPrompt() string {
	return `你是TEMU平台的产品属性映射专家。你的任务是根据Amazon产品信息，为每个TEMU属性选择最合适的值。

【关键约束 - 必须严格遵守】
🚨 RefPID唯一性约束：每个RefPID在结果中只能出现指定次数
   - 单选属性（最大选择数量=1）：该RefPID只能出现1次
   - 多选属性（最大选择数量=N）：该RefPID最多出现N次
   - 违反此约束将导致提交失败

【映射规则】
1. 选择类型属性：🚨 必须从可选值列表中选择，并使用对应的VID
   - ❌ 禁止使用 vid: 0
   - ❌ 禁止自己创建"不适用"等值
   - ✅ 必须从提供的可选值中选择一个有效的VID
   - ✅ 如果不确定，优先选择"不适用"、"无"、"其他"等中性选项
2. 必填属性（✅必填）：必须提供值，不能跳过
3. 可选属性（⭕可选）：如果不确定或不适用，可以直接跳过，不需要填写
4. 数字类型属性：值必须在指定范围内
5. 文本类型属性：可以自定义填写
6. ID字段：使用模板提供的RefPID作为ref_pid，TemplatePID作为template_pid
7. **材质一致性原则**（🚨 极其重要，违反会导致审核失败）：
   - 材质属性必须与产品标题/描述中的材质描述完全一致
   - 如果标题提到"Steel"、"钢"、"不锈钢"，材质必须选择"钢"、"不锈钢"或"金属"
   - 如果标题提到"Plastic"、"塑料"，材质必须选择"塑料"相关选项
   - 如果标题提到"Wood"、"木"，材质必须选择"木质"相关选项
   - 只有在完全无法从标题/描述中确定材质时，才选择"混合材质"或"其他"
8. **常识判断原则**：
   - 用常识判断产品是否需要供电：如果产品标题/描述中没有电子相关词汇，默认不需要供电
   - 供电相关的必选属性（插头、电压、功率）：默认选择"无"、"不适用"、"无需供电"
   - 只有明确的电子产品（灯具、电器、充电设备等）才选择具体的供电参数
   - 可选属性如果不确定或不适用，直接跳过

【输出要求】
返回JSON格式，每个属性项包含：
{
  "ref_pid": [使用模板的RefPID],
  "pid": [使用模板的PID], 
  "template_pid": [使用模板的TemplatePID],
  "template_module_id": [使用模板的TemplateModuleID],
  "value": "[选择的值]",
  "vid": [对应的VID，选择类型必填],
  "value_unit": "[单位，如果有]"
}

【示例说明】
如果有两个属性都是RefPID=185的单选属性：
❌ 错误：两个都选择会导致RefPID=185出现2次
✅ 正确：只选择其中最合适的1个

【检查清单】
✅ 每个RefPID出现次数不超过其最大选择数量
✅ 单选属性的RefPID只出现1次
✅ ✅必填属性都有值
✅ 选择类型属性使用了正确的VID（不能是0）
✅ 使用了正确的ref_pid和template_pid
✅ ⭕可选属性如果不确定已跳过
✅ 供电相关属性：非电子产品必须选择"无"、"不适用"、"无需供电"
✅ 用常识检查：属性值是否符合产品的实际物理特征`
}

// BuildUserPrompt 构建用户提示词
func (p *PromptBuilder) BuildUserPrompt(data PropertyMappingData) string {
	prompt := fmt.Sprintf(`【Amazon产品信息】

标题: %s
品牌: %s
描述: %s
特性: %v
产品尺寸: %s
重量: %s
型号: %s
部门: %s
制造商: %s
类别: %v

【产品详情】
`, data.AmazonProduct.Title, data.AmazonProduct.Brand, data.AmazonProduct.Description,
		data.AmazonProduct.Features, data.AmazonProduct.ProductDimensions, data.AmazonProduct.ItemWeight,
		data.AmazonProduct.ModelNumber, data.AmazonProduct.Department, data.AmazonProduct.Manufacturer,
		data.AmazonProduct.Categories)

	for _, detail := range data.AmazonProduct.ProductDetails {
		prompt += fmt.Sprintf("- %s: %s\n", detail.Type, detail.Value)
	}

	prompt += "\n【TEMU属性模板】\n"
	for i, prop := range data.TemuProperties {
		// 构建选择类型描述
		selectionDesc := ""
		constraintIcon := ""
		if prop.PropertyValueType == 1 && prop.ChooseMaxNum > 0 {
			if prop.ChooseMaxNum == 1 {
				selectionDesc = "🔒单选(RefPID只能出现1次)"
				constraintIcon = "🚨"
			} else {
				selectionDesc = fmt.Sprintf("📋多选(RefPID最多出现%d次)", prop.ChooseMaxNum)
				constraintIcon = "⚠️"
			}
		}

		// 必填标记
		requiredMark := ""
		if prop.Required {
			requiredMark = "✅必填"
		} else {
			requiredMark = "⭕可选"
		}

		prompt += fmt.Sprintf("%d. %s%s %s %s\n", i+1, constraintIcon, prop.Name, requiredMark, selectionDesc)
		prompt += fmt.Sprintf("   📋 PID=%d | RefPID=%d | TemplatePID=%d | 类型=%d\n",
			prop.PID, prop.RefPID, prop.TemplatePID, prop.PropertyValueType)

		if len(prop.Values) > 0 {
			prompt += "   可选值: "
			for j, value := range prop.Values {
				if j > 0 {
					prompt += ", "
				}
				prompt += fmt.Sprintf("%s(VID:%d)", value.Value, value.VID)
			}
			prompt += "\n"
		}

		if len(prop.ValueUnit) > 0 {
			prompt += fmt.Sprintf("   单位: %v\n", prop.ValueUnit)
		}

		if prop.MinValue != "" || prop.MaxValue != "" {
			prompt += fmt.Sprintf("   范围: %s - %s\n", prop.MinValue, prop.MaxValue)
		}
	}

	prompt += `

【映射指南】
🎯 根据Amazon产品信息，为每个属性选择最合适的值
🔍 优先选择与产品特征最匹配的选项
📝 对于文本类型，可以基于产品信息自定义填写

【属性选择策略】
🎯 对于每个属性，按以下优先级处理：

   1️⃣ 从产品信息提取：能从标题、描述、特性中找到明确信息吗？
      - 能找到：使用提取的信息
      - 找不到：进入下一步
   
   2️⃣ 常识判断：根据产品的实际物理特征判断
      - 产品标题/描述中没有提到"电"、"充电"、"插电"、"LED"、"灯"等电子相关词汇
      - 产品是静态物品（雕像、摆件、装饰品、画框、收纳盒等）
      - 产品是非电子类（服装、鞋帽、箱包、书籍、玩具、文具等）
      → 这些产品显然不需要供电，供电方式必须选择"无需供电"、"不含电池"、"无"等选项
   
   3️⃣ 必选属性的中性值选择：
      - 供电方式（必选）：默认选择"无需供电"、"不含电池"、"无"（除非产品明确是电子产品）
      - 插头类型（必选）：默认选择"无插头"、"不适用"、"无"（除非产品明确需要插电）
      - 电压（必选）：默认选择"无"、"不适用"（除非产品明确是电器）
      - 材质（必选）：⚠️ 重要！必须与标题/描述中的材质描述保持一致
        * 如果标题/描述明确提到材质（如"钢材"、"不锈钢"、"塑料"、"木质"等），必须选择对应的材质选项
        * 如果标题提到"Steel"，必须选择"钢"、"不锈钢"或"金属"相关选项，不能选"混合材质"
        * 如果标题提到"Plastic"，必须选择"塑料"相关选项
        * 如果标题提到"Wood"，必须选择"木质"相关选项
        * 只有在完全无法确定材质时，才选择"混合材质"、"其他"、"复合材质"
      - 风格（必选）：选择"简约"、"百搭"、"基础款"、"经典"
      - 场景（必选）：选择"日常"、"通用"、"多场景"
   
   4️⃣ 可选属性（⭕可选）：如果不相关或不确定，直接跳过，不要填写

⚠️ 核心原则：
   - ✅必填属性必须填，但要用常识判断选择合理的值
   - ⭕可选属性不确定就直接跳过，不要强行填写
   - 大部分产品都不需要供电，默认选"无需供电"
   - 只有明确的电子产品才选择具体的插头、电压等
   - 🚨 选择类型属性必须使用有效的VID，不能使用vid: 0
   - 🚨🚨 材质属性必须与标题/描述保持一致，避免审核失败！
     例如：标题含"Steel"→必选"钢"/"不锈钢"/"金属"，不能选"混合材质"

【输出检查】
在生成结果前，请确认：
✅ 每个🔒单选属性的RefPID只出现1次
✅ 每个📋多选属性的RefPID不超过最大次数  
✅ 所有✅必填属性都有值
✅ 选择类型属性使用了正确的VID（不能是0）
✅ 使用了模板提供的RefPID和TemplatePID
✅ ⭕可选属性如果不确定已跳过（不在结果中）
✅ 【重要】供电相关属性：如果产品标题/描述中没有电子相关词汇，必须选择"无"、"不适用"、"无需供电"
✅ 【重要】用常识检查：雕像、摆件、装饰品、服装、箱包等非电子产品，不要选择插头、电压等供电参数
✅ 【关键】材质一致性检查：
   - 检查标题中是否提到材质关键词（Steel/钢/不锈钢/Plastic/塑料/Wood/木/等）
   - 如果标题提到"Steel"或"钢"，材质属性必须选择"钢"、"不锈钢"或"金属"相关选项
   - 材质选择必须与标题/描述中的材质描述完全一致
   - 不要在标题明确提到具体材质时选择"混合材质"或"其他"

请返回JSON格式：{"properties": [...]}`

	return prompt
}
