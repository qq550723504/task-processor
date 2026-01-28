// Package handlers 提供百分比总和验证规则
package handlers

import (
	"fmt"
	"strconv"

	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// PercentageSumRule 百分比总和验证规则
type PercentageSumRule struct {
	logger    *logrus.Entry
	targetSum int
}

// NewPercentageSumRule 创建百分比总和验证规则
func NewPercentageSumRule(logger *logrus.Entry) *PercentageSumRule {
	return &PercentageSumRule{
		logger:    logger,
		targetSum: 100,
	}
}

// GetRuleName 获取规则名称
func (r *PercentageSumRule) GetRuleName() string {
	return "PercentageSumRule"
}

// Matches 判断规则是否匹配该属性特征
func (r *PercentageSumRule) Matches(feature PropertyFeature) bool {
	return feature.IsPercentageSum
}

// Validate 验证属性值
func (r *PercentageSumRule) Validate(props []*models.PropertyItem, templateProp types.TemplateRespGoodsProperty) RuleValidationResult {
	if len(props) == 0 {
		return RuleValidationResult{
			IsValid:      false,
			ErrorMessage: "没有找到属性数据",
			CanAutoFix:   false,
		}
	}

	currentSum := r.calculateSum(props)

	if currentSum == r.targetSum {
		return RuleValidationResult{
			IsValid:       true,
			CurrentValue:  currentSum,
			ExpectedValue: r.targetSum,
		}
	}

	return RuleValidationResult{
		IsValid:       false,
		ErrorMessage:  fmt.Sprintf("百分比总和为%d%%，期望%d%%", currentSum, r.targetSum),
		CurrentValue:  currentSum,
		ExpectedValue: r.targetSum,
		CanAutoFix:    true,
	}
}

// Fix 修复属性值
func (r *PercentageSumRule) Fix(props []*models.PropertyItem, templateProp types.TemplateRespGoodsProperty) error {
	if len(props) == 0 {
		return fmt.Errorf("没有属性数据需要修复")
	}

	currentSum := r.calculateSum(props)

	if currentSum == r.targetSum {
		r.logger.Debugf("✅ 百分比总和已经是%d%%，无需修复", r.targetSum)
		return nil
	}

	r.logger.Infof("🔧 修复百分比总和: %d%% -> %d%%", currentSum, r.targetSum)

	if len(props) == 1 {
		return r.fixSingleProperty(props[0])
	}

	return r.fixMultipleProperties(props, currentSum)
}

// calculateSum 计算百分比总和
func (r *PercentageSumRule) calculateSum(props []*models.PropertyItem) int {
	total := 0
	for _, prop := range props {
		if prop.NumberInputValue != "" {
			if percentage, err := strconv.Atoi(prop.NumberInputValue); err == nil {
				total += percentage
			}
		}
	}
	return total
}

// fixSingleProperty 修复单个属性
func (r *PercentageSumRule) fixSingleProperty(prop *models.PropertyItem) error {
	oldValue := prop.NumberInputValue
	prop.NumberInputValue = strconv.Itoa(r.targetSum)
	r.logger.Infof("✅ 单一属性修正: %s %s%% -> %d%%", prop.Value, oldValue, r.targetSum)
	return nil
}

// fixMultipleProperties 修复多个属性
func (r *PercentageSumRule) fixMultipleProperties(props []*models.PropertyItem, currentSum int) error {
	if currentSum <= 0 {
		return r.distributeEvenly(props)
	}

	return r.adjustProportionally(props, currentSum)
}

// distributeEvenly 平均分配百分比
func (r *PercentageSumRule) distributeEvenly(props []*models.PropertyItem) error {
	count := len(props)
	averagePercentage := r.targetSum / count
	remaining := r.targetSum - (averagePercentage * (count - 1))

	for i, prop := range props {
		oldValue := prop.NumberInputValue
		if i == count-1 {
			prop.NumberInputValue = strconv.Itoa(remaining)
		} else {
			prop.NumberInputValue = strconv.Itoa(averagePercentage)
		}
		r.logger.Infof("✅ 平均分配: %s %s%% -> %s%%", prop.Value, oldValue, prop.NumberInputValue)
	}

	return nil
}

// adjustProportionally 按比例调整百分比
func (r *PercentageSumRule) adjustProportionally(props []*models.PropertyItem, currentSum int) error {
	remaining := r.targetSum

	for i, prop := range props {
		oldValue := prop.NumberInputValue

		if prop.NumberInputValue != "" {
			if currentPercentage, err := strconv.Atoi(prop.NumberInputValue); err == nil {
				if i == len(props)-1 {
					// 最后一个属性使用剩余百分比
					prop.NumberInputValue = strconv.Itoa(remaining)
				} else {
					// 按比例计算新百分比
					newPercentage := (currentPercentage * r.targetSum) / currentSum
					if newPercentage < 1 {
						newPercentage = 1 // 最小1%
					}
					prop.NumberInputValue = strconv.Itoa(newPercentage)
					remaining -= newPercentage
				}
				r.logger.Infof("✅ 比例调整: %s %s%% -> %s%%", prop.Value, oldValue, prop.NumberInputValue)
			}
		}
	}

	return nil
}
