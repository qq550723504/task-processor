package pipeline

import (
	commonPipeline "task-processor/common/pipeline"
	"task-processor/platforms/temu/handlers"
)

// addImageHandlers 添加图片处理阶段处理器（18-20）
func (b *Builder) addImageHandlers(p *commonPipeline.Pipeline) {
	p.AddHandler(handlers.NewAISkuMappingHandler(b.openaiConfig)). // 18. AI SKU映射生成
									AddHandler(handlers.NewImageInitHandler()).    // 19. 图片初始化
									AddHandler(handlers.NewImageValidator()).      // 20. 图片验证（包含白边填充）
									AddHandler(handlers.NewImageUploadProcessor()) // 21. 图片上传（包含尺寸标注）
}
