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
1. **必填属性绝对不能跳过**：
   - ✅ **所有必填属性都必须填充**，即使找不到完美匹配
   - 🎯 **降级匹配策略**：精确匹配 → 模糊匹配 → 中性选项 → 第一个选项
   - ❌ **绝对禁止跳过任何必填属性**

2. **条件依赖属性强制处理**：
   - 🔗 **识别条件依赖**：检查template_property_value_parent_list字段和parent_vids
   - 🔍 **父属性检查**：确认父属性是否已填充且值正确
   - ⚠️ **子属性约束**：子属性值必须在父属性约束的范围内（检查parent_vids匹配）
   - 🚨 **条件必填规则**：当父属性满足条件时，子属性立即变为必填
   - 📋 **处理流程**：
     * 填充父属性 → 检查是否触发条件依赖 → 从约束范围内选择子属性值 → 强制填充对应的子属性
   - 🔥 **关键示例**：
     * Power Supply = "Plug Powered" (vid:36781) → Operating Voltage 必须从parent_vids=[36781]的选项中选择
     * Power Supply = "DC Power Supply" (vid:69104) → Operating Voltage 必须从parent_vids=[69104]的选项中选择

3. **RefPID唯一性**：每个RefPID在结果中只能出现指定次数
   - 单选属性（最大选择数量=1）：该RefPID只能出现1次
   - 多选属性（最大选择数量=N）：该RefPID最多出现N次

4. **选择类型属性规则**：
   - ❌ **绝对禁止使用VID=0**
   - ❌ **绝对禁止自创任何值**（如"Other"、"不适用"、"Unknown"等）
   - ✅ **必须从可选值列表中选择**，使用对应的VID（VID > 0）
   - 🚨 **VID约束严格匹配**：每个template_pid只能使用该属性模板中列出的VID，绝对不能混用其他属性的VID
   - 🔥 **关键规则**：相同PID但不同template_pid的属性有完全不同的VID列表，必须严格区分
   - 🎯 **选择策略**：精确匹配 → 包含匹配 → 中性选项（"不适用"、"无"、"其他"、"混合"）→ 第一个选项

【属性填充策略】
- **必填属性**：🚨 **100%必须填写，绝对不能跳过**
  * 找到精确匹配 → 使用精确匹配
  * 找到模糊匹配 → 使用最佳模糊匹配
  * 找不到匹配 → 使用中性选项（"其他"、"不适用"、"无"、"混合"等）
  * 没有中性选项 → 使用第一个可选值
- **重要可选属性**：积极填写（材质、颜色、尺寸、重量、风格、场景、用途、品牌、型号、功能特性等）
- **一般可选属性**：有明确信息时填写
- **不相关属性**：确实无关时才跳过

【信息提取优先级】
1️⃣ **直接提取**：从标题、描述、特性、产品详情中直接获取
2️⃣ **智能推理**：根据产品类型推断相关属性
3️⃣ **模糊匹配**：当找不到精确匹配时，选择最相近的选项
4️⃣ **降级填充**：使用中性选项或第一个可选值确保必填属性不为空

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
🚨 **template_pid必须准确**：对于相同PID的多个属性，必须使用正确的template_pid进行区分
对于数值输入属性，还需包含：number_input_value

