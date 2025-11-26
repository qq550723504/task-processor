package modules

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

// callGPTAPI 调用GPT API
func (h *SaleAttributeHandler) callGPTAPI(ctx *TaskContext, request *GenerationRequest) ResultSaleAttribute {
	// 检查变体数量，决定是否需要分批处理
	const maxVariantsPerBatch = 20
	variantCount := len(request.VariationData)

	if variantCount > maxVariantsPerBatch {
		logrus.Infof("🔄 变体数量(%d)超过单批限制(%d)，将分批处理", variantCount, maxVariantsPerBatch)
		return h.callGPTAPIInBatches(ctx, request, maxVariantsPerBatch)
	}

	// 单批处理
	return h.callGPTAPISingleBatch(ctx, request)
}

// callGPTAPIInBatches 分批调用GPT API
func (h *SaleAttributeHandler) callGPTAPIInBatches(ctx *TaskContext, request *GenerationRequest, batchSize int) ResultSaleAttribute {
	variationData := request.VariationData
	totalBatches := (len(variationData) + batchSize - 1) / batchSize
	logrus.Infof("📦 开始分批处理: 总变体数=%d, 批次大小=%d, 总批次=%d", len(variationData), batchSize, totalBatches)

	var allVariants []Variant

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variationData) {
			end = len(variationData)
		}

		// 创建当前批次的请求
		batchRequest := &GenerationRequest{
			ProductsData:             request.ProductsData[start:end],
			VariationData:            variationData[start:end],
			VariationAttributeValues: request.VariationAttributeValues,
			SaleAttributesData:       request.SaleAttributesData,
			AttributeMappings:        request.AttributeMappings,
			RequiredVariantCount:     end - start,
		}

		logrus.Infof("📦 处理批次 %d/%d: 变体[%d-%d]", batchIndex+1, totalBatches, start, end-1)

		// 处理当前批次
		batchResult := h.callGPTAPISingleBatch(ctx, batchRequest)

		allVariants = append(allVariants, batchResult.Variants...)
		logrus.Infof("✅ 批次 %d/%d 完成，生成%d个变体", batchIndex+1, totalBatches, len(batchResult.Variants))
	}

	logrus.Infof("✅ 所有批次处理完成，共生成%d个变体", len(allVariants))

	return ResultSaleAttribute{
		Variants: allVariants,
	}
}

// callGPTAPISingleBatch 单批次调用GPT API
func (h *SaleAttributeHandler) callGPTAPISingleBatch(ctx *TaskContext, request *GenerationRequest) ResultSaleAttribute {
	systemPrompt := h.generateSaleAttributeSystemPrompt()

	// 构建用户提示词
	userPrompt := h.buildUserPrompt(ctx, request)

	// 打印最终提交的提示词
	// logrus.Info("=" + strings.Repeat("=", 80))
	// logrus.Info("📝 最终提交的系统提示词 (System Prompt):")
	// logrus.Info("-" + strings.Repeat("-", 80))
	// logrus.Info(systemPrompt)
	// logrus.Info("=" + strings.Repeat("=", 80))
	// logrus.Info("📝 最终提交的用户提示词 (User Prompt):")
	// logrus.Info("-" + strings.Repeat("-", 80))
	// logrus.Info(userPrompt)
	// logrus.Info("=" + strings.Repeat("=", 80))

	req := h.createChatCompletionRequest(systemPrompt, userPrompt, len(request.ProductsData))

	response, err := h.openaiClient.CreateChatCompletion(ctx.Context, req)
	if err != nil {
		logrus.Errorf("❌ 调用GPT API失败: %v", err)
		return ResultSaleAttribute{}
	}

	if len(response.Choices) == 0 {
		logrus.Error("❌ GPT API响应为空")
		return ResultSaleAttribute{}
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)

	// 检查响应是否被截断
	if response.Choices[0].FinishReason == "length" {
		logrus.Warnf("⚠️ GPT响应被截断（达到token限制），响应长度: %d字符", len(content))
		logrus.Warn("⚠️ 尝试修复并解析部分JSON...")

		// 尝试修复被截断的JSON
		result := h.parseAndValidateJSON(content)
		if len(result.Variants) > 0 {
			logrus.Infof("✅ 成功从截断的响应中解析出%d个变体", len(result.Variants))
			return result
		}

		logrus.Error("❌ 无法从截断的响应中解析有效数据，建议增加MaxTokens")
		return ResultSaleAttribute{}
	}

	// 清理和验证JSON
	return h.parseAndValidateJSON(content)
}

