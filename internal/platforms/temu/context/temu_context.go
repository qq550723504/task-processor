// Package context 提供TEMU平台的强类型任务上下文
package context

import (
	"context"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	commonPipeline "task-processor/internal/pipeline"
	management_api "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/types"
)

// TemuTaskContext TEMU平台特定的任务上下文
type TemuTaskContext struct {
	*commonPipeline.DefaultTaskContext // 组合基础上下文

	// TEMU特定字段（直接访问，无需getter/setter）
	AmazonProcessor *amazon.AmazonProcessor
	APIClient       api.APIClientInterface // 使用接口避免循环依赖

	// TEMU特定产品数据
	TemuProduct *api.Product
	StoreInfo   *management_api.StoreRespDTO

	// AI处理结果
	AISkuMapping *types.AISkuMappingResponse

	// 模板信息
	TemplateInfo            interface{} // 模板信息
	UserInputParentSpecList interface{} // 用户输入父规格列表
	InputMaxSpecNum         int         // 最大规格数量
	SingleSpecValueNum      int         // 单规格值数量

	// 处理结果
	SubmitResult  interface{}
	SaveResult    interface{}
	PublishResult interface{}

	// 提交相关标志
	SavedToDraft bool // 是否已保存到草稿箱

	// 图片处理相关
	PaddedImages      map[string][]byte // 填充后的图片数据
	PaddedImageSizes  map[string][2]int // 填充后的图片尺寸
	CurrentSkuContext string            // 当前SKU上下文键

	// 变体和映射相关
	AsinSkuMap         map[string]string // ASIN到SKU的映射
	VariantAsins       []string          // 变体ASIN列表
	CleanedTitle       string            // 清理后的标题
	ProductDescription string            // 产品描述

	// 业务规则相关
	ProfitRule *management_api.ProfitRuleRespDTO // 利润规则
	FilterRule *management_api.FilterRuleRespDTO // 筛选规则

	// 价格查询相关
	PriceQueryResponse interface{} // 价格查询响应

	// 提交和响应相关
	CommitDetail   interface{} // 提交详情
	SubmitResponse interface{} // 提交响应数据
	ProductData    interface{} // 产品数据
}

// NewTemuTaskContext 创建TEMU任务上下文
func NewTemuTaskContext(ctx context.Context, task *model.Task) *TemuTaskContext {
	baseContext := commonPipeline.NewTaskContext(ctx, task).(*commonPipeline.DefaultTaskContext)
	return &TemuTaskContext{
		DefaultTaskContext: baseContext,
	}
}