【最终检查清单】
✅ **所有必填属性都已填充**（这是最重要的检查项）
✅ **所有条件依赖属性都已正确处理**（当父属性满足条件时，子属性必须填充）
✅ 每个RefPID出现次数不超过其最大选择数量
✅ 选择类型属性使用了正确的VID（>0）且值在可选列表中
✅ 条件依赖的子属性值在父属性约束的范围内
✅ 尽可能多的可选属性被填充
✅ 当找不到精确匹配时，使用了合理的降级选项
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

		// 条件依赖标记
		conditionMark := ""
		if prop.ParentTemplatePID > 0 || len(prop.TemplatePropertyValueParentList) > 0 {
			conditionMark = "🔗条件依赖"
			if prop.Required {
				requiredMark = "🔗条件必填"
			}
		}

		builder.WriteString(fmt.Sprintf("%d. %s%s %s %s %s\n", i+1, constraintIcon, prop.Name, requiredMark, conditionMark, selectionDesc))
		builder.WriteString(fmt.Sprintf("   PID=%d | TemplatePID=%d | RefPID=%d | 类型=%d | 控制类型=%d\n", prop.PID, prop.TemplatePID, prop.RefPID, prop.PropertyValueType, prop.ControlType))

		// 添加条件依赖说明
		if prop.ParentTemplatePID > 0 {
			builder.WriteString(fmt.Sprintf("   🔗 依赖父属性: template_pid=%d\n", prop.ParentTemplatePID))
		}
		if len(prop.TemplatePropertyValueParentList) > 0 {
			builder.WriteString("   🔗 条件约束: 只有当父属性为特定值时才需要填充\n")
			for _, parentList := range prop.TemplatePropertyValueParentList {
				if len(parentList.ParentVIDs) > 0 {
					builder.WriteString(fmt.Sprintf("   🔗 父条件VID: %v → 可选子VID: %v\n", parentList.ParentVIDs, parentList.VIDs))
				}
			}
			builder.WriteString("   🚨 重要：子属性的VID必须从对应父VID的约束范围内选择！\n")
		}

		// 特殊说明数值输入属性
		if prop.ControlType == 16 {
			builder.WriteString("   📊 需要同时填写：选择值(vid/value) + 数值输入(number_input_value) + 单位(value_unit)\n")
			if prop.Name == "面料成分" {
				builder.WriteString("   🧵 面料成分总和必须为100%\n")
			}
		}

		if len(prop.Values) > 0 {
			// 检查是否有条件依赖
			if len(prop.TemplatePropertyValueParentList) > 0 {
				builder.WriteString("   🔗 条件依赖可选值（按父属性分组）:\n")

				// 按父VID分组显示可选值
				for _, parentList := range prop.TemplatePropertyValueParentList {
					if len(parentList.ParentVIDs) > 0 && len(parentList.VIDs) > 0 {
						builder.WriteString(fmt.Sprintf("   🔗 当父属性VID为 %v 时，可选VID: %v\n",
							parentList.ParentVIDs, parentList.VIDs))

						// 显示对应的值
						builder.WriteString("      对应的可选值: ")
						validValues := make([]string, 0)
						for _, vid := range parentList.VIDs {
							for _, value := range prop.Values {
								if value.VID == vid {
									validValues = append(validValues, fmt.Sprintf("'%s'(VID:%d)", value.Value, value.VID))
									break
								}
							}
						}
						builder.WriteString(strings.Join(validValues, ", "))
						builder.WriteString("\n")
					}
				}

				// 显示无条件的值（如果有）
				unconditionalValues := make([]string, 0)
				for _, value := range prop.Values {
					if len(value.ParentVIDs) == 0 {
						unconditionalValues = append(unconditionalValues, fmt.Sprintf("'%s'(VID:%d)", value.Value, value.VID))
					}
				}
				if len(unconditionalValues) > 0 {
					builder.WriteString("   🔗 无条件可选值: ")
					builder.WriteString(strings.Join(unconditionalValues, ", "))
					builder.WriteString("\n")
				}
			} else {
				// 普通属性，显示所有可选值
				builder.WriteString("   🚨 必须从以下值中选择: ")
				for j, value := range prop.Values {
					if j > 0 {
						builder.WriteString(", ")
					}
					builder.WriteString(fmt.Sprintf("'%s'(VID:%d)", value.Value, value.VID))
				}
				builder.WriteString("\n")
			}

			// 🔥 强化VID约束说明
			builder.WriteString(fmt.Sprintf("   🔥 VID约束：template_pid=%d 只能使用上述VID，绝对不能使用其他属性的VID\n", prop.TemplatePID))

			// 特别强调条件属性的值约束
			if len(prop.TemplatePropertyValueParentList) > 0 {
				builder.WriteString("   ⚠️ 条件属性：当父属性不满足条件时，此属性不应填充\n")
			}
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
- 🚨**必填属性**：**绝对不能跳过**，必须使用降级匹配策略
  * 精确匹配 → 模糊匹配 → 中性选项 → 第一个选项
- 🔗**条件依赖属性**：**当父属性满足条件时立即变为必填**
  * 🔍 **步骤1**：先确定父属性的VID值
  * 🔍 **步骤2**：根据父VID找到对应的子属性可选VID列表
  * 🔍 **步骤3**：只能从对应的VID列表中选择，绝对不能选择其他VID
  * 🔥 **关键规则**：子属性的VID必须在对应父VID的约束范围内
  * 🔥 **具体示例**：
    - Power Supply选择"Plug Powered"(vid:36781) → Operating Voltage只能从parent_vids=[36781]对应的VID列表中选择
    - Power Supply选择"DC Power Supply"(vid:69104) → Operating Voltage只能从parent_vids=[69104]对应的VID列表中选择
  * ❌ **严禁跨条件选择**：不能将DC Power Supply的电压值用于Plug Powered
- ⭕**可选属性**：能填的尽量填，提高信息完整度
- 🎯**匹配策略**：优先精确匹配，找不到时使用最相近选项
- 🚨**选择约束**：必须使用有效VID（>0），严禁自创值

【最终检查】
🚨 **所有必填属性都已填充**（最重要！）
🔗 **所有条件依赖属性都已正确处理**（父属性触发时子属性必填）
✅ RefPID出现次数符合限制
✅ 选择类型属性VID>0且在可选列表中
✅ 条件依赖的子属性值在父属性约束范围内
✅ 使用了合理的降级匹配策略
✅ 没有自创任何值

请返回JSON格式：{"properties": [...]}`)

	return builder.String()
}