// generateSaleAttributeSystemPrompt 生成销售属性系统提示词
func (h *SaleAttributeHandler) generateSaleAttributeSystemPrompt() string {
	return `你是电商产品变体与销售属性生成专家，专门分析Amazon产品数据并生成SHEIN平台所需的销售属性。你具备深度理解产品特征、变体差异和属性映射的能力。

	# 核心任务
	基于Amazon产品信息，智能生成SHEIN平台的销售属性（saleAttributes）和变体（variants）数据，确保属性选择准确、变体区分清晰。

	# 目标与核心规则
	- variants数组长度必须等于输入ASIN数量，且每个ASIN都必须有且仅有一个变体。
	- 所有Required=true的销售属性必须包含。
	- 主属性按重要性评分（备注+100，必填+80，示例+40，活跃+30，显示+20）选择，次要属性从剩余高分属性中选，均需在变体中有有效值。
	- 属性值和变体组合需唯一，变体属性不超过2项，且必须包含主属性。
	- 属性值ID优先从用户提供的平台变体数据中的选择，无法匹配时可用自定义值（id从-1开始递减，确保唯一性）。
	- 物理信息如无数据请合理估算（尺寸单位必须严格使用: cm，不允许使用其他单位如inch、Inch、Ft等，重量单位: g，范围0.01g-250000g）。
	- quantityType 为单品=1、同款多件=2、单套=3、多套=4
	- UnitType 单位类型 件=1，双=2，套=3
	- Quantity 数量，如果是多件或多套时，数量必须大于等于2。

	# 属性值严格保持原样规则（重要）
	**必须严格使用用户提供的原始属性值，不得进行任何修改、翻译或简化**：
	- 属性值的大小写、空格、标点符号都必须与原始数据完全一致
	- 禁止对属性值进行任何形式的"优化"、"标准化"或"翻译"

	# 属性名映射规则（重要）
	- 用户会在提示中提供【属性名称映射】，包含每个属性ID对应的variantAttributeName
	- 在variants的attributes字段中，必须严格使用映射中指定的variantAttributeName作为键名
	- 如果映射中没有某个属性，则使用"attr_[属性ID]"格式

	# 变体属性提取规则（关键）
	- 用户在【产品物理信息】中为每个ASIN提供了该变体的属性信息（如Color、Size等）
	- 必须从【产品物理信息】中提取每个ASIN对应的属性值，并填充到variants的attributes字段中
	- 如果【产品物理信息】中某个ASIN包含属性（如"Color": "Black"），则该ASIN的variant必须在attributes中包含该属性
	- 属性值必须与【产品物理信息】中提供的值完全一致，不得修改

	# 销售属性值列表生成规则（关键）
	**saleAttributes中的attrValue数组必须包含所有变体中出现的不同属性值**：
	- 用户在【⚠️ 重要：原始属性值列表】中提供了variations_values数据，这是所有属性值的完整列表
	- 必须使用这个列表中的值来生成saleAttributes，不要自己创造或简化
	- 例如：如果原始属性值列表中color的values是["Green Wire-Red", "Green Wire-Pink", "Green Wire-Yellow"]，则saleAttributes中Color属性的attrValue必须包含这3个值，不要简化为["Red", "Pink", "Yellow"]或合并为["Multi-Color"]
	- 每个变体的属性值都必须在对应的saleAttributes.attrValue列表中存在
	- 属性值的顺序、大小写、空格、标点符号都必须与原始数据完全一致

	# 特殊情况处理
	- 必填主属性在变体中为空，仍需按【变体属性值】生成。
	- 高分属性无效时，选次高分且有效的属性。
	- 仅有一个必填属性时，采用单属性分组。
	- 两个必填属性时，重要性高者为主，另一个为次要。

	# 尺寸单位规范（重要）
	variants中的lengthUnit字段必须严格使用：
	- "cm" - 厘米（SHEIN平台只接受cm作为长宽高单位）
	- 不允许使用 inch、Inch、ft、Ft 等其他单位

	# 输出格式
	返回JSON对象，包含saleAttributes和variants两部分。

	# 示例（假设原始属性值为["Black and Silver", "Gold"]和["Small", "Medium"]）
	{
	"saleAttributes": [
		{
		"attrId": 27,
		"attrValue": [
			{"id": 1, "value": "Black and Silver"},
			{"id": 2, "value": "Gold"}
		]
		},
		{
		"attrId": 87,
		"attrValue": [
			{"id": 1, "value": "Small"},
			{"id": 2, "value": "Medium"}
		]
		}
	],
	"variants": [
		{
		"attributes": {
			"Color": "Black and Silver",
			"Size": "Small"
		},
		"length": "25",
		"width": "15",
		"height": "10",
		"weight": "500",
		"lengthUnit": "cm",
		"asin": "B1234567890",
		"quantity": 1,
		"quantityType": 1,
		"quantity_unit": 1,
		},
		{
		"attributes": {
			"Color": "Gold",
			"Size": "Medium"
		},
		"length": "25",
		"width": "15",
		"height": "10",
		"weight": "500",
		"lengthUnit": "cm",
		"asin": "B1234567891",
		"quantity": 2,
		"quantityType": 2,
		"quantity_unit": 1,
		},
		{
		"attributes": {
			"Color": "Gold",
			"Size": "Small"
		},
		"length": "26",
		"width": "16",
		"height": "11",
		"weight": "520",
		"lengthUnit": "cm",
		"asin": "B1234567892",
		"quantity": 1,
		"quantityType": 1,
		"quantity_unit": 2,
		}
	]
	}

	⚠️ 重要提醒：
	1. 属性值必须与用户提供的原始数据完全一致，包括大小写、空格、标点符号
	2. 只返回JSON格式数据，不要输出任何解释或多余内容`
}

