// Package property 提供属性验证阶段，负责验证属性的有效性和完整性
package property

import (
	"fmt"
	models "task-processor/internal/temu/api/product"
	"task-processor/internal/temu/rules"

	"github.com/sirupsen/logrus"
)

// PropertyValidationStage 属性验证阶段
type PropertyValidationStage struct {
	*BasePropertyStage
	ruleEngine *rules.ValidationRuleEngine
	logger     *logrus.Entry
}

// NewPropertyValidationStage 创建属性验证阶段
func NewPropertyValidationStage(logger *logrus.Entry) *PropertyValidationStage {
	return &PropertyValidationStage{
		BasePropertyStage: NewBasePropertyStage("属性验证阶段", 200),
		ruleEngine:        rules.NewValidationRuleEngine(logger),
		logger:            logger,
	}
}

// NewPropertyValidationStageWithEngine 创建带验证引擎的属性验证阶段
func NewPropertyValidationStageWithEngine(logger *logrus.Entry, ruleEngine *rules.ValidationRuleEngine) *PropertyValidationStage {
	return &PropertyValidationStage{
		BasePropertyStage: NewBasePropertyStage("属性验证阶段", 200),
		ruleEngine:        ruleEngine,
		logger:            logger,
	}
}

// Process 处理属性验证
func (s *PropertyValidationStage) Process(ctx *PropertyContext) error {
	s.logger.Info("🔍 开始属性验证处理")

	if s.ruleEngine == nil {
		s.ruleEngine = rules.NewValidationRuleEngine(s.logger)
	}

	// 创建临时的ExtensionInfo来兼容现有接口
	tempExt := &models.ExtensionInfo{
		GoodsProperty: models.GoodsPropertys{
			GoodsProperties: ctx.CurrentProperties,
		},
	}

	// 使用验证规则引擎进行验证
	if err := s.ruleEngine.ValidateAndFixAll(ctx.TemplateProperties, tempExt); err != nil {
		s.logger.WithError(err).Warn("⚠️ 验证过程中发现问题")
		// 在非严格模式下，验证失败不阻止流程继续
		if ctx.Config.EnableStrictMode {
			return fmt.Errorf("严格模式下验证失败: %w", err)
		}
	}

	// 更新上下文中的属性
	ctx.CurrentProperties = tempExt.GoodsProperty.GoodsProperties

	// 更新统计
	ctx.Statistics.ProcessedCount += len(ctx.TemplateProperties)

	s.logger.Info("✅ 属性验证处理完成")
	return nil
}

// IsEnabled 检查阶段是否启用
func (s *PropertyValidationStage) IsEnabled(ctx *PropertyContext) bool {
	return s.BasePropertyStage.IsEnabled(ctx)
}

// SetRuleEngine 设置验证规则引擎
func (s *PropertyValidationStage) SetRuleEngine(ruleEngine *rules.ValidationRuleEngine) {
	s.ruleEngine = ruleEngine
}
