package handlers

import (
	"fmt"
	"strings"
)

// buildAIPrompt 构建AI提示
func (sb *SkuBuilder) buildAIPrompt(request VariantMappingRequest) string {
	variantsJSON, _ := sb.marshalWithoutHTMLEscape(request.Variants)
	temuSpecsInfo := sb.buildTemuSpecsInfo(request.TemuSpecProperties)

	return fmt.Sprintf(`将Amazon变体转换为TEMU SKU结构：

产品: %s

Amazon变体数据（包含description、features、product_details）:
%s

TEMU规格属性:
%s

重要规则:
1. **规格数量限制（最重要）**: 每个SKU的spec数组**严格限制为最多2个规格**（TEMU平台硬性限制）
   - **所有SKU必须使用相同的规格维度组合**（例如：所有SKU都用颜色+尺寸，不能有的用颜色+数量，有的用颜色+尺寸）
   - **如果Amazon产品有超过2个变体维度（如颜色、尺寸、数量），必须只选择最重要的2个**
   - **优先级顺序**: 颜色 > 尺寸 > 其他属性（如数量、材质等）
   - **示例**: 如果产品有颜色、尺寸、数量3个维度，只选择颜色+尺寸，忽略数量维度
2. **单一产品规格要求**: 即使只有1个变体（单一产品），也必须根据产品信息（颜色、尺寸、材质等）从TEMU模板中选择合适的规格
   - 从product_details、features、description中提取产品特征
   - 优先选择颜色和尺寸规格
   - 如果产品有明确的颜色（如Black、White），必须映射到颜色规格
   - 如果产品有明确的尺寸（如S、M、L或具体尺寸），必须映射到尺寸规格
   - 不要留空spec数组，必须至少有1个规格
3. 规格映射：从TEMU规格属性中选择parent_spec_id，按类型匹配（颜色→【颜色相关规格】，尺寸→【尺寸相关规格】）
4. spec_id: **必须从TEMU模板的可选值中选择**。如果确实没有匹配的可选值，使用临时ID格式"TEMP_{spec_name}"
5. spec_name: **必须是具体的值**（如"Black"、"Red"、"S"、"M"），从Amazon变体的attributes、product_details或features中提取，**绝对不能使用规格维度名称**（如"Color"、"Size"）
6. unique_id: {主规格spec_id}_{次规格spec_id}，单规格时仅用该spec_id
7. 必填规格必须包含
6. **中性属性选择原则**（当无法从Amazon数据确定某些属性时）：
   - 供电方式：优先选择"无需供电"、"不含电池"等选项
   - 材质：选择"混合材质"、"其他"等通用选项
   - 风格：选择"简约"、"百搭"、"基础款"等中性选项
   - 适用场景：选择"日常"、"通用"、"多场景"等广泛适用选项
   - 功能特性：选择"标准"、"基础款"等中性选项
   - 其他属性：选择最通用、最中性、适用范围最广的选项

【物流信息】提取/估算规则（单位：磅lb、英寸in）:
1. 优先从item_weight、product_dimensions提取
2. 其次从product_details中查找相关字段
3. 单位转换规则：
   - 重量：如果是克(g)、千克(kg)、盎司(oz)等，转换为磅(lb)
   - 尺寸：如果是毫米(mm)、厘米(cm)、米(m)等，转换为英寸(in)
4. 无数据时根据产品类型常识估算（如：手机壳0.11lb/5.9x3.1x0.4in，T恤0.44lb/27.6x19.7x0.4in）
5. 禁止使用固定默认值，必须合理估算
6. 返回格式：weight="0.5", length="10.5", width="8.2", height="2.3"（纯数字，不带单位）

【多件装信息】智能判断:
• sku_classification: 1=单品(默认), 2=组合装(含"pack/pieces/count"), 3=混合装(含"bundle/kit/combo")
• number_of_pieces: 单品填1, 组合装填实际数量, 混合装填0
• piece_unit_code: 1=件(默认), 2=双(袜子/鞋), 3=包(袋装)
• net_content_number: 按重量/体积计价时填数值(如"500ml"→"500"), 否则填""
• net_content_unit_code: 有净含量时填单位代码, 否则填0
• individually_packed: 单品必须填1(独立包装), 组合装根据实际情况填0或1
• **无法确定时的默认值**: sku_classification=1, number_of_pieces=1, piece_unit_code=1, individually_packed=1

返回JSON（无解释文字）:
{
  "sku_list": [
    {
      "unique_id": "2001_TEMP_S",
      "asin": "B0FQDM23S4",
      "spec": [
        {"parent_spec_id": "1001", "spec_id": "2001", "spec_name": "白色"},
        {"parent_spec_id": "3001", "spec_id": "TEMP_S", "spec_name": "S"}
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

注意：spec_name必须是从Amazon attributes中提取的具体值（如"Black"、"White"、"Small"、"Large"），不能是规格维度名称（如"Color"、"Size"）！`, request.ProductTitle, string(variantsJSON), temuSpecsInfo)
}

