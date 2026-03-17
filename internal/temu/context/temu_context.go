// Package context 提供TEMU平台的强类型任务上下文
package context

import (
	"context"
	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management"
	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/pipeline"
	"task-processor/internal/product"
	"task-processor/internal/temu/api"
)

// TemuTaskContext TEMU平台任务上下文，嵌入通用上下文并持有平台特定字段。
type TemuTaskContext struct {
	*pipeline.DefaultTaskContext

	// 基础组件
	ManagementClientMgr *management.ClientManager
	MemoryManager       *state.MemoryManager

	// Amazon 抓取结果
	AmazonProcessor product.AmazonScraper
	AmazonProduct   *model.Product
	Variants        []*model.Product

	// TEMU API 客户端
	APIClient api.APIClientInterface
	QueryAPI  any

	// TEMU 产品数据
	TemuProduct *api.Product
	StoreInfo   *management_api.StoreRespDTO

	// AI 处理结果
	AISkuMapping *AISkuMappingResponse

	// 模板信息
	TemplateInfo            any
	UserInputParentSpecList any
	InputMaxSpecNum         int
	SingleSpecValueNum      int

	// 处理结果
	SubmitResult  any
	SaveResult    any
	PublishResult any

	// 提交相关标志
	SavedToDraft bool

	// 图片处理相关
	PaddedImages      map[string][]byte
	PaddedImageSizes  map[string][2]int
	CurrentSkuContext string

	// 变体和映射相关
	AsinSkuMap         map[string]string
	VariantAsins       []string
	CleanedTitle       string
	ProductDescription string

	// 业务规则相关
	ProfitRule *management_api.ProfitRuleRespDTO
	FilterRule *management_api.FilterRuleRespDTO

	// 价格查询相关
	PriceQueryResponse any

	// 提交和响应相关
	CommitDetail   any
	SubmitResponse any
	ProductData    any
}

// NewTemuTaskContext 创建TEMU任务上下文
func NewTemuTaskContext(ctx context.Context, task *model.Task) *TemuTaskContext {
	return &TemuTaskContext{
		DefaultTaskContext: pipeline.NewTaskContext(ctx, task),
	}
}

// GetAmazonProduct 实现 pipeline.AmazonContext
func (tc *TemuTaskContext) GetAmazonProduct() *model.Product {
	return tc.AmazonProduct
}

// SetAmazonProduct 实现 pipeline.AmazonContext
func (tc *TemuTaskContext) SetAmazonProduct(product *model.Product) {
	tc.AmazonProduct = product
}

// GetVariants 实现 pipeline.AmazonContext
func (tc *TemuTaskContext) GetVariants() []*model.Product {
	return tc.Variants
}

// SetVariants 实现 pipeline.AmazonContext
func (tc *TemuTaskContext) SetVariants(variants []*model.Product) {
	tc.Variants = variants
}

// AddVariant 实现 pipeline.AmazonContext
func (tc *TemuTaskContext) AddVariant(variant *model.Product) {
	tc.Variants = append(tc.Variants, variant)
}

var _ pipeline.AmazonContext = (*TemuTaskContext)(nil)
