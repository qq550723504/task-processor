// Package context TEMU平台特定的上下文定义
package context

import (
	"context"
	"task-processor/internal/common/amazon"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	commonPipeline "task-processor/internal/common/pipeline"
	"task-processor/internal/common/types"
	"task-processor/internal/platforms/temu/client"
	temutypes "task-processor/internal/platforms/temu/types"
)

// TemuTaskContext TEMU平台特定的任务上下文
type TemuTaskContext struct {
	*commonPipeline.BaseTaskContext // 组合基础上下文

	// TEMU特定字段
	AmazonProcessor *amazon.AmazonProcessor
	APIClient       *client.APIClient

	// 强类型字段
	AmazonProduct *model.Product
	TemuProduct   *temutypes.Product
	StoreInfo     *api.StoreRespDTO
	DataSource    string

	// 变体相关数据
	AmazonVariants []*model.Product

	// 处理结果
	SubmitResult  interface{}
	SaveResult    interface{}
	PublishResult interface{}
}

// NewTemuTaskContext 创建TEMU任务上下文
func NewTemuTaskContext(ctx context.Context, task *types.Task) *TemuTaskContext {
	return &TemuTaskContext{
		BaseTaskContext: commonPipeline.NewBaseTaskContext(ctx, task),
		AmazonVariants:  make([]*model.Product, 0),
	}
}

// SetAmazonProcessor 设置Amazon处理器
func (ttc *TemuTaskContext) SetAmazonProcessor(processor *amazon.AmazonProcessor) {
	ttc.AmazonProcessor = processor
}

// GetAmazonProcessor 获取Amazon处理器
func (ttc *TemuTaskContext) GetAmazonProcessor() *amazon.AmazonProcessor {
	return ttc.AmazonProcessor
}

// SetAPIClient 设置API客户端
func (ttc *TemuTaskContext) SetAPIClient(apiClient *client.APIClient) {
	ttc.APIClient = apiClient
}

// GetAPIClient 获取API客户端
func (ttc *TemuTaskContext) GetAPIClient() *client.APIClient {
	return ttc.APIClient
}

// SetAmazonProduct 设置Amazon产品数据
func (ttc *TemuTaskContext) SetAmazonProduct(product *model.Product) {
	ttc.AmazonProduct = product
}

// GetAmazonProduct 获取Amazon产品数据
func (ttc *TemuTaskContext) GetAmazonProduct() *model.Product {
	return ttc.AmazonProduct
}

// SetTemuProduct 设置TEMU产品数据
func (ttc *TemuTaskContext) SetTemuProduct(product *temutypes.Product) {
	ttc.TemuProduct = product
}

// GetTemuProduct 获取TEMU产品数据
func (ttc *TemuTaskContext) GetTemuProduct() *temutypes.Product {
	return ttc.TemuProduct
}

// SetStoreInfo 设置店铺信息
func (ttc *TemuTaskContext) SetStoreInfo(storeInfo *api.StoreRespDTO) {
	ttc.StoreInfo = storeInfo
}

// GetStoreInfo 获取店铺信息
func (ttc *TemuTaskContext) GetStoreInfo() *api.StoreRespDTO {
	return ttc.StoreInfo
}

// SetAmazonVariants 设置Amazon变体数据
func (ttc *TemuTaskContext) SetAmazonVariants(variants []*model.Product) {
	ttc.AmazonVariants = variants
}

// GetAmazonVariants 获取Amazon变体数据
func (ttc *TemuTaskContext) GetAmazonVariants() []*model.Product {
	return ttc.AmazonVariants
}

// AddAmazonVariant 添加单个Amazon变体
func (ttc *TemuTaskContext) AddAmazonVariant(variant *model.Product) {
	if ttc.AmazonVariants == nil {
		ttc.AmazonVariants = make([]*model.Product, 0)
	}
	ttc.AmazonVariants = append(ttc.AmazonVariants, variant)
}

// SetSubmitResult 设置提交结果
func (ttc *TemuTaskContext) SetSubmitResult(result interface{}) {
	ttc.SubmitResult = result
}

// GetSubmitResult 获取提交结果
func (ttc *TemuTaskContext) GetSubmitResult() interface{} {
	return ttc.SubmitResult
}

// SetSaveResult 设置保存结果
func (ttc *TemuTaskContext) SetSaveResult(result interface{}) {
	ttc.SaveResult = result
}

// GetSaveResult 获取保存结果
func (ttc *TemuTaskContext) GetSaveResult() interface{} {
	return ttc.SaveResult
}

// SetPublishResult 设置发布结果
func (ttc *TemuTaskContext) SetPublishResult(result interface{}) {
	ttc.PublishResult = result
}

// GetPublishResult 获取发布结果
func (ttc *TemuTaskContext) GetPublishResult() interface{} {
	return ttc.PublishResult
}

// SetDataSource 设置数据源标识
func (ttc *TemuTaskContext) SetDataSource(dataSource string) {
	ttc.DataSource = dataSource
}

// GetDataSource 获取数据源标识
func (ttc *TemuTaskContext) GetDataSource() string {
	return ttc.DataSource
}
