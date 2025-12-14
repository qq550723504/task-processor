package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/common/amazon/model"
	"task-processor/common/pipeline"
	"task-processor/openai"
	"task-processor/platforms/temu/types"
)

// VariantMappingRequest AI变体映射请求
type VariantMappingRequest struct {
	ProductTitle       string               `json:"product_title"`
	Variants           []AmazonVariantForAI `json:"variants"`
	Instructions       string               `json:"instructions"`
	TemuSpecProperties []GoodsSpecProperty  `json:"temu_spec_properties"`
}

// AmazonVariantForAI AI处理用的Amazon变体数据
type AmazonVariantForAI struct {
	Name              string            `json:"name"`
	Asin              string            `json:"asin"`
	Price             float64           `json:"price"`
	Image             string            `json:"image"`
	Attributes        map[string]any    `json:"attributes"`
	ProductDimensions string            `json:"product_dimensions,omitempty"` // 产品尺寸
	ItemWeight        string            `json:"item_weight,omitempty"`        // 产品重量
	Description       string            `json:"description,omitempty"`        // 产品描述
	Features          []string          `json:"features,omitempty"`           // 产品特性
	ProductDetails    map[string]string `json:"product_details,omitempty"`    // 产品详情
}

// AISkuMappingResponse AI SKU映射响应
type AISkuMappingResponse struct {
	SkuList []AIGeneratedSku `json:"sku_list"`
}

// AIGeneratedSku AI生成的SKU结构
type AIGeneratedSku struct {
	UniqueID          string            `json:"unique_id"`
	Asin              string            `json:"asin"`
	Spec              []types.SpecInfo  `json:"spec"`
	ColorSpecID       string            `json:"color_spec_id"`
	SpecID            string            `json:"spec_id"`
	VariantAttributes map[string]string `json:"variant_attributes"`
	// 物流信息
	Weight string `json:"weight"` // 重量，单位：克
	Length string `json:"length"` // 长度，单位：毫米
	Width  string `json:"width"`  // 宽度，单位：毫米
	Height string `json:"height"` // 高度，单位：毫米
	// 多件装信息
	SkuClassification  int    `json:"sku_classification"`    // SKU类型：1-单品，2-组合装，3-混合装
	NumberOfPieces     int    `json:"number_of_pieces"`      // 可单独售卖的产品数量
	PieceUnitCode      int    `json:"piece_unit_code"`       // 单位规格ID：1-件，2-双，3-包
	NetContentNumber   string `json:"net_content_number"`    // 净含量数值（用于计算单价）
	NetContentUnitCode int    `json:"net_content_unit_code"` // 净含量单位代码
	IndividuallyPacked int    `json:"individually_packed"`   // 是否独立包装：1-是，0-否
}

// generateAISkuMapping 使用AI生成SKU映射
func (sb *SkuBuilder) generateAISkuMapping(ctx *pipeline.TaskContext, variants []*model.Product) (*AISkuMappingResponse, error) {
	if sb.aiClient == nil {
		return nil, fmt.Errorf("AI客户端未初始化")
	}

	// 检查变体数量限制（超过100个变体无法处理，不应重试）
	if len(variants) > 100 {
		sb.logger.Errorf("❌ 变体数量超过限制: %d > 100，系统无法处理如此多的变体", len(variants))
		sb.logger.Error("❌ 此错误不应重试，请检查产品数据或联系技术支持")
		return nil, fmt.Errorf("变体数量超过限制: %d > 100，系统无法处理", len(variants))
	}

	// 根据token限制决定是否需要分批处理
	// Gemini 2.0 Flash输出限制约8000 tokens，每个SKU约300 tokens
	// 安全起见，每批最多处理20个变体
	const maxVariantsPerBatch = 20

	if len(variants) > maxVariantsPerBatch {
		sb.logger.Infof("🔄 变体数量(%d)超过单批限制(%d)，将分批处理", len(variants), maxVariantsPerBatch)
		return sb.generateAISkuMappingInBatches(ctx, variants, maxVariantsPerBatch)
	}

	// 单批处理
	return sb.generateAISkuMappingSingleBatch(ctx, variants)
}

