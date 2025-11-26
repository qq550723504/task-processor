package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/common/pipeline"
	openaiClient "task-processor/openai"
	"task-processor/platforms/temu/types"

	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

// AIPropertyMapper AI属性映射器
type AIPropertyMapper struct {
	logger        *logrus.Entry
	openaiClient  *openaiClient.Client
	promptBuilder *PromptBuilder
}

// NewAIPropertyMapper 创建新的AI属性映射器
func NewAIPropertyMapper(logger *logrus.Entry, openaiClient *openaiClient.Client) *AIPropertyMapper {
	return &AIPropertyMapper{
		logger:        logger,
		openaiClient:  openaiClient,
		promptBuilder: NewPromptBuilder(),
	}
}

// BuildGoodsProperties 构建商品属性（使用AI智能映射）
func (m *AIPropertyMapper) BuildGoodsProperties(ctx *pipeline.TaskContext, ext *types.ExtensionInfo) error {
	// 获取模板信息
	templateInfo, exists := GetTemplateInfoFromContext(ctx)
	if !exists {
		m.logger.Warn("未找到模板信息，跳过属性构建")
		return nil
	}

	m.logger.WithFields(logrus.Fields{
		"templateID":           templateInfo.TemplateID,
		"goodsPropertiesCount": len(templateInfo.GoodsProperties),
		"specPropertiesCount":  len(templateInfo.GoodsSpecProperties),
	}).Info("使用AI智能映射商品属性")

	// 记录所有必填属性（包括详细信息）
	requiredProps := []string{}
	for _, prop := range templateInfo.GoodsProperties {
		if prop.Required {
			requiredProps = append(requiredProps, fmt.Sprintf("%s(PID=%d,Type=%d)", prop.Name, prop.PID, prop.PropertyValueType))
		}
	}
	m.logger.Infof("📋 模板中的必填属性(%d个): %v", len(requiredProps), requiredProps)

	// 组织数据供AI处理
	mappingData := m.preparePropertyMappingData(ctx, templateInfo.GoodsProperties)

	// 调用AI进行属性映射
	mappedProperties, err := m.callAIForPropertyMapping(mappingData)
	if err != nil {
		m.logger.WithError(err).Error("❌ AI属性映射失败，使用默认值填充所有必填属性")
		m.fillRequiredPropertiesWithDefaults(templateInfo.GoodsProperties, ext)
		// 再次验证确保所有必填属性都已填充
		m.verifyRequiredProperties(templateInfo.GoodsProperties, ext)
		m.logger.Infof("✅ 默认属性填充完成，共处理 %d 个属性", len(ext.GoodsProperty.GoodsProperties))
		return nil
	}

	// 应用AI映射的结果
	ext.GoodsProperty.GoodsProperties = append(ext.GoodsProperty.GoodsProperties, mappedProperties...)
	m.logger.Infof("📝 AI映射返回 %d 个属性", len(mappedProperties))

	// 检查是否所有必填属性都已填充
	m.verifyRequiredProperties(templateInfo.GoodsProperties, ext)

	m.logger.Infof("✅ AI属性映射完成，共处理 %d 个属性", len(ext.GoodsProperty.GoodsProperties))
	return nil
}

