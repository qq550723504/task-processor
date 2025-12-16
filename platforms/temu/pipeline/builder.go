// Package pipeline TEMU管道构建器
package pipeline

import (
	"task-processor/common/management/api"
	commonPipeline "task-processor/common/pipeline"
	"task-processor/internal/config"
	"task-processor/openai"
)

// Builder TEMU管道构建器
type Builder struct {
	storeClient       StoreClientProvider
	rawJsonDataClient api.RawJsonDataAPI
	filterRuleClient  api.FilterRuleAPI
	profitRuleClient  api.ProfitRuleAPI
	storeAPIClient    api.StoreAPI
	mappingClient     api.ProductImportMappingAPI
	memoryManager     any // 内存管理器（memory.MemoryManager）
	amazonConfig      *config.AmazonConfig
	amazonProcessor   any // 共享的Amazon处理器（可以为nil）
	openaiConfig      *openai.ClientConfig
}

// StoreClientProvider 店铺客户端提供者接口
type StoreClientProvider interface {
	GetStore(id int64) (*api.StoreRespDTO, error)
}

// NewBuilder 创建TEMU管道构建器
func NewBuilder(
	storeClient StoreClientProvider,
	rawJsonDataClient api.RawJsonDataAPI,
	filterRuleClient api.FilterRuleAPI,
	profitRuleClient api.ProfitRuleAPI,
	storeAPIClient api.StoreAPI,
	mappingClient api.ProductImportMappingAPI,
	memoryManager any,
	amazonConfig *config.AmazonConfig,
	amazonProcessor any,
	openaiConfig *openai.ClientConfig,
) *Builder {
	return &Builder{
		storeClient:       storeClient,
		rawJsonDataClient: rawJsonDataClient,
		filterRuleClient:  filterRuleClient,
		profitRuleClient:  profitRuleClient,
		storeAPIClient:    storeAPIClient,
		mappingClient:     mappingClient,
		memoryManager:     memoryManager,
		amazonConfig:      amazonConfig,
		amazonProcessor:   amazonProcessor,
		openaiConfig:      openaiConfig,
	}
}

// Build 构建TEMU管道（包含Amazon数据处理）
func (b *Builder) Build() *commonPipeline.Pipeline {
	p := commonPipeline.NewPipeline("TEMU产品发布管道(含Amazon)")
	b.addAllHandlers(p)
	return p
}

// addAllHandlers 添加所有处理器（按阶段组织）
func (b *Builder) addAllHandlers(p *commonPipeline.Pipeline) {
	b.addInitHandlers(p)     // 1-5: 初始化和数据获取
	b.addFilterHandlers(p)   // 6-11: 筛选和验证（含每日限制检查）
	b.addCategoryHandlers(p) // 12-18: 分类和SKU处理
	b.addImageHandlers(p)    // 19-22: 图片处理
	b.addContentHandlers(p)  // 23-30: 内容构建和优化
	b.addSubmitHandlers(p)   // 31-33: 提交和保存
}
