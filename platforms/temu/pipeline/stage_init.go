package pipeline

import (
	commonPipeline "task-processor/common/pipeline"
	"task-processor/platforms/temu/handlers"
)

// addInitHandlers 添加初始化阶段处理器（1-5）
func (b *Builder) addInitHandlers(p *commonPipeline.Pipeline) {
	p.AddHandler(handlers.NewInitDataHandler()). // 1. 初始化产品数据
							AddHandler(handlers.NewStoreInfoHandler(b.storeClient)).                                              // 2. 获取店铺信息
							AddHandler(handlers.NewRawJsonDataHandlerV2(b.rawJsonDataClient, b.amazonConfig, b.amazonProcessor)). // 3. 获取原始JSON数据
							AddHandler(handlers.NewCacheProductHandler(b.rawJsonDataClient, b.amazonConfig, b.amazonProcessor)).  // 4. 缓存产品数据
							AddHandler(handlers.NewProductExistsCheckHandler(b.mappingClient))                                    // 5. 产品存在性检查
}