// parseAndValidateJSON 解析和验证JSON
func (h *SaleAttributeHandler) parseAndValidateJSON(content string) ResultSaleAttribute {
	logrus.Infof("📝 开始解析AI响应，长度: %d 字符", len(content))

	// 清理JSON格式
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		logrus.Debug("清理了markdown代码块标记")
	}
	content = strings.TrimSpace(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		logrus.Warn("⚠️ JSON格式无效，尝试修复...")

		// 尝试修复常见的JSON问题
		fixedContent := h.fixCommonJsonIssues(content)
		if !json.Valid([]byte(fixedContent)) {
			logrus.Error("❌ JSON修复失败，无法解析")
			logrus.Debugf("原始内容前500字符: %s", content[:min(500, len(content))])
			return ResultSaleAttribute{}
		}
		logrus.Info("✅ JSON修复成功")
		content = fixedContent
	} else {
		logrus.Debug("✅ JSON格式有效")
	}

	var saleAttributeData ResultSaleAttribute
	if err := json.Unmarshal([]byte(content), &saleAttributeData); err != nil {
		logrus.Errorf("❌ JSON解析失败: %v", err)
		logrus.Debugf("内容前500字符: %s", content[:min(500, len(content))])
		return ResultSaleAttribute{}
	}

	logrus.Infof("✅ 成功解析AI响应 - 销售属性: %d 个, 变体: %d 个",
		len(saleAttributeData.SaleAttributes), len(saleAttributeData.Variants))

	return saleAttributeData
}

