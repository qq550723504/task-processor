// Package property 提供属性映射编排器，统筹整个属性处理流程
package property

import (
	"fmt"

	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/handlers/template"

	"github.com/sirupsen/logrus"
)

// PropertyMappingOrchestrator 属性映射编排器
type PropertyMappingOrchestrator struct {
	pipeline PropertyProcessingPipeline
	logger   *logrus.Entry
}

// NewPropertyMappingOrchestrator 创建新的属性映射编排器
func NewPropertyMappingOrchestrator(logger *logrus.Entry) *PropertyMappingOrchestrator {
	orchestrator := &PropertyMappingOrchestrator{
		pipeline: NewPropertyProcessingPipeline(logger),
		logger:   logger,
	}

	// 初始化默认的处理阶段
	orchestrator.initializeDefaultStages()

	return orchestrator
}

// ProcessProperties 处理属性（主入口方法）
func (o *PropertyMappingOrchestrator) ProcessProperties(temuCtx *temucontext.TemuTaskContext, ext *models.ExtensionInfo) error {
	// 获取模板信息
	templateInfo, exists := template.GetTemplateInfoFromContext(temuCtx)
	if !exists {
		o.logger.Warn("未找到模板信息，跳过属性处理")
		return nil
	}

	o.logger.WithFields(logrus.Fields{
		"模板ID":   templateInfo.TemplateID,
		"商品属性数量": len(templateInfo.GoodsProperties),
		"规格属性数量": len(templateInfo.GoodsSpecProperties),
	}).Info("🎯 开始属性映射编排")

	// 使用管道处理属性
	ctx := temuCtx.GetContext()
	if err := o.pipeline.Process(ctx, templateInfo.GoodsProperties, ext); err != nil {
		return fmt.Errorf("属性处理管道执行失败: %w", err)
	}

	o.logger.Infof("✅ 属性映射编排完成，最终属性数量: %d", len(ext.GoodsProperty.GoodsProperties))
	return nil
}

// initializeDefaultStages 初始化默认的处理阶段
func (o *PropertyMappingOrchestrator) initializeDefaultStages() {
	// 阶段1: 属性映射阶段 (order: 100)
	mappingStage := NewPropertyMappingStage(o.logger)
	o.pipeline.AddStage(mappingStage)

	// 阶段2: 属性验证阶段 (order: 200)
	validationStage := NewPropertyValidationStage(o.logger)
	o.pipeline.AddStage(validationStage)

	// 阶段3: 属性修复阶段 (order: 300)
	fixingStage := NewPropertyFixingStage(o.logger)
	o.pipeline.AddStage(fixingStage)

	// 阶段4: 属性最终化阶段 (order: 400)
	finalizationStage := NewPropertyFinalizationStage(o.logger)
	o.pipeline.AddStage(finalizationStage)

	o.logger.Debug("🔧 默认处理阶段初始化完成")
}

// AddCustomStage 添加自定义处理阶段
func (o *PropertyMappingOrchestrator) AddCustomStage(stage PropertyStage) {
	o.pipeline.AddStage(stage)
	o.logger.Debugf("➕ 添加自定义阶段: %s", stage.GetName())
}

// SetConfig 设置处理配置
func (o *PropertyMappingOrchestrator) SetConfig(config *ProcessingConfig) {
	o.pipeline.SetConfig(config)
	o.logger.Debug("⚙️ 更新处理配置")
}

// GetPipeline 获取处理管道（用于测试和调试）
func (o *PropertyMappingOrchestrator) GetPipeline() PropertyProcessingPipeline {
	return o.pipeline
}

// GetStages 获取所有处理阶段（用于监控和调试）
func (o *PropertyMappingOrchestrator) GetStages() []PropertyStage {
	return o.pipeline.GetStages()
}

// ValidateConfiguration 验证配置是否正确
func (o *PropertyMappingOrchestrator) ValidateConfiguration() error {
	stages := o.pipeline.GetStages()
	if len(stages) == 0 {
		return fmt.Errorf("没有配置任何处理阶段")
	}

	// 检查阶段顺序是否合理
	for i := 1; i < len(stages); i++ {
		if stages[i].GetOrder() < stages[i-1].GetOrder() {
			return fmt.Errorf("处理阶段顺序配置错误: %s (order: %d) 应该在 %s (order: %d) 之前",
				stages[i].GetName(), stages[i].GetOrder(),
				stages[i-1].GetName(), stages[i-1].GetOrder())
		}
	}

	o.logger.Debug("✅ 配置验证通过")
	return nil
}

// GetProcessingStats 获取处理统计信息（用于监控）
func (o *PropertyMappingOrchestrator) GetProcessingStats() map[string]any {
	stages := o.pipeline.GetStages()
	stats := make(map[string]any)

	stats["total_stages"] = len(stages)

	stageInfo := make([]map[string]any, 0, len(stages))
	for _, stage := range stages {
		info := map[string]any{
			"name":    stage.GetName(),
			"order":   stage.GetOrder(),
			"enabled": stage.IsEnabled(nil), // 这里传nil，实际使用时需要传入真实的context
		}
		stageInfo = append(stageInfo, info)
	}
	stats["stages"] = stageInfo

	return stats
}
