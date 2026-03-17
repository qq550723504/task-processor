// Package rules 提供验证规则引擎
package rules

import (
	"fmt"

	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"
	"task-processor/internal/temu/handlerbase"

	"github.com/sirupsen/logrus"
)

// PropertyFeatureDetector 属性特征检测器（类型别名）
type PropertyFeatureDetector = handlerbase.PropertyFeatureDetector

// NewPropertyFeatureDetector 创建属性特征检测器
var NewPropertyFeatureDetector = handlerbase.NewPropertyFeatureDetector

// ValidationRuleEngine 验证规则引擎
type ValidationRuleEngine struct {
	logger          *logrus.Entry
	featureDetector *handlerbase.PropertyFeatureDetector
	rules           []ValidationRule
}

// NewValidationRuleEngine 创建验证规则引擎
func NewValidationRuleEngine(logger *logrus.Entry) *ValidationRuleEngine {
	engine := &ValidationRuleEngine{
		logger:          logger,
		featureDetector: handlerbase.NewPropertyFeatureDetector(logger),
		rules:           make([]ValidationRule, 0),
	}

	// 注册内置规则
	engine.RegisterRule(NewPercentageSumRule(logger))

	return engine
}

// RegisterRule 注册验证规则
func (e *ValidationRuleEngine) RegisterRule(rule ValidationRule) {
	e.rules = append(e.rules, rule)
	e.logger.Debugf("✅ 注册验证规则: %s", rule.GetRuleName())
}

// ValidateAndFixAll 验证并修复所有属性
func (e *ValidationRuleEngine) ValidateAndFixAll(templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) error {
	e.logger.Info("🔍 开始验证和修复属性...")

	// 识别所有属性特征
	features := e.featureDetector.DetectAllFeatures(templateProps)

	// 按PID分组属性
	propertyGroups := e.groupPropertiesByPID(ext.GoodsProperty.GoodsProperties)

	fixedCount := 0
	var validationErrors []string

	// 遍历模板属性
	for _, templateProp := range templateProps {
		feature, exists := features[templateProp.PID]
		if !exists {
			continue
		}

		// 获取该PID的所有属性项
		props := propertyGroups[templateProp.PID]
		if len(props) == 0 {
			continue
		}

		// 应用匹配的验证规则
		for _, rule := range e.rules {
			if !rule.Matches(feature) {
				continue
			}

			// 验证
			result := rule.Validate(props, templateProp)

			if !result.IsValid {
				e.logger.Warnf("❌ 验证失败 [%s]: %s (属性: %s)",
					rule.GetRuleName(), result.ErrorMessage, templateProp.Name)

				if result.CanAutoFix {
					// 尝试修复
					if err := rule.Fix(props, templateProp); err != nil {
						e.logger.Errorf("修复失败 [%s]: %v", rule.GetRuleName(), err)
						validationErrors = append(validationErrors,
							fmt.Sprintf("%s: %s", rule.GetRuleName(), result.ErrorMessage))
					} else {
						e.logger.Infof("✅ 自动修复成功 [%s]: %s", rule.GetRuleName(), templateProp.Name)
						fixedCount++
					}
				} else {
					validationErrors = append(validationErrors,
						fmt.Sprintf("%s: %s", rule.GetRuleName(), result.ErrorMessage))
				}
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("属性验证失败: %v", validationErrors)
	}

	e.logger.Infof("✅ 属性验证完成，自动修复了 %d 个问题", fixedCount)
	return nil
}

// groupPropertiesByPID 按PID分组属性
func (e *ValidationRuleEngine) groupPropertiesByPID(properties []models.PropertyItem) map[int][]*models.PropertyItem {
	groups := make(map[int][]*models.PropertyItem)

	for i := range properties {
		prop := &properties[i]
		groups[prop.Pid] = append(groups[prop.Pid], prop)
	}

	return groups
}