// verifyRequiredProperties 验证所有必填属性是否都已填充
func (m *AIPropertyMapper) verifyRequiredProperties(templateProps []GoodsProperty, ext *types.ExtensionInfo) {
	m.logger.Info("🔍 开始验证必填属性是否都已填充")

	// 创建已填充属性的映射（使用 PID+RefPID 作为唯一标识）
	filledMap := make(map[string]types.PropertyItem)
	filledDetails := []string{}
	for _, prop := range ext.GoodsProperty.GoodsProperties {
		key := fmt.Sprintf("%d_%d", prop.Pid, prop.RefPid)
		filledMap[key] = prop
		filledDetails = append(filledDetails, fmt.Sprintf("PID=%d,RefPID=%d,VID=%d,Value=%s", prop.Pid, prop.RefPid, prop.Vid, prop.Value))
	}
	m.logger.Infof("📝 当前已填充 %d 个属性: %v", len(ext.GoodsProperty.GoodsProperties), filledDetails)

	// 检查每个必填属性（使用 PID+RefPID 匹配）
	missingRequired := []string{}
	for _, templateProp := range templateProps {
		if templateProp.Required {
			key := fmt.Sprintf("%d_%d", templateProp.PID, templateProp.RefPID)
			if _, found := filledMap[key]; !found {
				missingRequired = append(missingRequired,
					fmt.Sprintf("%s(PID=%d,RefPID=%d,Type=%d)", templateProp.Name, templateProp.PID, templateProp.RefPID, templateProp.PropertyValueType))
				m.logger.Warnf("⚠️ 缺失必填属性: %s (PID=%d, RefPID=%d, Type=%d)", templateProp.Name, templateProp.PID, templateProp.RefPID, templateProp.PropertyValueType)
			}
		}
	}

	if len(missingRequired) > 0 {
		m.logger.Errorf("❌ 发现 %d 个缺失的必填属性: %v", len(missingRequired), missingRequired)
		m.logger.Info("🔧 尝试补充缺失的必填属性")

		// 补充缺失的必填属性（使用 PID+RefPID 匹配）
		for _, templateProp := range templateProps {
			if templateProp.Required {
				key := fmt.Sprintf("%d_%d", templateProp.PID, templateProp.RefPID)
				if _, found := filledMap[key]; !found {
					m.fillSingleRequiredProperty(templateProp, ext)
				}
			}
		}

		// 再次验证补充后的结果（使用 PID+RefPID 匹配）
		m.logger.Info("🔍 验证补充后的属性")
		stillMissing := []string{}
		for _, templateProp := range templateProps {
			if templateProp.Required {
				found := false
				for _, prop := range ext.GoodsProperty.GoodsProperties {
					if prop.Pid == templateProp.PID && prop.RefPid == templateProp.RefPID {
						found = true
						m.logger.Infof("✅ 必填属性已填充: %s (PID=%d, RefPID=%d, VID=%d, Value=%s)",
							templateProp.Name, prop.Pid, prop.RefPid, prop.Vid, prop.Value)
						break
					}
				}
				if !found {
					stillMissing = append(stillMissing, fmt.Sprintf("%s(PID=%d,RefPID=%d)", templateProp.Name, templateProp.PID, templateProp.RefPID))
					m.logger.Errorf("❌ 必填属性仍然缺失: %s (PID=%d, RefPID=%d)", templateProp.Name, templateProp.PID, templateProp.RefPID)
				}
			}
		}

		if len(stillMissing) > 0 {
			m.logger.Errorf("❌❌❌ 警告：仍有 %d 个必填属性未能填充: %v", len(stillMissing), stillMissing)
		} else {
			m.logger.Info("✅ 所有缺失的必填属性已成功补充")
		}
	} else {
		m.logger.Info("✅ 所有必填属性都已正确填充")
	}
}

// fillSingleRequiredProperty 填充单个必填属性
func (m *AIPropertyMapper) fillSingleRequiredProperty(templateProp GoodsProperty, ext *types.ExtensionInfo) {
	m.logger.Infof("🔧 补充必填属性: %s (PID=%d, RefPID=%d, TemplatePID=%d, Type=%d)",
		templateProp.Name, templateProp.PID, templateProp.RefPID, templateProp.TemplatePID, templateProp.PropertyValueType)

	propertyItem := types.PropertyItem{
		RefPid:      templateProp.RefPID,
		Pid:         templateProp.PID,
		TemplatePid: templateProp.TemplatePID,
	}

	hasValue := false

	// 优先检查是否有可选值列表（无论类型是什么）
	if len(templateProp.Values) > 0 {
		selectedValue := m.selectBestDefaultValue(templateProp)
		propertyItem.Value = selectedValue.Value
		propertyItem.Vid = selectedValue.VID
		hasValue = true
		m.logger.Infof("✅ 从可选值列表中选择默认值: %s (VID=%d)", selectedValue.Value, selectedValue.VID)
	} else {
		// 没有可选值时，根据类型设置默认值
		switch templateProp.PropertyValueType {
		case 2: // 数字类型
			if templateProp.MinValue != "" {
				propertyItem.Value = templateProp.MinValue
			} else {
				propertyItem.Value = "1"
			}
			hasValue = true
			m.logger.Infof("✅ 数字类型属性使用默认值: %s", propertyItem.Value)
		default: // 文本类型或其他
			propertyItem.Value = "Not specified"
			hasValue = true
			m.logger.Warnf("⚠️ 文本类型属性使用默认值: %s (可能无效)", propertyItem.Value)
		}
	}

	if hasValue {
		if len(templateProp.ValueUnit) > 0 {
			propertyItem.ValueUnit = templateProp.ValueUnit[0]
			m.logger.Infof("📏 设置单位: %s", propertyItem.ValueUnit)
		}
		ext.GoodsProperty.GoodsProperties = append(ext.GoodsProperty.GoodsProperties, propertyItem)
		m.logger.Infof("✅ 成功补充必填属性: %s (PID=%d, VID=%d, Value=%s)",
			templateProp.Name, propertyItem.Pid, propertyItem.Vid, propertyItem.Value)
	} else {
		m.logger.Errorf("❌❌❌ 无法为必填属性 %s (PID=%d) 设置值", templateProp.Name, templateProp.PID)
	}
}