// generateAISkuMappingInBatches 分批生成AI SKU映射
func (sb *SkuBuilder) generateAISkuMappingInBatches(ctx *pipeline.TaskContext, variants []*model.Product, batchSize int) (*AISkuMappingResponse, error) {
	totalBatches := (len(variants) + batchSize - 1) / batchSize
	sb.logger.Infof("📦 开始分批处理: 总变体数=%d, 批次大小=%d, 总批次=%d", len(variants), batchSize, totalBatches)

	var allSkus []AIGeneratedSku
	var selectedSpecDimensions []string // 记录第一批选择的规格维度

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variants) {
			end = len(variants)
		}

		batchVariants := variants[start:end]
		sb.logger.Infof("📦 处理批次 %d/%d: 变体[%d-%d]", batchIndex+1, totalBatches, start, end-1)

		// 处理当前批次
		batchResponse, err := sb.generateAISkuMappingSingleBatch(ctx, batchVariants)
		if err != nil {
			sb.logger.Errorf("❌ 批次 %d/%d 处理失败: %v", batchIndex+1, totalBatches, err)
			return nil, fmt.Errorf("批次 %d 处理失败: %w", batchIndex+1, err)
		}

		// 第一批：记录选择的规格维度
		if batchIndex == 0 && len(batchResponse.SkuList) > 0 {
			specDimensions := make(map[string]bool)
			for _, spec := range batchResponse.SkuList[0].Spec {
				specDimensions[spec.ParentSpecID] = true
			}
			for parentSpecID := range specDimensions {
				selectedSpecDimensions = append(selectedSpecDimensions, parentSpecID)
			}
			sb.logger.Infof("📌 第一批选择的规格维度: %v", selectedSpecDimensions)
		}

		allSkus = append(allSkus, batchResponse.SkuList...)
		sb.logger.Infof("✅ 批次 %d/%d 完成，生成%d个SKU", batchIndex+1, totalBatches, len(batchResponse.SkuList))
	}

	sb.logger.Infof("✅ 所有批次处理完成，共生成%d个SKU", len(allSkus))

	// 合并后的结果需要再次统一规格维度
	// 因为不同批次可能选择了不同的规格维度
	mergedResponse := &AISkuMappingResponse{
		SkuList: allSkus,
	}

	sb.logger.Info("🔄 对合并结果执行规格维度统一...")
	sb.enforceSpecCountLimit(mergedResponse)

	return mergedResponse, nil
}

