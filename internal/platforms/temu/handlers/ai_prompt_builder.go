package handlers

import (
	"fmt"
	"strings"
	"task-processor/internal/platforms/temu/types"
)

// PromptBuilder AI提示词构建器
type PromptBuilder struct{}

// NewPromptBuilder 创建新的提示词构建器
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// BuildSystemPrompt 构建系统提示词
func (p *PromptBuilder) BuildSystemPrompt() string {
	return `你是TEMU平台的产品属性映射专家。根据Amazon产品信息，为TEMU属性填充有价值的信息。

【核心目标】
🎯 最大化属性填充率：从Amazon产品信息中提取尽可能多的有价值属性
🔍 深度信息挖掘：充分利用标题、描述、特性、产品详情中的每个细节
📝 智能推理填充：基于产品类型和常识进行合理的属性推断

【🚨 严格约束 - 违反将导致失败】
1. **RefPID唯一性**：每个RefPID在结果中只能出现指定次数
   - 单选属性（最大选择数量=1）：该RefPID只能出现1次
   - 多选属性（最大选择数量=N）：该RefPID最多出现N次

2. **选择类型属性规则**：
   - ❌ **绝对禁止使用VID=0**
   - ❌ **绝对禁止自创任何值**（如"Other"、"不适用"、"Unknown"等）
   - ✅ **必须从可选值列表中选择**，使用对应的VID（VID > 0）
   - 🎯 **选择策略**：优先选择列表中的"不适用"、"无"、"其他"、"混合"等中性选项

【属性填充策略】
- **必填属性**：100%必须填写
- **重要可选属性**：积极填写（材质、颜色、尺寸、重量、风格、场景、用途、品牌、型号、功能特性等）
- **一般可选属性**：有明确信息时填写
- **不相关属性**：确实无关时才跳过

【信息提取优先级】
1️⃣ **直接提取**：从标题、描述、特性、产品详情中直接获取
2️⃣ **智能推理**：根据产品类型推断相关属性
3️⃣ **常识填充**：使用合理的默认值

【特殊属性处理】
- **材质属性**：必须与产品标题/描述中的材质描述完全一致
  * Steel/钢 → 选择"钢"、"不锈钢"或"金属"
  * Plastic/塑料 → 选择"塑料"相关选项
  * Wood/木 → 选择"木质"相关选项
- **供电属性**：根据产品实际情况判断
  * 包含"LED"、"电"、"充电"等 → 选择对应供电方式
  * 静态装饰品、服装、箱包等 → 选择"无需供电"
- **数值输入属性（control_type=16）**：需要同时填写选择值和数值
  * 面料成分：选择材料类型(vid)，填写成分占比(number_input_value)，总和必须为100%
  * 示例：{"vid": 95, "value": "nylon", "number_input_value": "100", "value_unit": "%"}

【输出格式】
返回JSON格式：{"properties": [...]}
每个属性项包含：ref_pid, pid, template_pid, template_module_id, value, vid, value_unit
对于数值输入属性，还需包含：number_input_value

【最终检查清单】
✅ 每个RefPID出现次数不超过其最大选择数量
✅ 所有必填属性都有值
✅ 选择类型属性使用了正确的VID（>0）且值在可选列表中
✅ 尽可能多的可选属性被填充
✅ 材质选择与标题/描述严格一致
✅ 绝对没有自创任何不在可选列表中的值`
}

