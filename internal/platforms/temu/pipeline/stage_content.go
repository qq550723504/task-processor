package pipeline

import (
	commonPipeline "task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/handlers"
)

// addContentHandlers 添加内容构建和优化阶段处理器（22-24）
func (b *Builder) addContentHandlers(p *commonPipeline.Pipeline) {
	p.AddHandler(handlers.NewTemplateQueryHandler()). // 22. 模板查询
								AddHandler(handlers.NewBuildSpuHandler(b.openaiConfig, b.profitRuleClient)). // 23. 构建SPU（内含AI内容重写并行优化）
		// 24. 并行执行内容验证器
		AddHandler(commonPipeline.NewParallelHandler(
			"内容验证并行处理",
			handlers.NewProductNameValidator(),
			handlers.NewBulletPointsValidator(),
			handlers.NewProductDescriptionValidator(),
			handlers.NewSensitiveWordsFilter(),
		)).
		AddHandler(handlers.NewBrandClearHandler()) // 25. 清除品牌名称
}