// fixCommonJsonIssues 修复常见的JSON问题
func (h *SaleAttributeHandler) fixCommonJsonIssues(content string) string {
	original := content
	logrus.Infof("🔧 开始修复JSON，原始长度: %d", len(content))

	// 1. 移除尾部的无效内容（在最后一个有效结构后的说明文字）
	content = removeTrailingExplanation(content)

	// 2. 修复被截断的JSON对象（移除最后一个不完整的对象）
	content = h.removeIncompleteLastObject(content)

	// 3. 修复尾部缺失的中括号
	openBrackets := strings.Count(content, "[")
	closeBrackets := strings.Count(content, "]")
	if openBrackets > closeBrackets {
		missing := openBrackets - closeBrackets
		logrus.Infof("🔧 修复缺失的%d个中括号", missing)
		// 在最后一个 } 之前添加缺失的 ]
		lastBraceIndex := strings.LastIndex(content, "}")
		if lastBraceIndex > 0 {
			content = content[:lastBraceIndex] + strings.Repeat("]", missing) + content[lastBraceIndex:]
		} else {
			content = content + strings.Repeat("]", missing)
		}
	}

	// 4. 修复尾部缺失的大括号
	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces > closeBraces {
		missing := openBraces - closeBraces
		logrus.Infof("🔧 修复缺失的%d个大括号", missing)
		content = content + strings.Repeat("}", missing)
	}

	// 4. 修复双引号问题
	content = strings.ReplaceAll(content, `\"`, `"`)

	// 5. 确保JSON以大括号开始和结束
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		logrus.Warnf("JSON不以{开头，添加开头大括号")
		content = "{" + content
	}
	if !strings.HasSuffix(content, "}") {
		logrus.Warnf("JSON不以}结尾，添加结尾大括号")
		content = content + "}"
	}

	if content != original {
		logrus.Infof("✅ JSON修复完成，原始长度: %d, 修复后长度: %d", len(original), len(content))
	} else {
		logrus.Debug("JSON无需修复")
	}

	return content
}

// removeIncompleteLastObject 移除被截断的最后一个不完整对象
func (h *SaleAttributeHandler) removeIncompleteLastObject(content string) string {
	// 查找variants数组的最后一个逗号位置
	// 如果JSON被截断，最后一个对象可能不完整，需要移除

	// 查找"variants"数组
	variantsIndex := strings.Index(content, `"variants"`)
	if variantsIndex == -1 {
		return content
	}

	// 从variants位置开始查找
	afterVariants := content[variantsIndex:]

	// 查找最后一个完整的对象结束位置（},）
	lastCompleteObjectPattern := regexp.MustCompile(`\},\s*\{[^}]*$`)
	if match := lastCompleteObjectPattern.FindStringIndex(afterVariants); match != nil {
		// 找到了不完整的最后一个对象，截断到最后一个完整对象
		cutPosition := variantsIndex + match[0] + 1 // +1保留}
		logrus.Infof("🔧 检测到不完整的最后一个对象，截断位置: %d", cutPosition)
		content = content[:cutPosition] + "\n]}"
	}

	return content
}

// removeTrailingExplanation 移除JSON后的说明文字
func removeTrailingExplanation(content string) string {
	// 查找可能的说明文字开始位置
	// 通常说明文字以 "### " 或 "**" 或换行符+中文开始
	patterns := []string{
		"\n###",
		"\n**",
		"\n\n1.",
		"\n\n-",
		"```\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			// 检查这个位置之前是否有完整的JSON结构
			beforePattern := content[:idx]
			if looksLikeCompleteJson(beforePattern) {
				logrus.Infof("检测到说明文字开始于位置%d，移除后续内容", idx)
				return beforePattern
			}
		}
	}

	return content
}