// BuildUserPrompt 构建用户提示词
func (p *PromptBuilder) BuildUserPrompt(data types.PropertyMappingData) string {
	var builder strings.Builder

	// 添加产品信息
	builder.WriteString(fmt.Sprintf(`【Amazon产品信息】

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
		data.AmazonProduct.Categories))

	for _, detail := range data.AmazonProduct.ProductDetails {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", detail.Type, detail.Value))
	}

	builder.WriteString("\n【TEMU属性模板】\n")

	// 添加属性信息
	for i, prop := range data.TemuProperties {
		// 构建选择类型描述
		selectionDesc := ""
		constraintIcon := ""

		// 特殊处理control_type为16的属性（数值输入选择类型）
		if prop.ControlType == 16 {
			selectionDesc = "🔢数值输入选择"
			constraintIcon = "📊"
		} else if prop.PropertyValueType == 1 && prop.ChooseMaxNum > 0 {
			if prop.ChooseMaxNum == 1 {
				selectionDesc = "🔒单选"
				constraintIcon = "🚨"
			} else {
				selectionDesc = fmt.Sprintf("📋多选(最多%d)", prop.ChooseMaxNum)
				constraintIcon = "⚠️"
			}
		}

		// 必填标记
		requiredMark := "✅必填"
		if !prop.Required {
			requiredMark = "⭕可选"
		}

		builder.WriteString(fmt.Sprintf("%d. %s%s %s %s\n", i+1, constraintIcon, prop.Name, requiredMark, selectionDesc))
		builder.WriteString(fmt.Sprintf("   PID=%d | RefPID=%d | 类型=%d | 控制类型=%d\n", prop.PID, prop.RefPID, prop.PropertyValueType, prop.ControlType))

		// 特殊说明数值输入属性
		if prop.ControlType == 16 {
			builder.WriteString("   📊 需要同时填写：选择值(vid/value) + 数值输入(number_input_value) + 单位(value_unit)\n")
			if prop.Name == "面料成分" {
				builder.WriteString("   🧵 面料成分总和必须为100%\n")
			}
		}

		if len(prop.Values) > 0 {
			builder.WriteString("   🚨 必须从以下值中选择: ")
			for j, value := range prop.Values {
				if j > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(fmt.Sprintf("'%s'(VID:%d)", value.Value, value.VID))
			}
			builder.WriteString("\n")
		} else {
			builder.WriteString("   📝 文本输入类型\n")
		}

		if len(prop.ValueUnit) > 0 {
			builder.WriteString(fmt.Sprintf("   单位: %v\n", prop.ValueUnit))
		} else if len(prop.ValueUnitDTOList) > 0 {
			units := make([]string, len(prop.ValueUnitDTOList))
			for j, unit := range prop.ValueUnitDTOList {
				units[j] = unit.ValueUnit
			}
			builder.WriteString(fmt.Sprintf("   单位: %v\n", units))
		}

		if prop.MinValue != "" || prop.MaxValue != "" {
			builder.WriteString(fmt.Sprintf("   范围: %s - %s\n", prop.MinValue, prop.MaxValue))
		}

		// 添加数值输入的额外说明
		if prop.ControlType == 16 {
			if prop.PropertyChooseTitle != "" {
				builder.WriteString(fmt.Sprintf("   选择标题: %s\n", prop.PropertyChooseTitle))
			}
			if prop.NumberInputTitle != "" {
				builder.WriteString(fmt.Sprintf("   数值标题: %s\n", prop.NumberInputTitle))
			}
		}
	}

	builder.WriteString(`

【映射指南】
🎯 从Amazon产品信息中提取尽可能多的有价值属性信息

【信息提取策略】
1️⃣ **直接提取**：从标题、描述、特性、产品详情中获取
2️⃣ **智能推理**：根据产品类型推断相关属性
3️⃣ **特殊处理**：
   - **材质**：严格匹配标题/描述（Steel→金属，Plastic→塑料，Wood→木质）
   - **供电**：根据产品判断（LED/电器→对应供电方式，装饰品→无需供电）

【填充原则】
- ✅必填属性：100%必须填写
- ⭕可选属性：能填的尽量填，提高信息完整度
- 🚨选择约束：必须使用有效VID（>0），严禁自创值

【最终检查】
✅ RefPID出现次数符合限制
✅ 必填属性都有值  
✅ 选择类型属性VID>0且在可选列表中
✅ 材质选择与描述一致
✅ 没有自创任何值

请返回JSON格式：{"properties": [...]}`)

	return builder.String()
}
