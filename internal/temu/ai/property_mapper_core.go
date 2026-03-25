// Package ai 提供TEMU平台的AI属性映射核心功能
package ai

import (
	"fmt"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/timeout"
	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/property"

	"github.com/sirupsen/logrus"
)

// AIPropertyMapper AI属性映射器
type AIPropertyMapper struct {
	logger       *logrus.Entry
	openaiClient openaiClient.ChatCompleter

	// 注入的专职处理器
	aiService         *AIService
	propertyValidator *property.PropertyValidator
	defaultFiller     *property.DefaultPropertyFiller

	// 新增的严格验证组件
	valueValidator     *property.PropertyValueValidator
	valueFixer         *property.PropertyValueFixer
	statsCollector     *property.PropertyValidationStatsCollector
	deduplicator       *property.PropertyDeduplicator
	selectionValidator *property.PropertySelectionValidator
	propertyGuardian   *property.RequiredPropertyGuardian
}

// NewAIPropertyMapper 创建新的AI属性映射器
func NewAIPropertyMapper(logger *logrus.Entry, client openaiClient.ChatCompleter) *AIPropertyMapper {
	aiService := NewAIService(client, logger)
	propertyValidator := property.NewPropertyValidator(logger)

	// 创建新的严格验证组件
	valueValidator := property.NewPropertyValueValidator(logger)
	valueFixer := property.NewPropertyValueFixer(logger)
	statsCollector := property.NewPropertyValidationStatsCollector(logger)
	deduplicator := property.NewPropertyDeduplicator(logger)
	selectionValidator := property.NewPropertySelectionValidator(logger)
	propertyGuardian := property.NewRequiredPropertyGuardian(logger)

	return &AIPropertyMapper{
		logger:             logger,
		openaiClient:       client,
		aiService:          aiService,
		propertyValidator:  propertyValidator,
		defaultFiller:      propertyGuardian.GetDefaultFiller(), // 使用propertyGuardian中的DefaultPropertyFiller
		valueValidator:     valueValidator,
		valueFixer:         valueFixer,
		statsCollector:     statsCollector,
		deduplicator:       deduplicator,
		selectionValidator: selectionValidator,
		propertyGuardian:   propertyGuardian,
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
func (m *AIPropertyMapper) BuildGoodsProperties(temuCtx *temucontext.TemuTaskContext, ext *models.ExtensionInfo) error {
	input, err := temucontext.BuildPropertyMappingInput(temuCtx)
	if err != nil {
		m.logger.WithError(err).Warn("property mapping input is incomplete, skip property build")
		return nil
	}
	templateInfo := input.TemplateInfo

	m.logger.WithFields(logrus.Fields{
		"templateID":           templateInfo.TemplateID,
		"goodsPropertiesCount": len(templateInfo.GoodsProperties),
		"specPropertiesCount":  len(templateInfo.GoodsSpecProperties),
	}).Info("使用AI智能映射商品属性")

	// 调用AI进行属性映射 - 使用传入的context，添加超时控制
	aiCtx, cancel := timeout.WithAITimeout(temuCtx.GetContext())
	defer cancel()

	mappingData := preparePropertyMappingData(input, templateInfo.GoodsProperties)

	mappedProperties, err := m.aiService.CallAIForPropertyMapping(aiCtx, mappingData)
	if err != nil {
		m.logger.WithError(err).Error("❌ AI属性映射失败，使用默认值填充所有必填属性")
		m.defaultFiller.FillRequiredPropertiesWithDefaults(templateInfo.GoodsProperties, ext)
		// 再次验证确保所有必填属性都已填充
		m.verifyRequiredProperties(templateInfo.GoodsProperties, ext)
		m.logger.Infof("✅ 默认属性填充完成，共处理 %d 个属性", len(ext.GoodsProperty.GoodsProperties))
		return nil
	}

	// 🔧 关键修复：为AI返回的属性补充模板相关字段
	// 修复template_module_id字段缺失导致TEMU API忽略属性的问题
	m.enrichPropertiesWithTemplateInfo(mappedProperties, templateInfo.GoodsProperties)

	// 应用AI映射的结果并进行严格验证
	m.logger.Infof("📝 AI映射返回 %d 个属性，开始严格验证", len(mappedProperties))

	// 启动统计收集
	m.statsCollector.StartValidation(len(mappedProperties))

	// 使用新的严格验证和修复系统
	validatedProperties := m.valueFixer.FixAllInvalidProperties(mappedProperties, templateInfo.GoodsProperties)

	// 最终验证确保所有属性都有效
	finalProperties := make([]models.PropertyItem, 0, len(validatedProperties))
	for _, prop := range validatedProperties {
		// 查找模板属性
		var templateProp *temutemplate.TemplateRespGoodsProperty
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

	// 🛡️ 最终保障：使用属性守护者进行全面的必填属性检查
	if err := m.propertyGuardian.GuardAllRequiredProperties(templateInfo.GoodsProperties, ext); err != nil {
		m.logger.WithError(err).Error("❌ 必填属性保障失败")
		return fmt.Errorf("必填属性保障失败: %w", err)
	}

	m.logger.Infof("✅ AI属性映射完成，共处理 %d 个属性（必填=%d, 可选=%d）",
		len(ext.GoodsProperty.GoodsProperties), mappedRequired, mappedOptional)
	return nil
}

// enrichPropertiesWithTemplateInfo 为AI返回的属性补充模板相关字段
// 修复template_module_id字段缺失导致TEMU API忽略属性的问题
func (m *AIPropertyMapper) enrichPropertiesWithTemplateInfo(properties []models.PropertyItem, templateProps []temutemplate.TemplateRespGoodsProperty) {
	m.logger.Info("🔧 开始为AI属性补充模板信息")
	m.logger.Infof("🔧 AI返回属性数量: %d, 模板属性数量: %d", len(properties), len(templateProps))

	// 记录所有AI返回的属性
	for i, prop := range properties {
		m.logger.Infof("🔧 AI属性[%d]: PID=%d, TemplatePID=%d, Value=%s, VID=%d",
			i, prop.Pid, prop.TemplatePid, prop.Value, prop.Vid)
	}

	// 创建模板属性映射表，使用PID+TemplatePID组合键来处理相同PID的不同属性
	templateMap := make(map[string]temutemplate.TemplateRespGoodsProperty)
	pidToTemplateProps := make(map[int][]temutemplate.TemplateRespGoodsProperty)

	for _, templateProp := range templateProps {
		// 使用PID+TemplatePID作为唯一键
		key := fmt.Sprintf("%d_%d", templateProp.PID, templateProp.TemplatePID)
		templateMap[key] = templateProp

		// 同时维护PID到属性列表的映射，用于处理AI只返回PID的情况
		pidToTemplateProps[templateProp.PID] = append(pidToTemplateProps[templateProp.PID], templateProp)
	}

	enrichedCount := 0
	for i := range properties {
		prop := &properties[i]
		m.logger.Infof("🔧 处理属性[%d]: PID=%d, TemplatePID=%d, Value=%s, VID=%d",
			i, prop.Pid, prop.TemplatePid, prop.Value, prop.Vid)

		var templateProp *temutemplate.TemplateRespGoodsProperty

		// 首先尝试使用PID+TemplatePID精确匹配
		if prop.TemplatePid != 0 {
			key := fmt.Sprintf("%d_%d", prop.Pid, prop.TemplatePid)
			m.logger.Infof("🔍 尝试精确匹配: key=%s", key)
			if tmplProp, exists := templateMap[key]; exists {
				templateProp = &tmplProp
				m.logger.Infof("✅ 精确匹配成功: %s (PID=%d, TemplatePID=%d)", templateProp.Name, prop.Pid, prop.TemplatePid)
			} else {
				m.logger.Warnf("⚠️ 精确匹配失败: key=%s", key)
			}
		}

		// 如果精确匹配失败，尝试使用PID匹配
		if templateProp == nil {
			if matchedProps, exists := pidToTemplateProps[prop.Pid]; exists {
				if len(matchedProps) == 1 {
					// 只有一个匹配的模板属性，直接使用
					templateProp = &matchedProps[0]
				} else {
					// 多个匹配的模板属性，使用智能选择策略
					templateProp = m.selectBestTemplate(prop, matchedProps)
				}
			}
		}

		if templateProp != nil {
			// 补充关键的模板字段
			prop.TemplatePid = templateProp.TemplatePID
			prop.TemplateModuleID = templateProp.TemplateModuleID
			prop.RefPid = templateProp.RefPID

			// 🔧 新增：设置单位信息
			if len(templateProp.ValueUnit) > 0 {
				prop.ValueUnit = templateProp.ValueUnit[0]
				m.logger.Debugf("🔧 设置属性单位: %s (PID=%d) -> %s", templateProp.Name, prop.Pid, prop.ValueUnit)
			} else if len(templateProp.ValueUnitDTOList) > 0 {
				prop.ValueUnit = templateProp.ValueUnitDTOList[0].ValueUnit
				m.logger.Debugf("🔧 从DTO设置属性单位: %s (PID=%d) -> %s", templateProp.Name, prop.Pid, prop.ValueUnit)
			}

			// 🔧 新增：验证和修复VID/Value组合
			if templateProp.PropertyValueType == 1 && len(templateProp.Values) > 0 {
				m.logger.Infof("🔍 检查属性 %s (template_pid=%d) VID=%d 是否有效", templateProp.Name, templateProp.TemplatePID, prop.Vid)

				// 打印可选值列表用于调试
				validVIDs := make([]int, len(templateProp.Values))
				for i, v := range templateProp.Values {
					validVIDs[i] = v.VID
				}
				m.logger.Infof("🔍 可选VID列表: %v", validVIDs)

				if !m.isValidVIDForTemplate(prop.Vid, templateProp.Values) {
					m.logger.Warnf("⚠️ 属性 %s (template_pid=%d) 的VID=%d不在可选值中，使用第一个可选值修复", templateProp.Name, templateProp.TemplatePID, prop.Vid)

					// 直接使用第一个可选值，避免复杂的语义匹配
					if len(templateProp.Values) > 0 {
						firstValue := templateProp.Values[0]
						m.logger.Infof("🔧 修复VID: %s template_pid=%d VID %d->%d, Value %s->%s",
							templateProp.Name, templateProp.TemplatePID, prop.Vid, firstValue.VID, prop.Value, firstValue.Value)
						prop.Vid = firstValue.VID
						prop.Value = firstValue.Value
					}
				} else {
					m.logger.Infof("✅ 属性 %s (template_pid=%d) VID=%d 验证通过", templateProp.Name, templateProp.TemplatePID, prop.Vid)
				}
			}

			enrichedCount++
			m.logger.Debugf("✅ 属性 %s (PID=%d) 补充模板信息: TemplateModuleID=%d, TemplatePID=%d, RefPID=%d",
				templateProp.Name, prop.Pid, prop.TemplateModuleID, prop.TemplatePid, prop.RefPid)
		} else {
			m.logger.Warnf("⚠️ 未找到PID=%d对应的模板属性", prop.Pid)
		}
	}

	m.logger.Infof("✅ 模板信息补充完成，共处理 %d/%d 个属性", enrichedCount, len(properties))

	// 🔧 新增：条件属性依赖验证和清理
	validator := property.NewConditionalPropertyValidator(m.logger)
	validator.ValidateAndCleanConditionalProperties(&properties, templateProps)
}
