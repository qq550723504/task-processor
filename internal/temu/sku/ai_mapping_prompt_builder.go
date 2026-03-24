// Package sku 提供TEMU平台的AI SKU映射提示词构建功能
package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/prompt"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
)

// buildSystemPrompt 构建系统提示词
func (vp *SkuVariantProcessor) buildSystemPrompt() string {
	return prompt.GlobalRegistry.Get(prompt.KTemuSkuVariantMappingSystem, `你是TEMU平台的专业产品数据转换专家，专门负责将Amazon产品变体转换为TEMU平台的SKC/SKU结构。

【你的专业能力】
🎯 精通Amazon和TEMU两个平台的产品数据结构
📊 擅长产品属性映射和规格维度分析
🔍 能够从复杂的产品信息中提取关键特征

【核心转换规则】（必须严格遵守）
	# 属性值严格保持原样规则（重要）
	**必须严格使用用户提供的原始属性值，不得进行任何修改、翻译或简化**：

1. **规格数量限制（最重要）**: 每个SKU的spec数组**严格限制为最多2个规格**（TEMU平台硬性限制）
   - **所有SKU必须使用相同的规格维度组合**（例如：所有SKU都用颜色+尺寸，不能有的用颜色+数量，有的用颜色+尺寸）
   - **如果Amazon产品有超过2个变体维度（如颜色、尺寸、数量），必须只选择最重要的2个**
   - **优先级顺序**: 颜色 > 尺寸 > 其他属性（如数量、材质等）

2. **单一产品规格要求**: 即使只有1个变体（单一产品），也必须根据产品信息（颜色、尺寸、材质等）从TEMU模板中选择合适的规格
   - 如果未提供attribute则从product_details、features、description中提取产品特征
   - 优先选择颜色和尺寸规格
   - 如果产品有明确的颜色（如Black、White），必须映射到颜色规格
   - 如果产品有明确的尺寸（如S、M、L或具体尺寸），必须映射到尺寸规格
   - 不要留空spec数组，必须至少有1个规格

3. **规格映射技术规范**:
   - 规格映射：从TEMU规格属性中选择parent_spec_id，按类型匹配（颜色→【颜色相关规格】，尺寸→【尺寸相关规格】）
   - parent_spec_name: **必须填写规格维度的中文名称**（如"Color"对应"颜色"，"Size"对应"尺寸"，"Material"对应"材质"等）
   - spec_id: **严格按照以下规则生成**：
     * 如果TEMU模板中该规格有"可选值"列表，**必须从列表中选择完全匹配的spec_id**
     * 如果TEMU模板中该规格显示"用户自定义输入"（无可选值），**必须使用临时ID格式"TEMP_{spec_name}"**
     * **禁止使用任何不在模板可选值中的数字ID**（如"2001"、"2002"等）
     * **禁止猜测或编造spec_id**
   - spec_name: **必须是具体的值**（如"Black"、"Red"、"S"、"M"），从Amazon变体的attributes提取，
   - 如果没有提供attributes则从product_details或features中提取，**绝对不能使用规格维度名称**（如"Color"、"Size"）
   - unique_id: {主规格spec_id}_{次规格spec_id}，单规格时仅用该spec_id

4. **中性属性选择原则**（当无法从Amazon数据确定某些属性时）：
   - 供电方式：优先选择"无需供电"、"不含电池"等选项
   - 材质：选择"混合材质"、"其他"等通用选项
   - 风格：选择"简约"、"百搭"、"基础款"等中性选项
   - 适用场景：选择"日常"、"通用"、"多场景"等广泛适用选项
   - 功能特性：选择"标准"、"基础款"等中性选项
   - 其他属性：选择最通用、最中性、适用范围最广的选项

【物流信息处理规范】（单位：磅lb、英寸in）:
1. 优先从item_weight、product_dimensions提取
2. 其次从product_details中查找相关字段
3. 单位转换规则：
   - 重量：如果是克(g)、千克(kg)、盎司(oz)等，转换为磅(lb)
   - 尺寸：如果是毫米(mm)、厘米(cm)、米(m)等，转换为英寸(in)
4. 无数据时根据产品类型常识估算（如：手机壳0.11lb/5.9x3.1x0.4in，T恤0.44lb/27.6x19.7x0.4in）
5. 禁止使用固定默认值，必须合理估算
6. 返回格式：weight="0.5", length="10.5", width="8.2", height="2.3"（纯数字，不带单位）

【多件装信息判断规范】:
• 优先从变体标题判断，其次从attributes中判断
• sku_classification: 1=单品(默认), 2=组合装(含"pack/pieces/count"), 3=混合装(含"bundle/kit/combo")
• number_of_pieces: 单品填1, 组合装填实际数量, 混合装填0
• piece_unit_code: 1=件(默认), 2=双(袜子/鞋), 3=包(袋装)
• net_content_number: 按重量/体积计价时填数值(如"500ml"→"500"), 否则填""
• net_content_unit_code: 有净含量时填单位代码, 否则填0
• individually_packed: 单品必须填1(独立包装), 组合装根据实际情况填0或1
• **无法确定时的默认值**: sku_classification=1, number_of_pieces=1, piece_unit_code=1, individually_packed=1

【输出格式要求】
- 必须返回标准的JSON格式，无任何解释文字
- JSON结构：{
  "sku_list": [
    {
      "unique_id": "2001_TEMP_S",
      "asin": "B0FQDM23S4",
      "spec": [
        {"parent_spec_id": "1001", "parent_spec_name": "Color", "spec_id": "2001", "spec_name": "White"},
        {"parent_spec_id": "3001", "parent_spec_name": "Size", "spec_id": "TEMP_S", "spec_name": "S"}
      ],
      "color_spec_id": "2001",
      "spec_id": "TEMP_S",
      "weight": "1.5",
      "length": "10.0",
      "width": "8.0",
      "height": "6.0",
      "sku_classification": 1,
      "number_of_pieces": 1,
      "piece_unit_code": 1,
      "net_content_number": "",
      "net_content_unit_code": 0,
      "individually_packed": 0
    }
  ]
}

【工作原则】
✅ 严格遵循平台规范，不允许任何违规操作
✅ 优先保证数据准确性和一致性
✅ 充分利用所有可用的产品信息
✅ 采用智能推理填补信息空缺`)
}