// generateAISkuMappingSingleBatch 单批次生成AI SKU映射
func (sb *SkuBuilder) generateAISkuMappingSingleBatch(ctx *pipeline.TaskContext, variants []*model.Product) (*AISkuMappingResponse, error) {

	// 准备AI请求数据
	sb.logger.Infof("开始准备AI请求数据，变体数量: %d", len(variants))

	aiVariants := make([]AmazonVariantForAI, len(variants))
	// 创建ASIN到attributes的映射，用于后续填充VariantAttributes
	asinToAttributes := make(map[string]map[string]any)
	successCount := 0
	failedCount := 0

	// 创建ASIN到完整变体信息的映射
	asinToFullVariant := make(map[string]*model.Product)
	if ctx.AmazonVariants != nil {
		for _, fullVariant := range ctx.AmazonVariants {
			asinToFullVariant[fullVariant.Asin] = fullVariant
		}
		sb.logger.Infof("从上下文获取到%d个完整变体信息", len(asinToFullVariant))
	}

	for i, variant := range variants {
		// 使用variant作为fullVariant（保持后续代码兼容）
		fullVariant := variant

		// 提取变体的属性信息
		// 关键修复：优先使用索引从父产品的Variations中获取正确的attributes
		// 因为同一个ASIN可能对应多个不同的尺寸组合
		attributes := make(map[string]any)

		// 方法1：优先从父产品的Variations中按索引获取（最可靠）
		// 这样可以避免ASIN重复导致的匹配错误
		if ctx.AmazonProduct != nil && i < len(ctx.AmazonProduct.Variations) {
			variation := ctx.AmazonProduct.Variations[i]
			if len(variation.Attributes) > 0 {
				attributes = variation.Attributes
				asinToAttributes[variant.Asin] = attributes
				successCount++
				sb.logger.Infof("✅ 变体[%d]从父产品Variations匹配（按索引）: ASIN=%s, Attributes=%+v", i, variant.Asin, attributes)
			} else {
				sb.logger.Warnf("⚠️ 变体[%d]从父产品Variations找到但Attributes为空: ASIN=%s", i, variant.Asin)
			}
		}

		// 方法2：如果方法1失败，尝试从variant自己的Variations中获取
		if len(attributes) == 0 && len(variant.Variations) > 0 {
			sb.logger.Infof("🔍 变体[%d]从自身Variations中查找: ASIN=%s", i, variant.Asin)
			for _, variation := range variant.Variations {
				if variation.Asin == variant.Asin {
					if len(variation.Attributes) > 0 {
						attributes = variation.Attributes
						asinToAttributes[variant.Asin] = attributes
						successCount++
						sb.logger.Infof("✅ 变体[%d]从自身Variations匹配: ASIN=%s, Attributes=%+v", i, variant.Asin, attributes)
					}
					break
				}
			}
		}

		// 如果还是没有属性，检查是否为单一产品（没有变体）
		if len(attributes) == 0 {
			// 对于单一产品（没有变体），这是正常情况
			if len(variants) == 1 {
				sb.logger.Infof("ℹ️ 单一产品（无变体），AI将根据产品标题和描述生成规格: ASIN=%s", variant.Asin)
			} else {
				// 对于多变体产品，缺少attributes是个问题
				failedCount++
				sb.logger.Errorf("❌ 变体[%d]无法获取Attributes: ASIN=%s, 这将导致AI生成不完整的规格", i, variant.Asin)
				sb.logger.Errorf("   变体信息: Title=%s, Variations数量=%d", variant.Title, len(variant.Variations))
			}
		}

		// 转换ProductDetails为map
		productDetailsMap := make(map[string]string)
		for _, detail := range fullVariant.ProductDetails {
			if detail.Type != "" && detail.Value != "" {
				productDetailsMap[detail.Type] = detail.Value
			}
		}

		aiVariant := AmazonVariantForAI{
			Name:              fullVariant.Title,
			Asin:              fullVariant.Asin,
			Price:             fullVariant.FinalPrice,
			Image:             fullVariant.ImageURL,
			Attributes:        attributes,
			ProductDimensions: fullVariant.ProductDimensions,
			ItemWeight:        fullVariant.ItemWeight,
			Description:       fullVariant.Description,
			Features:          fullVariant.Features,
			ProductDetails:    productDetailsMap,
		}

		// 记录物流信息
		if fullVariant.ProductDimensions != "" || fullVariant.ItemWeight != "" {
			sb.logger.Infof("📦 变体[%d] ASIN=%s 物流信息: dimensions=%s, weight=%s",
				i, fullVariant.Asin, fullVariant.ProductDimensions, fullVariant.ItemWeight)
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少物流信息，AI将根据产品信息估算", i, fullVariant.Asin)
		}

		// 记录Description和Features信息
		if fullVariant.Description != "" {
			sb.logger.Infof("📝 变体[%d] ASIN=%s 有描述信息，长度: %d", i, fullVariant.Asin, len(fullVariant.Description))
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少描述信息", i, fullVariant.Asin)
		}
		if len(fullVariant.Features) > 0 {
			sb.logger.Infof("📝 变体[%d] ASIN=%s 有特性信息，数量: %d", i, fullVariant.Asin, len(fullVariant.Features))
		} else {
			sb.logger.Warnf("⚠️ 变体[%d] ASIN=%s 缺少特性信息", i, fullVariant.Asin)
		}

		aiVariants[i] = aiVariant
	}

	sb.logger.Infof("AI请求数据准备完成: 成功=%d, 失败=%d, 总计=%d", successCount, failedCount, len(aiVariants))

	productTitle := "Product"
	if ctx.AmazonProduct != nil {
		productTitle = ctx.AmazonProduct.Title
	}

	// 从上下文获取TEMU模板信息，优先使用goods_spec_properties
	var temuSpecProperties []GoodsSpecProperty
	if templateInfo, exists := GetTemplateInfoFromContext(ctx); exists {
		temuSpecProperties = templateInfo.GoodsSpecProperties
		sb.logger.Infof("成功获取TEMU模板信息，goods_spec_properties数量: %d", len(temuSpecProperties))

		// 如果goods_spec_properties为空，尝试使用user_input_parent_spec_list
		if len(temuSpecProperties) == 0 {
			if userInputSpecs, exists := GetUserInputParentSpecListFromContext(ctx); exists {
				sb.logger.Infof("goods_spec_properties为空，使用user_input_parent_spec_list，数量: %d", len(userInputSpecs))
				temuSpecProperties = sb.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
			} else {
				sb.logger.Warn("goods_spec_properties和user_input_parent_spec_list都为空")
			}
		}

	} else {
		sb.logger.Warn("未能从上下文获取TEMU模板信息")

		// 尝试直接获取user_input_parent_spec_list作为备选
		if userInputSpecs, exists := GetUserInputParentSpecListFromContext(ctx); exists {
			sb.logger.Infof("使用备选方案：user_input_parent_spec_list，数量: %d", len(userInputSpecs))
			temuSpecProperties = sb.convertUserInputSpecsToGoodsSpecProperties(userInputSpecs)
		}
	}

	request := VariantMappingRequest{
		ProductTitle:       productTitle,
		Variants:           aiVariants,
		Instructions:       sb.getAIInstructions(),
		TemuSpecProperties: temuSpecProperties,
	}

	// 构建AI提示
	prompt := sb.buildAIPrompt(request)

	sb.logger.Infof("不限制MaxTokens，允许AI自由生成完整响应 (变体数=%d)", len(variants))

	// 调用AI API
	aiCtx := context.Background()
	resp, err := sb.aiClient.CreateChatCompletion(aiCtx, &openai.ChatCompletionRequest{
		Model: sb.aiClient.GetDefaultModel(),
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "你是一个专业的电商产品数据转换专家，擅长将Amazon产品变体转换为TEMU平台的SKC/SKU结构。",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: func() *float32 { t := float32(0.1); return &t }(),
		MaxTokens:   nil, // 不限制输出token数量
	})

	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI响应为空")
	}

	// 解析AI响应
	var aiResponse AISkuMappingResponse
	content := resp.Choices[0].Message.Content

	// 提取JSON部分（去除可能的解释文本）
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}") + 1
	if jsonStart == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("AI响应中未找到有效JSON")
	}

	jsonContent := content[jsonStart:jsonEnd]

	// 检查响应是否被截断
	if resp.Choices[0].FinishReason == "length" {
		sb.logger.Errorf("❌ AI响应被截断（达到token限制），响应长度: %d字符", len(content))
		sb.logger.Errorf("❌ 建议：增加MaxTokens参数或减少变体数量")
		return nil, fmt.Errorf("AI响应被截断，请增加MaxTokens或减少变体数量")
	}

	if err := json.Unmarshal([]byte(jsonContent), &aiResponse); err != nil {
		// 提供更详细的错误信息
		sb.logger.Errorf("❌ JSON解析失败: %v", err)
		sb.logger.Errorf("❌ 响应长度: %d字符, JSON长度: %d字符", len(content), len(jsonContent))
		sb.logger.Errorf("❌ FinishReason: %s", resp.Choices[0].FinishReason)

		// 截取部分内容用于调试（避免日志过长）
		previewLen := 500
		if len(jsonContent) < previewLen {
			previewLen = len(jsonContent)
		}
		sb.logger.Errorf("❌ JSON开头: %s", jsonContent[:previewLen])

		// 显示JSON结尾（帮助诊断截断问题）
		if len(jsonContent) > previewLen {
			endStart := len(jsonContent) - previewLen
			if endStart < 0 {
				endStart = 0
			}
			sb.logger.Errorf("❌ JSON结尾: %s", jsonContent[endStart:])
		}

		return nil, fmt.Errorf("解析AI响应失败: %w, 响应内容长度: %d", err, len(jsonContent))
	}

	// 验证和修复AI响应，确保spec_id都来自TEMU模板
	sb.validateAndFixAIResponse(&aiResponse, temuSpecProperties)

	// 验证规格数量限制，确保每个SKU最多2个规格
	sb.enforceSpecCountLimit(&aiResponse)

	// 调试：打印asinToAttributes映射内容
	sb.logger.Infof("🔍 asinToAttributes映射内容 (共%d个):", len(asinToAttributes))
	for asin, attrs := range asinToAttributes {
		sb.logger.Infof("  ASIN: %s -> Attributes: %+v", asin, attrs)
	}

	// 填充VariantAttributes：将匹配到的Amazon attributes转换为string格式
	sb.logger.Infof("🔄 开始填充VariantAttributes，SKU数量: %d", len(aiResponse.SkuList))
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		sb.logger.Infof("🔍 处理SKU[%d]: UniqueID=%s, ASIN=%s", i, sku.UniqueID, sku.Asin)

		if attributes, exists := asinToAttributes[sku.Asin]; exists {
			sku.VariantAttributes = make(map[string]string)
			for key, value := range attributes {
				sku.VariantAttributes[key] = fmt.Sprintf("%v", value)
			}
			sb.logger.Infof("✅ 为SKU %s (ASIN: %s) 填充了VariantAttributes: %+v",
				sku.UniqueID, sku.Asin, sku.VariantAttributes)
		} else {
			sb.logger.Warnf("SKU %s (ASIN: %s) 未找到对应的attributes", sku.UniqueID, sku.Asin)
		}
	}

	sb.logger.Infof("AI成功生成%d个SKU映射", len(aiResponse.SkuList))
	return &aiResponse, nil
}

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

