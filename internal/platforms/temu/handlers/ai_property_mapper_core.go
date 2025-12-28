// Package handlers 提供TEMU平台的AI属性映射核心功能
package handlers

import (
	"context"
	"fmt"
	"time"

	openaiClient "task-processor/internal/clients/openai"
	"task-processor/internal/config"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// AIPropertyMapper AI属性映射器
type AIPropertyMapper struct {
	logger       *logrus.Entry
	openaiClient *openaiClient.Client

	// 注入的专职处理器
	aiService         *AIService
	propertyValidator *PropertyValidator
	defaultFiller     *DefaultPropertyFiller

	// 新增的严格验证组件
	valueValidator     *PropertyValueValidator
	valueFixer         *PropertyValueFixer
	statsCollector     *PropertyValidationStatsCollector
	deduplicator       *PropertyDeduplicator
	selectionValidator *PropertySelectionValidator
}

// NewAIPropertyMapper 创建新的AI属性映射器
func NewAIPropertyMapper(logger *logrus.Entry, openaiClient *openaiClient.Client, openaiConfig *openaiClient.ClientConfig) *AIPropertyMapper {
	// 创建专职处理器
	// 将ClientConfig转换为config.OpenAIConfig
	configOpenAI := &config.OpenAIConfig{
		APIKey:  openaiConfig.APIKey,
		Model:   openaiConfig.Model,
		BaseURL: openaiConfig.BaseURL,
		Timeout: int(openaiConfig.Timeout.Seconds()),
	}

	aiService := NewAIService(openaiClient, configOpenAI, logger)
	propertyValidator := NewPropertyValidator(logger)
	defaultFiller := NewDefaultPropertyFiller(logger)

	// 创建新的严格验证组件
	valueValidator := NewPropertyValueValidator(logger)
	valueFixer := NewPropertyValueFixer(logger)
	statsCollector := NewPropertyValidationStatsCollector(logger)
	deduplicator := NewPropertyDeduplicator(logger)
	selectionValidator := NewPropertySelectionValidator(logger)

	return &AIPropertyMapper{
		logger:             logger,
		openaiClient:       openaiClient,
		aiService:          aiService,
		propertyValidator:  propertyValidator,
		defaultFiller:      defaultFiller,
		valueValidator:     valueValidator,
		valueFixer:         valueFixer,
		statsCollector:     statsCollector,
		deduplicator:       deduplicator,
		selectionValidator: selectionValidator,
	}
}

// Name 返回处理器名称
func (m *AIPropertyMapper) Name() string {
	return "AI属性映射器"
}

// HandleTemu 处理任务（强类型上下文）
func (m *AIPropertyMapper) HandleTemu(temuCtx *temucontext.TemuTaskContext) error {
	// 检查TEMU产品信息
	if temuCtx.TemuProduct == nil {
		return fmt.Errorf("TEMU产品信息为空")
	}

	// 构建商品属性
	return m.BuildGoodsProperties(temuCtx, &temuCtx.TemuProduct.GoodsExtensionInfo)
}

// BuildGoodsProperties 构建商品属性（使用AI智能映射）
func (m *AIPropertyMapper) BuildGoodsProperties(temuCtx *temucontext.TemuTaskContext, ext *types.ExtensionInfo) error {
	// 获取模板信息
	templateInfo, exists := GetTemplateInfoFromContext(temuCtx)
	if !exists {
		m.logger.Warn("未找到模板信息，跳过属性构建")
		return nil
	}

	m.logger.WithFields(logrus.Fields{
		"templateID":           templateInfo.TemplateID,
		"goodsPropertiesCount": len(templateInfo.GoodsProperties),
		"specPropertiesCount":  len(templateInfo.GoodsSpecProperties),
	}).Info("使用AI智能映射商品属性")

	// 调用AI进行属性映射 - 使用传入的context，添加超时控制
	aiCtx, cancel := context.WithTimeout(temuCtx.GetContext(), 60*time.Second)
	defer cancel()

	mappingData := preparePropertyMappingData(temuCtx, templateInfo.GoodsProperties)

	mappedProperties, err := m.aiService.CallAIForPropertyMapping(aiCtx, mappingData)
	if err != nil {
		m.logger.WithError(err).Error("❌ AI属性映射失败，使用默认值填充所有必填属性")
		m.defaultFiller.FillRequiredPropertiesWithDefaults(templateInfo.GoodsProperties, ext)
		// 再次验证确保所有必填属性都已填充
		m.verifyRequiredProperties(templateInfo.GoodsProperties, ext)
		m.logger.Infof("✅ 默认属性填充完成，共处理 %d 个属性", len(ext.GoodsProperty.GoodsProperties))
		return nil
	}

	// 应用AI映射的结果并进行严格验证
	m.logger.Infof("📝 AI映射返回 %d 个属性，开始严格验证", len(mappedProperties))

	// 启动统计收集
	m.statsCollector.StartValidation(len(mappedProperties))

	// 使用新的严格验证和修复系统
	validatedProperties := m.valueFixer.FixAllInvalidProperties(mappedProperties, templateInfo.GoodsProperties)

	// 最终验证确保所有属性都有效
	finalProperties := make([]types.PropertyItem, 0, len(validatedProperties))
	for _, prop := range validatedProperties {
		// 查找模板属性
		var templateProp *types.TemplateRespGoodsProperty
		for _, tmpl := range templateInfo.GoodsProperties {
			if tmpl.PID == prop.Pid {
				templateProp = &tmpl
				break
			}
		}

		if templateProp == nil {
			m.statsCollector.RecordSkippedProperty("unknown")
			continue
		}

		// 对选择类型属性进行最终验证
		if templateProp.PropertyValueType == 1 {

			isValid, validVID, validValue, err := m.valueValidator.ValidateSelectionValue(prop, *templateProp)
			if !isValid {
				m.logger.Errorf("❌ 最终验证失败: PID=%d, Error=%v", prop.Pid, err)
				m.statsCollector.RecordSkippedProperty("selection")
				continue
			}

			// 确保使用验证通过的值
			prop.Vid = validVID
			prop.Value = validValue
		}

		finalProperties = append(finalProperties, prop)
		m.statsCollector.RecordValidProperty(getPropertyTypeName(templateProp.PropertyValueType))
	}

	// 完成统计并生成报告
	m.statsCollector.FinishValidation()

	// 应用最终验证的属性，并进行选择约束验证和去重处理
	allProperties := append(ext.GoodsProperty.GoodsProperties, finalProperties...)

	// 先验证选择约束（单选/多选）
	constraintValidatedProperties := m.selectionValidator.ValidateSelectionConstraints(allProperties, templateInfo.GoodsProperties)

	// 再进行去重处理
	ext.GoodsProperty.GoodsProperties = m.deduplicator.DeduplicateByPidOnly(constraintValidatedProperties)

	m.logger.Infof("📝 选择约束验证、去重完成，最终属性数: %d", len(ext.GoodsProperty.GoodsProperties))

	// 验证属性约束
	if err := m.validatePropertyConstraints(templateInfo.GoodsProperties, ext); err != nil {
		m.logger.WithError(err).Error("❌ 属性约束验证失败")
		return fmt.Errorf("属性约束验证失败: %w", err)
	}

	// 验证属性值有效性
	if err := m.validatePropertyValues(templateInfo.GoodsProperties, ext); err != nil {
		m.logger.WithError(err).Error("❌ 属性值验证失败")
		return fmt.Errorf("属性值验证失败: %w", err)
	}

	// 统计映射结果
	mappedRequired := 0
	mappedOptional := 0
	for _, mappedProp := range mappedProperties {
		isRequired := m.isRequiredProperty(int64(mappedProp.Pid), templateInfo.GoodsProperties)
		if isRequired {
			mappedRequired++
		} else {
			mappedOptional++
		}
	}

	// 检查是否所有必填属性都已填充，如果有遗漏则补充默认值
	missingRequired := m.verifyAndFillMissingRequired(templateInfo.GoodsProperties, ext)
	if missingRequired > 0 {
		m.logger.Warnf("⚠️ AI遗漏了%d个必填属性，已用默认值补充", missingRequired)
	}

	m.logger.Infof("✅ AI属性映射完成，共处理 %d 个属性（必填=%d, 可选=%d）",
		len(ext.GoodsProperty.GoodsProperties), mappedRequired, mappedOptional)
	return nil
}

// getPropertyTypeName 获取属性类型名称
func getPropertyTypeName(propertyValueType int) string {
	switch propertyValueType {
	case 1:
		return "selection"
	case 2:
		return "numeric"
	case 3:
		return "text"
	default:
		return "unknown"
	}
}

func preparePropertyMappingData(temuCtx *temucontext.TemuTaskContext, templateProps []types.TemplateRespGoodsProperty) types.PropertyMappingData {
	data := types.PropertyMappingData{
		TemuProperties: make([]types.TemplateRespGoodsProperty, 0, len(templateProps)),
	}

	// 组织Amazon产品数据
	if temuCtx.GetAmazonProduct() != nil {
		data.AmazonProduct = convertAmazonProductData(temuCtx)
	}

	// 组织TEMU属性选项
	for _, templateProp := range templateProps {
		data.TemuProperties = append(data.TemuProperties, templateProp)
	}

	return data
}

// convertAmazonProductData 将Amazon产品数据转换为AI映射所需的格式
func convertAmazonProductData(temuCtx *temucontext.TemuTaskContext) types.AmazonProductData {
	amazonProduct := temuCtx.GetAmazonProduct()
	if amazonProduct == nil {
		return types.AmazonProductData{}
	}

	// 转换产品详情
	productDetails := make([]types.ProductDetailData, 0, len(amazonProduct.ProductDetails))
	for _, detail := range amazonProduct.ProductDetails {
		productDetails = append(productDetails, types.ProductDetailData{
			Type:  detail.Type,
			Value: detail.Value,
		})
	}

	return types.AmazonProductData{
		Title:             amazonProduct.Title,
		Brand:             amazonProduct.Brand,
		Description:       amazonProduct.Description,
		Features:          amazonProduct.Features,
		ProductDetails:    productDetails,
		ProductDimensions: amazonProduct.ProductDimensions,
		ItemWeight:        amazonProduct.ItemWeight,
		ModelNumber:       amazonProduct.ModelNumber,
		Department:        amazonProduct.Department,
		Manufacturer:      amazonProduct.Manufacturer,
		Categories:        amazonProduct.Categories,
	}
}