// getAIInstructions 获取AI指令
func (sb *SkuBuilder) getAIInstructions() string {
	return "智能转换Amazon变体为TEMU SKU：识别属性→匹配规格→生成唯一ID→建立关联"
}

// buildTemuSpecsInfo 构建TEMU规格属性信息
func (sb *SkuBuilder) buildTemuSpecsInfo(specProperties []GoodsSpecProperty) string {
	if len(specProperties) == 0 {
		return "无可用规格属性"
	}

	var info strings.Builder
	info.WriteString("可用规格属性:\n")

	// 按属性类型分组显示
	colorSpecs := []GoodsSpecProperty{}
	sizeSpecs := []GoodsSpecProperty{}
	otherSpecs := []GoodsSpecProperty{}

	for _, spec := range specProperties {
		specNameLower := strings.ToLower(spec.Name)
		if sb.isColorSpec(specNameLower) {
			colorSpecs = append(colorSpecs, spec)
		} else if sb.isSizeSpec(specNameLower) {
			sizeSpecs = append(sizeSpecs, spec)
		} else {
			otherSpecs = append(otherSpecs, spec)
		}
	}

	// 显示颜色相关规格
	if len(colorSpecs) > 0 {
		info.WriteString("【颜色相关规格】:\n")
		for _, spec := range colorSpecs {
			sb.writeSpecInfo(&info, spec)
		}
	}

	// 显示尺寸相关规格
	if len(sizeSpecs) > 0 {
		info.WriteString("【尺寸相关规格】:\n")
		for _, spec := range sizeSpecs {
			sb.writeSpecInfo(&info, spec)
		}
	}

	// 显示其他规格
	if len(otherSpecs) > 0 {
		info.WriteString("【其他规格】:\n")
		for _, spec := range otherSpecs {
			sb.writeSpecInfo(&info, spec)
		}
	}

	return info.String()
}

// writeSpecInfo 写入规格信息
func (sb *SkuBuilder) writeSpecInfo(info *strings.Builder, spec GoodsSpecProperty) {
	info.WriteString(fmt.Sprintf("- %s (parent_spec_id:%s)", spec.Name, spec.ParentSpecID))
	if spec.Required {
		info.WriteString(" [必填]")
	}

	if len(spec.Values) > 0 {
		info.WriteString(" 可选值:")
		for j, value := range spec.Values {
			if j > 0 {
				info.WriteString(",")
			}
			info.WriteString(fmt.Sprintf("%s(%s)", value.Value, value.SpecID))
		}
	} else {
		info.WriteString(" [无预定义值，将通过API创建]")
	}
	info.WriteString("\n")
}

// isColorSpec 判断是否为颜色相关规格
func (sb *SkuBuilder) isColorSpec(specName string) bool {
	colorKeywords := []string{"color", "colour", "颜色", "色彩", "色调"}
	for _, keyword := range colorKeywords {
		if strings.Contains(specName, keyword) {
			return true
		}
	}
	return false
}

// isSizeSpec 判断是否为尺寸相关规格
func (sb *SkuBuilder) isSizeSpec(specName string) bool {
	sizeKeywords := []string{"size", "尺寸", "尺码", "大小", "规格"}
	for _, keyword := range sizeKeywords {
		if strings.Contains(specName, keyword) {
			return true
		}
	}
	return false
}

// convertUserInputSpecsToGoodsSpecProperties 将用户输入规格转换为商品规格属性
func (sb *SkuBuilder) convertUserInputSpecsToGoodsSpecProperties(userInputSpecs []UserInputParentSpec) []GoodsSpecProperty {
	var specProperties []GoodsSpecProperty

	for i, userSpec := range userInputSpecs {
		// 创建基本的GoodsSpecProperty结构
		specProperty := GoodsSpecProperty{
			PID:               i + 1000, // 使用临时ID
			TemplateModuleID:  0,
			TemplatePID:       0,
			RefPID:            0,
			Name:              userSpec.ParentSpecName,
			PropertyValueType: 1, // 假设为选择类型
			ValueUnit:         []string{},
			Values:            []PropertyValue{}, // 用户输入规格通常没有预定义值
			MaxValue:          "",
			MinValue:          "",
			ValuePrecision:    0,
			Required:          false,
			IsSale:            true,
			ParentSpecID:      userSpec.ParentSpecID,
			MainSale:          true,
			Feature:           0,
			ControlType:       1,
		}

		specProperties = append(specProperties, specProperty)
		sb.logger.Debugf("转换用户输入规格: %s (parent_spec_id: %s)",
			userSpec.ParentSpecName, userSpec.ParentSpecID)
	}

	sb.logger.Infof("成功转换%d个用户输入规格为商品规格属性", len(specProperties))
	return specProperties
}
