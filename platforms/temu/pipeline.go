package temu

import (
	"task-processor/common/config"
	"task-processor/common/management/api"
	"task-processor/common/pipeline"
	"task-processor/platforms/temu/handlers"
)

// TemuPipelineBuilder TEMU管道构建器
type TemuPipelineBuilder struct {
	storeClient interface {
		GetStore(id int64) (*api.StoreRespDTO, error)
	}
	rawJsonDataClient api.RawJsonDataAPI
	amazonConfig      *config.AmazonConfig
}

// NewTemuPipelineBuilder 创建TEMU管道构建器
func NewTemuPipelineBuilder(
	storeClient interface {
		GetStore(id int64) (*api.StoreRespDTO, error)
	},
	rawJsonDataClient api.RawJsonDataAPI,
	amazonConfig *config.AmazonConfig,
) *TemuPipelineBuilder {
	return &TemuPipelineBuilder{
		storeClient:       storeClient,
		rawJsonDataClient: rawJsonDataClient,
		amazonConfig:      amazonConfig,
	}
}

// BuildPipeline 构建TEMU管道（包含Amazon数据处理）
func (b *TemuPipelineBuilder) BuildPipeline() *pipeline.Pipeline {
	p := pipeline.NewPipeline("TEMU产品发布管道(含Amazon)")
	b.addHandlers(p)
	return p
}

// addHandlers 添加处理器序列（固定包含Amazon处理器）
func (b *TemuPipelineBuilder) addHandlers(p *pipeline.Pipeline) {
	p.AddHandler(handlers.NewInitDataHandler()). // 1. 初始化产品数据
							AddHandler(handlers.NewStoreInfoHandler(b.storeClient)).                             // 2. 获取店铺信息
							AddHandler(handlers.NewRawJsonDataHandlerV2(b.rawJsonDataClient, b.amazonConfig)).   // 3. 获取原始JSON数据（优化版）
							AddHandler(handlers.NewAmazonDataHandler()).                                         // 4. Amazon数据处理
							AddHandler(handlers.NewTextCheckHandler()).                                          // 5. 文本检查
							AddHandler(handlers.NewVariantJsonDataHandler(b.rawJsonDataClient, b.amazonConfig)). // 6. 获取变体JSON数据
							AddHandler(handlers.NewCategoryRecommendHandler()).                                  // 7. 分类推荐
							AddHandler(handlers.NewCategoryDisclaimHandler()).                                   // 8. 分类免责声明
							AddHandler(handlers.NewCommitCreateHandler()).                                       // 9. 提交创建
							AddHandler(handlers.NewCategoryHandler()).                                           // 10. 分类处理
							AddHandler(handlers.NewImageUploadHandler()).                                        // 11. 图片上传
							AddHandler(handlers.NewImageHandler()).                                              // 12. 图片处理
							AddHandler(handlers.NewBuildSpuHandler()).                                           // 13. 构建SPU
							AddHandler(handlers.NewProductSubmitHandler()).                                      // 14. 产品提交
							AddHandler(handlers.NewProductSaveHandler()).                                        // 15. 产品保存
							AddHandler(handlers.NewSkuCheckHandler()).                                           // 16. SKU检查
							AddHandler(handlers.NewOutGoodsSnHandler()).                                         // 17. 外部商品编号
							AddHandler(handlers.NewTemplateQueryHandler()).                                      // 18. 模板查询
							AddHandler(handlers.NewMaxRetailPriceHandler()).                                     // 19. 最大零售价格
							AddHandler(handlers.NewSupplierPriceHandler()).                                      // 20. 供应商价格
							AddHandler(handlers.NewCompliancePhotoHandler()).                                    // 21. 合规性照片
							AddHandler(handlers.NewComplianceCertHandler()).                                     // 22. 合规性认证
							AddHandler(handlers.NewCommitDetailHandler()).                                       // 23. 提交详情
							AddHandler(handlers.NewCostTemplateHandler()).                                       // 24. 成本模板
							AddHandler(handlers.NewPublishHandler())                                             // 25. 发布产品
}

// CreateTEMUPipeline 创建TEMU处理管道（保持向后兼容）
func CreateTEMUPipeline(storeClient interface {
	GetStore(id int64) (*api.StoreRespDTO, error)
}, rawJsonDataClient api.RawJsonDataAPI, amazonConfig *config.AmazonConfig) *pipeline.Pipeline {
	builder := NewTemuPipelineBuilder(storeClient, rawJsonDataClient, amazonConfig)
	return builder.BuildPipeline()
}
