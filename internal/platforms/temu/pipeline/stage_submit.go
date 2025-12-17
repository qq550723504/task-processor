package pipeline

import (
	"task-processor/internal/common/memory"
	commonPipeline "task-processor/internal/common/pipeline"
	"task-processor/internal/platforms/temu/handlers"
)

// addSubmitHandlers 添加提交和保存阶段处理器（30-32）
func (b *Builder) addSubmitHandlers(p *commonPipeline.Pipeline) {
	// 类型断言 memoryManager
	var memMgr *memory.MemoryManager
	if b.memoryManager != nil {
		memMgr, _ = b.memoryManager.(*memory.MemoryManager)
	}

	p.AddHandler(handlers.NewPriceQueryHandler()). // 30. 价格查询
							AddHandler(handlers.NewProductSubmitHandler(b.mappingClient)).            // 31. 产品提交
							AddHandler(handlers.NewSavePublishResultHandler(b.mappingClient, memMgr)) // 32. 保存发品结果
}