// validateAndFixAIResponse 验证和修复AI响应，处理临时spec_id
func (sb *SkuBuilder) validateAndFixAIResponse(aiResponse *AISkuMappingResponse, temuSpecProperties []GoodsSpecProperty) {
	// 构建parent_spec_id到可用spec_id的映射
	parentSpecToValidSpecIDs := make(map[string]map[string]PropertyValue)
	parentSpecExists := make(map[string]bool)
	parentSpecNames := make(map[string]string) // parent_spec_id -> parent_spec_name
	userInputSpecs := make(map[string]bool)    // 标记哪些是用户输入规格（无预定义值）

	for _, specProp := range temuSpecProperties {
		if specProp.ParentSpecID != "" {
			parentSpecExists[specProp.ParentSpecID] = true
			parentSpecNames[specProp.ParentSpecID] = specProp.Name // 保存parent_spec_name

			// 如果Values为空，说明这是用户输入规格，不需要验证spec_id
			if len(specProp.Values) == 0 {
				userInputSpecs[specProp.ParentSpecID] = true
				sb.logger.Debugf("识别到用户输入规格: %s (parent_spec_id: %s)，将接受任意spec_id",
					specProp.Name, specProp.ParentSpecID)
				continue
			}

			if parentSpecToValidSpecIDs[specProp.ParentSpecID] == nil {
				parentSpecToValidSpecIDs[specProp.ParentSpecID] = make(map[string]PropertyValue)
			}

			// 添加所有可用的spec_id
			for _, value := range specProp.Values {
				if value.SpecID != "" {
					parentSpecToValidSpecIDs[specProp.ParentSpecID][value.SpecID] = value
				}
			}
		}
	}

	sb.logger.Infof("构建了%d个parent_spec_id的有效spec_id映射，其中%d个为用户输入规格",
		len(parentSpecToValidSpecIDs), len(userInputSpecs))

	// 验证和修复每个SKU的spec
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		validSpecs := []types.SpecInfo{}

		for _, spec := range sku.Spec {
			// 检查parent_spec_id是否存在于模板中
			if !parentSpecExists[spec.ParentSpecID] {
				sb.logger.Warnf("SKU[%d] parent_spec_id %s 不存在于模板中，跳过该规格", i, spec.ParentSpecID)
				continue
			}

			// 如果是用户输入规格，直接接受AI提供的spec_id和spec_name
			if userInputSpecs[spec.ParentSpecID] {
				validSpecs = append(validSpecs, types.SpecInfo{
					SpecID:         spec.SpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: parentSpecNames[spec.ParentSpecID],
				})
				sb.logger.Debugf("✅ SKU[%d] 用户输入规格 parent_spec_id=%s, spec_id=%s, spec_name=%s",
					i, spec.ParentSpecID, spec.SpecID, spec.SpecName)
				continue
			}

			if validSpecIDs, exists := parentSpecToValidSpecIDs[spec.ParentSpecID]; exists {
				if _, specExists := validSpecIDs[spec.SpecID]; specExists {
					// spec_id有效，保留（使用AI提供的spec_name，不是模板中的值）
					validSpecs = append(validSpecs, types.SpecInfo{
						SpecID:         spec.SpecID,
						SpecName:       spec.SpecName, // 使用AI提供的具体值，不是模板中的规格维度名称
						ParentSpecID:   spec.ParentSpecID,
						ParentSpecName: parentSpecNames[spec.ParentSpecID],
					})
					sb.logger.Debugf("✅ SKU[%d] spec_id %s 验证通过，spec_name=%s", i, spec.SpecID, spec.SpecName)
				} else {
					// spec_id无效，尝试通过spec_name匹配
					matched := false
					for _, validValue := range validSpecIDs {
						if strings.EqualFold(validValue.Value, spec.SpecName) {
							validSpecs = append(validSpecs, types.SpecInfo{
								SpecID:         validValue.SpecID,
								SpecName:       validValue.Value,
								ParentSpecID:   spec.ParentSpecID,
								ParentSpecName: parentSpecNames[spec.ParentSpecID],
							})
							sb.logger.Infof("🔧 SKU[%d] 通过名称匹配修复: %s -> %s", i, spec.SpecID, validValue.SpecID)
							matched = true
							break
						}
					}
					if !matched {
						// 没有匹配的预定义值，创建临时ID，后续通过API解析
						tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
						validSpecs = append(validSpecs, types.SpecInfo{
							SpecID:         tempSpecID,
							SpecName:       spec.SpecName,
							ParentSpecID:   spec.ParentSpecID,
							ParentSpecName: parentSpecNames[spec.ParentSpecID],
						})
						sb.logger.Infof("🔧 SKU[%d] 创建临时spec_id: %s -> %s", i, spec.SpecName, tempSpecID)
					}
				}
			} else {
				// parent_spec_id存在但没有预定义值，创建临时ID
				tempSpecID := fmt.Sprintf("TEMP_%s", spec.SpecName)
				validSpecs = append(validSpecs, types.SpecInfo{
					SpecID:         tempSpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: parentSpecNames[spec.ParentSpecID],
				})
				sb.logger.Infof("🔧 SKU[%d] parent_spec_id %s 无预定义值，创建临时spec_id: %s", i, spec.ParentSpecID, tempSpecID)
			}
		}

		// 更新SKU的spec列表
		sku.Spec = validSpecs

		// 暂时不更新ColorSpecID和SpecID，等临时ID解析完成后再处理
	}

	sb.logger.Infof("AI响应验证完成，临时spec_id将在后续步骤中解析")
}

