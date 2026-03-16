// Package property 提供属性映射阶段，负责默认填充
package property

import (
	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyMappingStage 属性映射阶段
type PropertyMappingStage struct {
	*BasePropertyStage
	defaultFiller *DefaultPropertyFiller
	logger        *logrus.Entry
}

// NewPropertyMappingStage 创建属性映射阶段
func NewPropertyMappingStage(logger *logrus.Entry) *PropertyMappingStage {
	return &PropertyMappingStage{
		BasePropertyStage: NewBasePropertyStage("属性映射阶段", 100),
		logger:            logger,
	}
}

// Process 处理属性映射
func (s *PropertyMappingStage) Process(ctx *PropertyContext) error {
	s.logger.Info("🎯 开始属性映射处理")

	// 初始化组件
	if s.defaultFiller == nil {
		s.defaultFiller = NewDefaultPropertyFiller(s.logger)
	}

	// 创建临时的ExtensionInfo来兼容现有接口
	tempExt := &models.ExtensionInfo{
		GoodsProperty: models.GoodsPropertys{
			GoodsProperties: ctx.CurrentProperties,
		},
	}

	// 使用默认填充器处理必填属性
	s.defaultFiller.FillRequiredPropertiesWithDefaults(ctx.TemplateProperties, tempExt)

	// 更新上下文中的属性
	ctx.CurrentProperties = tempExt.GoodsProperty.GoodsProperties

	// 更新统计
	filledCount := s.countFilledProperties(ctx.TemplateProperties, ctx.CurrentProperties)
	ctx.Statistics.ProcessedCount += len(ctx.TemplateProperties)
	ctx.Statistics.FixedCount += filledCount

	s.logger.Infof("✅ 属性映射完成，填充属性数量: %d", filledCount)
	return nil
}

// countFilledProperties 统计填充的属性数量
func (s *PropertyMappingStage) countFilledProperties(templateProps []temutemplate.TemplateRespGoodsProperty, currentProps []models.PropertyItem) int {
	// 创建已有属性的PID集合
	existingPIDs := make(map[int]bool)
	for _, prop := range currentProps {
		existingPIDs[prop.Pid] = true
	}

	// 统计必填属性中已填充的数量
	count := 0
	for _, templateProp := range templateProps {
		if templateProp.Required && existingPIDs[templateProp.PID] {
			count++
		}
	}
	return count
}

// IsEnabled 检查阶段是否启用
func (s *PropertyMappingStage) IsEnabled(ctx *PropertyContext) bool {
	return s.BasePropertyStage.IsEnabled(ctx)
}

// SetDefaultFiller 设置默认填充器
func (s *PropertyMappingStage) SetDefaultFiller(defaultFiller *DefaultPropertyFiller) {
	s.defaultFiller = defaultFiller
}