// callAIForPropertyMapping 调用AI进行属性映射
func (m *AIPropertyMapper) callAIForPropertyMapping(data PropertyMappingData) ([]types.PropertyItem, error) {
	m.logger.Info("准备调用AI进行属性映射")

	// 检查AI客户端是否可用
	if m.openaiClient == nil {
		m.logger.Warn("OpenAI客户端未配置，返回空结果")
		return []types.PropertyItem{}, nil
	}

	// 构建提示词
	systemPrompt := m.promptBuilder.BuildSystemPrompt()
	userPrompt := m.promptBuilder.BuildUserPrompt(data)

	m.logger.Debugf("系统提示词长度: %d 字符", len(systemPrompt))
	m.logger.Debugf("用户提示词长度: %d 字符", len(userPrompt))

	// 创建请求并调用API
	req := m.createChatCompletionRequest(systemPrompt, userPrompt)
	ctx := context.Background()
	response, err := m.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		m.logger.WithError(err).Error("调用OpenAI API失败")
		return nil, fmt.Errorf("调用AI服务失败: %w", err)
	}

	if len(response.Choices) == 0 {
		m.logger.Error("AI响应为空")
		return nil, fmt.Errorf("AI响应为空")
	}

	// 处理AI响应
	return m.processAIResponse(response, data)
}