// enforceSpecCountLimit 强制执行规格数量限制（最多2个）
func (sb *SkuBuilder) enforceSpecCountLimit(aiResponse *AISkuMappingResponse) {
	// 1. 统计所有SKU使用的parent_spec_id
	parentSpecUsage := make(map[string]int)
	parentSpecNames := make(map[string]string)

	for _, sku := range aiResponse.SkuList {
		for _, spec := range sku.Spec {
			parentSpecUsage[spec.ParentSpecID]++
			parentSpecNames[spec.ParentSpecID] = spec.ParentSpecName
		}
	}

	// 2. 如果parent_spec_id数量<=2，无需处理
	if len(parentSpecUsage) <= 2 {
		sb.logger.Infof("✅ 规格维度数量符合要求: %d个", len(parentSpecUsage))
		return
	}

	// 3. 超过2个，需要选择最重要的2个
	sb.logger.Warnf("⚠️ 检测到%d个规格维度，超过TEMU限制(2个)，将自动选择最重要的2个", len(parentSpecUsage))

	// 按优先级排序：颜色 > 尺寸 > 其他
	type specPriority struct {
		parentSpecID   string
		parentSpecName string
		priority       int
		usage          int
	}

	var specs []specPriority
	for parentSpecID, usage := range parentSpecUsage {
		priority := sb.getSpecPriority(parentSpecNames[parentSpecID])
		specs = append(specs, specPriority{
			parentSpecID:   parentSpecID,
			parentSpecName: parentSpecNames[parentSpecID],
			priority:       priority,
			usage:          usage,
		})
	}

	// 排序：优先级高的在前，优先级相同时使用频率高的在前
	for i := 0; i < len(specs); i++ {
		for j := i + 1; j < len(specs); j++ {
			if specs[i].priority > specs[j].priority ||
				(specs[i].priority == specs[j].priority && specs[i].usage < specs[j].usage) {
				specs[i], specs[j] = specs[j], specs[i]
			}
		}
	}

	// 选择前2个
	selectedSpecs := make(map[string]bool)
	for i := 0; i < 2 && i < len(specs); i++ {
		selectedSpecs[specs[i].parentSpecID] = true
		sb.logger.Infof("✅ 选择规格维度[%d]: %s (parent_spec_id=%s, 使用次数=%d)",
			i+1, specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 记录被忽略的规格
	for i := 2; i < len(specs); i++ {
		sb.logger.Warnf("⚠️ 忽略规格维度: %s (parent_spec_id=%s, 使用次数=%d)",
			specs[i].parentSpecName, specs[i].parentSpecID, specs[i].usage)
	}

	// 4. 过滤每个SKU的spec，只保留选中的parent_spec_id
	for i := range aiResponse.SkuList {
		sku := &aiResponse.SkuList[i]
		filteredSpecs := []types.SpecInfo{}

		for _, spec := range sku.Spec {
			if selectedSpecs[spec.ParentSpecID] {
				filteredSpecs = append(filteredSpecs, spec)
			}
		}

		if len(filteredSpecs) != len(sku.Spec) {
			sb.logger.Infof("🔧 SKU[%d] 规格从%d个减少到%d个", i, len(sku.Spec), len(filteredSpecs))
		}

		sku.Spec = filteredSpecs
	}

	sb.logger.Infof("✅ 规格数量限制强制执行完成，所有SKU现在使用%d个规格维度", len(selectedSpecs))
}

// getSpecPriority 获取规格的优先级（数字越小优先级越高）
func (sb *SkuBuilder) getSpecPriority(specName string) int {
	specNameLower := strings.ToLower(specName)

	// 颜色相关：优先级1
	if sb.isColorSpec(specNameLower) {
		return 1
	}

	// 尺寸相关：优先级2
	if sb.isSizeSpec(specNameLower) {
		return 2
	}

	// 其他：优先级3
	return 3
}
