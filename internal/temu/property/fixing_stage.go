// Package property 提供属性修复阶段，负责修复无效或不完整的属性
package property

import (
	"strconv"
	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyFixingStage 属性修复阶段
type PropertyFixingStage struct {
	*BasePropertyStage
	logger *logrus.Entry
}

// NewPropertyFixingStage 创建属性修复阶段
func NewPropertyFixingStage(logger *logrus.Entry) *PropertyFixingStage {
	return &PropertyFixingStage{
		BasePropertyStage: NewBasePropertyStage("属性修复阶段", 300),
		logger:            logger,
	}
}

// Process 处理属性修复
func (s *PropertyFixingStage) Process(ctx *PropertyContext) error {
	s.logger.Info("🔧 开始属性修复处理")

	// 修复阶段主要处理百分比总和等特殊情况
	// 大部分修复工作已经在验证阶段的ValidationRuleEngine中完成

	fixedCount := 0

	// 检查是否有需要特殊处理的属性
	for _, templateProp := range ctx.TemplateProperties {
		feature, exists := ctx.GetFeature(templateProp.PID)
		if !exists {
			continue
		}

		// 处理百分比总和属性的特殊情况
		if feature.IsPercentageSum {
			if s.fixPercentageProperty(ctx, feature, templateProp) {
				fixedCount++
			}
		}
	}

	// 更新统计
	ctx.Statistics.ProcessedCount += len(ctx.TemplateProperties)
	ctx.Statistics.FixedCount += fixedCount

	s.logger.Infof("🔧 属性修复完成，修复数量: %d", fixedCount)
	return nil
}

// fixPercentageProperty 修复百分比属性
func (s *PropertyFixingStage) fixPercentageProperty(ctx *PropertyContext, feature PropertyFeature, _ temutemplate.TemplateRespGoodsProperty) bool {
	// 查找该PID的属性
	var targetProps []*models.PropertyItem
	for i := range ctx.CurrentProperties {
		if ctx.CurrentProperties[i].Pid == feature.PID {
			targetProps = append(targetProps, &ctx.CurrentProperties[i])
		}
	}

	if len(targetProps) == 0 {
		return false
	}

	// 检查百分比总和
	totalPercentage := 0
	for _, prop := range targetProps {
		if prop.NumberInputValue != "" {
			if percentage, err := strconv.Atoi(prop.NumberInputValue); err == nil {
				totalPercentage += percentage
			}
		}
	}

	// 如果总和不是100%，进行修复
	if totalPercentage != 100 && totalPercentage > 0 {
		s.logger.Infof("🔢 修复百分比总和: %s, %d%% -> 100%%", feature.Name, totalPercentage)

		// 简单修复：将第一个属性调整为100%
		if len(targetProps) > 0 {
			targetProps[0].NumberInputValue = "100"
			return true
		}
	}

	return false
}

// IsEnabled 检查阶段是否启用
func (s *PropertyFixingStage) IsEnabled(ctx *PropertyContext) bool {
	return s.BasePropertyStage.IsEnabled(ctx)
}
