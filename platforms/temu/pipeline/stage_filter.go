package pipeline

import (
	"task-processor/common/memory"
	commonPipeline "task-processor/common/pipeline"
	"task-processor/platforms/temu/handlers"
)

// addFilterHandlers 添加筛选和验证阶段处理器（6-11）
func (b *Builder) addFilterHandlers(p *commonPipeline.Pipeline) {
	// 类型断言 memoryManager
	var memMgr *memory.MemoryManager
	if b.memoryManager != nil {
		memMgr, _ = b.memoryManager.(*memory.MemoryManager)
	}

	p.AddHandler(handlers.NewFilterRuleHandler(b.filterRuleClient)). // 6. 主产品筛选规则检查
										AddHandler(handlers.NewTextCheckHandler()).                                                             // 7. 文本检查
										AddHandler(handlers.NewVariantJsonDataHandler(b.rawJsonDataClient, b.amazonConfig, b.amazonProcessor)). // 8. 获取变体JSON数据
										AddHandler(handlers.NewCacheVariantsHandler(b.rawJsonDataClient, b.amazonConfig, b.amazonProcessor)).   // 9. 缓存变体数据
										AddHandler(handlers.NewVariantFilterHandler(b.filterRuleClient)).                                       // 10. 变体筛选规则检查
										AddHandler(handlers.NewCheckDailyLimitHandler(memMgr))                                                  // 11. 检查每日上架限制
}