// createChatCompletionRequest 创建聊天完成请求
func (m *AIPropertyMapper) createChatCompletionRequest(systemPrompt, userPrompt string) *openaiClient.ChatCompletionRequest {
	seed := 42
	temperature := float32(0.1)

	return &openaiClient.ChatCompletionRequest{
		Model: m.openaiClient.GetDefaultModel(),
		Messages: []openaiClient.ChatCompletionMessage{
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
func (m *AIPropertyMapper) processAIResponse(response *openaiClient.ChatCompletionResponse, data PropertyMappingData) ([]types.PropertyItem, error) {
	content := strings.TrimSpace(response.Choices[0].Message.Content)

	// 清理JSON格式
	content = m.cleanJSONContent(content)

	// 验证JSON格式
	if !json.Valid([]byte(content)) {
		m.logger.Errorf("AI返回的JSON格式无效: %s", content)
		return nil, fmt.Errorf("AI返回的JSON格式无效")
	}

	// 解析AI响应
	var aiResponse struct {
		Properties []types.PropertyItem `json:"properties"`
	}

	if err := json.Unmarshal([]byte(content), &aiResponse); err != nil {
		m.logger.WithError(err).Errorf("解析AI响应失败: %s", content)
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	// 使用属性验证器验证和修复属性值
	validator := NewPropertyValidator(m.logger)
	validatedProperties := validator.ValidateAndFixProperties(aiResponse.Properties, data)

	m.logger.Infof("AI属性映射成功，返回 %d 个属性", len(validatedProperties))
	return validatedProperties, nil
}

// cleanJSONContent 清理JSON内容
func (m *AIPropertyMapper) cleanJSONContent(content string) string {
	// 移除markdown代码块标记
	if after, found := strings.CutPrefix(content, "```json"); found {
		content = strings.TrimSuffix(after, "```")
	} else if after, found := strings.CutPrefix(content, "```"); found {
		content = strings.TrimSuffix(after, "```")
	}

	return strings.TrimSpace(content)
}

// preparePropertyMappingData 准备属性映射数据
func (m *AIPropertyMapper) preparePropertyMappingData(ctx *pipeline.TaskContext, templateProps []GoodsProperty) PropertyMappingData {
	converter := NewDataConverter()
	return converter.PreparePropertyMappingData(ctx, templateProps)
}

// fillRequiredPropertiesWithDefaults 用默认值填充必填属性
func (m *AIPropertyMapper) fillRequiredPropertiesWithDefaults(templateProps []GoodsProperty, ext *types.ExtensionInfo) {
	m.logger.Info("🔧 开始用默认值填充必填属性")

	for _, templateProp := range templateProps {
		if !templateProp.Required {
			continue
		}

		m.logger.Infof("处理必填属性: %s (PID=%d, RefPID=%d, Type=%d)",
			templateProp.Name, templateProp.PID, templateProp.RefPID, templateProp.PropertyValueType)

		propertyItem := types.PropertyItem{
			RefPid:      templateProp.RefPID,
			Pid:         templateProp.PID,
			TemplatePid: templateProp.TemplatePID,
		}

		hasValue := false

		// 优先检查是否有可选值列表（无论类型是什么）
		if len(templateProp.Values) > 0 {
			selectedValue := m.selectBestDefaultValue(templateProp)
			propertyItem.Value = selectedValue.Value
			propertyItem.Vid = selectedValue.VID
			hasValue = true
			m.logger.Infof("✅ 属性 %s 从可选值列表中选择默认值: %s (VID=%d)",
				templateProp.Name, selectedValue.Value, selectedValue.VID)
		} else {
			// 没有可选值时，根据类型设置默认值
			switch templateProp.PropertyValueType {
			case 2: // 数字类型
				if templateProp.MinValue != "" {
					propertyItem.Value = templateProp.MinValue
				} else {
					propertyItem.Value = "1"
				}
				hasValue = true
				m.logger.Infof("✅ 数字类型属性 %s 使用默认值: %s", templateProp.Name, propertyItem.Value)
			default: // 文本类型或其他
				m.logger.Errorf("❌ 必填属性 %s (PID=%d, Type=%d) 没有可选值且不是数字类型，无法填充",
					templateProp.Name, templateProp.PID, templateProp.PropertyValueType)
			}
		}

		// 只添加有值的属性，避免 "关键词属性值ID和关键词属性值不能同时为空" 错误
		if !hasValue {
			m.logger.Errorf("❌ 必填属性 %s 无法设置默认值，跳过", templateProp.Name)
			continue
		}

		// 设置单位
		if len(templateProp.ValueUnit) > 0 {
			propertyItem.ValueUnit = templateProp.ValueUnit[0]
		}

		ext.GoodsProperty.GoodsProperties = append(ext.GoodsProperty.GoodsProperties, propertyItem)
		m.logger.Infof("✅ 成功填充必填属性: %s = %s (VID=%d)",
			templateProp.Name, propertyItem.Value, propertyItem.Vid)
	}

	m.logger.Infof("🎉 必填属性填充完成，共填充 %d 个属性", len(ext.GoodsProperty.GoodsProperties))
}

// selectBestDefaultValue 选择最佳的默认值（优先选择英文中性选项，避免中文）
func (m *AIPropertyMapper) selectBestDefaultValue(templateProp GoodsProperty) PropertyValue {
	// 优先选择的英文关键词（按优先级排序）
	englishNeutralKeywords := []string{
		"Other", "N/A", "None", "Not Applicable", "No", "Without",
		"Mixed", "General", "Universal", "Standard",
	}

	// 中文关键词作为备选
	chineseNeutralKeywords := []string{
		"其他", "其它", "不适用", "无需", "混合", "通用",
	}

	// 首先尝试找到包含英文中性关键词且不含中文的选项
	for _, keyword := range englishNeutralKeywords {
		for _, value := range templateProp.Values {
			if strings.Contains(value.Value, keyword) {
				// 检查是否包含中文字符
				hasChinese := false
				for _, r := range value.Value {
					if r >= 0x4e00 && r <= 0x9fff {
						hasChinese = true
						break
					}
				}

				if !hasChinese {
					m.logger.Debugf("找到英文中性选项: %s (VID=%d)", value.Value, value.VID)
					return value
				}
			}
		}
	}

	// 如果没有找到英文选项，尝试找不含中文的第一个可选值
	for _, value := range templateProp.Values {
		hasChinese := false
		for _, r := range value.Value {
			if r >= 0x4e00 && r <= 0x9fff {
				hasChinese = true
				break
			}
		}

		if !hasChinese {
			m.logger.Debugf("找到不含中文的选项: %s (VID=%d)", value.Value, value.VID)
			return value
		}
	}

	// 如果所有选项都包含中文，尝试找中文中性选项
	for _, keyword := range chineseNeutralKeywords {
		for _, value := range templateProp.Values {
			if strings.Contains(value.Value, keyword) {
				m.logger.Warnf("⚠️ 只找到中文中性选项: %s (VID=%d)", value.Value, value.VID)
				return value
			}
		}
	}

	// 最后的备选：返回第一个可选值（即使包含中文）
	m.logger.Warnf("⚠️ 未找到合适选项，使用第一个可选值: %s (VID=%d)",
		templateProp.Values[0].Value, templateProp.Values[0].VID)
	return templateProp.Values[0]
}
