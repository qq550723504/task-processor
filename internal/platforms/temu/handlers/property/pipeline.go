// Package property 提供属性处理管道，统筹整个属性处理流程
package property

import (
	"context"
	"fmt"
	"sort"
	"time"

	models "task-processor/internal/platforms/temu/api/product"
	temutemplate "task-processor/internal/platforms/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyProcessingPipeline 属性处理管道接口
type PropertyProcessingPipeline interface {
	// Process 处理属性
	Process(ctx context.Context, templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) error

	// AddStage 添加处理阶段
	AddStage(stage PropertyStage)

	// GetStages 获取所有阶段
	GetStages() []PropertyStage

	// SetConfig 设置配置
	SetConfig(config *ProcessingConfig)
}

// DefaultPropertyProcessingPipeline 默认属性处理管道实现
type DefaultPropertyProcessingPipeline struct {
	stages []PropertyStage
	logger *logrus.Entry
	config *ProcessingConfig
}

// NewPropertyProcessingPipeline 创建新的属性处理管道
func NewPropertyProcessingPipeline(logger *logrus.Entry) PropertyProcessingPipeline {
	return &DefaultPropertyProcessingPipeline{
		stages: make([]PropertyStage, 0),
		logger: logger,
		config: NewDefaultProcessingConfig(),
	}
}

// AddStage 添加处理阶段
func (p *DefaultPropertyProcessingPipeline) AddStage(stage PropertyStage) {
	p.stages = append(p.stages, stage)

	// 按执行顺序排序
	sort.Slice(p.stages, func(i, j int) bool {
		return p.stages[i].GetOrder() < p.stages[j].GetOrder()
	})

	p.logger.Debugf("添加处理阶段: %s (顺序: %d)", stage.GetName(), stage.GetOrder())
}

// GetStages 获取所有阶段
func (p *DefaultPropertyProcessingPipeline) GetStages() []PropertyStage {
	return p.stages
}

// SetConfig 设置配置
func (p *DefaultPropertyProcessingPipeline) SetConfig(config *ProcessingConfig) {
	p.config = config
}

// Process 处理属性
func (p *DefaultPropertyProcessingPipeline) Process(ctx context.Context, templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) error {
	startTime := time.Now()
	p.logger.Infof("🚀 开始属性处理管道，模板属性数量: %d, 当前属性数量: %d",
		len(templateProps), len(ext.GoodsProperty.GoodsProperties))

	// 创建处理上下文
	propertyCtx := p.buildPropertyContext(ctx, templateProps, ext)

	// 预处理：识别所有属性特征
	if err := p.preprocessFeatures(propertyCtx); err != nil {
		return fmt.Errorf("属性特征预处理失败: %w", err)
	}

	// 执行所有处理阶段
	if err := p.executeStages(propertyCtx); err != nil {
		return fmt.Errorf("管道执行失败: %w", err)
	}

	// 更新结果
	ext.GoodsProperty.GoodsProperties = propertyCtx.CurrentProperties

	// 输出处理统计
	p.logProcessingStats(propertyCtx, startTime)

	return nil
}

// buildPropertyContext 构建属性处理上下文
func (p *DefaultPropertyProcessingPipeline) buildPropertyContext(ctx context.Context, templateProps []temutemplate.TemplateRespGoodsProperty, ext *models.ExtensionInfo) *PropertyContext {
	propertyCtx := NewPropertyContext(ctx, p.logger)

	// 设置模板属性
	propertyCtx.TemplateProperties = templateProps

	// 设置当前属性
	propertyCtx.CurrentProperties = ext.GoodsProperty.GoodsProperties

	// 设置配置
	propertyCtx.Config = p.config

	// 初始化统计
	propertyCtx.Statistics.TotalProperties = len(templateProps)

	return propertyCtx
}

// preprocessFeatures 预处理属性特征
func (p *DefaultPropertyProcessingPipeline) preprocessFeatures(ctx *PropertyContext) error {
	if !ctx.Config.EnableCache {
		return nil
	}

	p.logger.Debug("🔍 开始预处理属性特征")

	featureDetector := NewPropertyFeatureDetector(p.logger)
	processedCount := 0

	for _, templateProp := range ctx.TemplateProperties {
		// 检查缓存
		cacheKey := GenerateFeatureCacheKey(templateProp.PID, templateProp.ControlType,
			p.getValueUnit(templateProp))

		if feature, exists := ctx.Cache.GetFeature(cacheKey); exists {
			ctx.SetFeature(templateProp.PID, feature)
			continue
		}

		// 识别特征
		feature := featureDetector.DetectFeatures(templateProp)

		// 设置到上下文和缓存
		ctx.SetFeature(templateProp.PID, feature)
		ctx.Cache.SetFeature(cacheKey, feature)

		processedCount++
	}

	p.logger.Debugf("✅ 属性特征预处理完成，处理数量: %d", processedCount)
	return nil
}

// executeStages 执行所有处理阶段
func (p *DefaultPropertyProcessingPipeline) executeStages(ctx *PropertyContext) error {
	for _, stage := range p.stages {
		// 检查阶段是否启用
		if !stage.IsEnabled(ctx) {
			p.logger.Debugf("⏭️ 跳过禁用的阶段: %s", stage.GetName())
			continue
		}

		// 记录阶段开始
		ctx.Statistics.RecordStageStart(stage.GetName())
		stageStartTime := time.Now()

		p.logger.Infof("🔄 执行阶段: %s", stage.GetName())

		// 执行阶段
		if err := stage.Process(ctx); err != nil {
			ctx.Statistics.RecordStageEnd(stage.GetName(), 0, 0, 1)
			return NewStageError(stage.GetName(), "阶段执行失败", err)
		}

		// 记录阶段结束
		stageDuration := time.Since(stageStartTime)
		ctx.Statistics.RecordStageEnd(stage.GetName(), 1, 1, 0)

		p.logger.Infof("✅ 阶段完成: %s (耗时: %v)", stage.GetName(), stageDuration)
	}

	return nil
}

// logProcessingStats 输出处理统计
func (p *DefaultPropertyProcessingPipeline) logProcessingStats(ctx *PropertyContext, startTime time.Time) {
	totalDuration := time.Since(startTime)

	p.logger.WithFields(logrus.Fields{
		"总耗时":    totalDuration,
		"模板属性数量": len(ctx.TemplateProperties),
		"最终属性数量": len(ctx.CurrentProperties),
		"处理阶段数量": len(p.stages),
	}).Info("📊 属性处理管道完成")

	// 输出各阶段统计
	if ctx.Config.EnableStatistics {
		for stageName, stats := range ctx.Statistics.StageStats {
			p.logger.WithFields(logrus.Fields{
				"阶段":   stageName,
				"耗时":   stats.Duration,
				"处理数量": stats.ProcessedCount,
				"成功数量": stats.SuccessCount,
				"错误数量": stats.ErrorCount,
			}).Debug("📈 阶段统计")
		}
	}
}

// getValueUnit 获取属性的值单位
func (p *DefaultPropertyProcessingPipeline) getValueUnit(templateProp temutemplate.TemplateRespGoodsProperty) string {
	if len(templateProp.ValueUnit) > 0 {
		return templateProp.ValueUnit[0]
	}
	if len(templateProp.ValueUnitDTOList) > 0 {
		return templateProp.ValueUnitDTOList[0].ValueUnit
	}
	return ""
}