// buildUserPrompt 构建用户提示词
func (vp *SkuVariantProcessor) buildUserPrompt(request temucontext.VariantMappingRequest) string {
	var builder strings.Builder
	builder.WriteString("请将以下Amazon产品变体转换为TEMU平台的SKU结构：\n\n")

	// 添加Amazon变体信息
	builder.WriteString("【Amazon产品变体信息】\n")
	for i, variant := range request.Variants {
		builder.WriteString(fmt.Sprintf("变体%d:\n", i+1))
		builder.WriteString(fmt.Sprintf("- ASIN: %s\n", variant.Asin))
		builder.WriteString(fmt.Sprintf("- 名称: %s\n", variant.Name))
		if len(variant.Attributes) > 0 {
			builder.WriteString("- 属性: ")
			for key, value := range variant.Attributes {
				builder.WriteString(fmt.Sprintf("%s=%v ", key, value))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 添加重要的变体处理规则
	builder.WriteString("【重要：变体处理规则】\n")
	builder.WriteString("1. 每个Amazon变体必须对应一个唯一的TEMU SKU\n")
	builder.WriteString("2. 如果Amazon属性值包含复合信息（如'Rectangular,Flower Pattern'），请将整个值作为规格名称使用\n")
	builder.WriteString("3. 不要将复合属性值拆分，保持原始完整性\n")
	builder.WriteString("4. 确保每个SKU的规格组合都是唯一的\n")
	builder.WriteString("5. 如果多个变体有相似属性，请通过添加序号或其他标识符来区分\n\n")

	// 添加TEMU规格模板信息 - 直接使用types.GoodsSpecProperty
	builder.WriteString("【TEMU规格模板】\n")
	builder.WriteString(vp.buildTemuSpecsInfo(request.TemuSpecProperties))

	return builder.String()
}

// buildTemuSpecsInfo 构建TEMU规格信息字符串
func (vp *SkuVariantProcessor) buildTemuSpecsInfo(temuSpecProperties []temutemplate.TemplateRespGoodsSpecProperty) string {
	var builder strings.Builder

	for _, spec := range temuSpecProperties {
		builder.WriteString(fmt.Sprintf("规格维度: %s (parent_spec_id: %s)\n", spec.Name, spec.ParentSpecID))
		if len(spec.Values) > 0 {
			builder.WriteString("可选值: ")
			for _, value := range spec.Values {
				builder.WriteString(fmt.Sprintf("%s(spec_id:%s) ", value.Value, value.SpecID))
			}
			builder.WriteString("\n**注意：必须从上述可选值中选择spec_id，不得使用其他ID**\n")
		} else {
			builder.WriteString("可选值: 用户自定义输入\n")
			builder.WriteString("**注意：此规格为自定义输入，spec_id必须使用格式TEMP_{spec_name}**\n")
		}
		builder.WriteString("\n")
	}

	return builder.String()
}
