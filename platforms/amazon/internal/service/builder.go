// Package service 提供Amazon平台管道构建服务
package service

import (
	"task-processor/platforms/amazon/internal/handler"
	"task-processor/platforms/amazon/internal/model"

	"github.com/sirupsen/logrus"
)

// PipelineBuilder 管道构建器
type PipelineBuilder struct {
	services *model.Services
	logger   *logrus.Entry
}

// NewPipelineBuilder 创建管道构建器
func NewPipelineBuilder(services *model.Services) *PipelineBuilder {
	return &PipelineBuilder{
		services: services,
		logger:   logrus.WithField("service", "PipelineBuilder"),
	}
}

// BuildAmazonPipeline 构建Amazon上架管道
func (pb *PipelineBuilder) BuildAmazonPipeline() *PipelineService {
	pipeline := NewPipelineService()

	// 按照Amazon上架流程顺序添加处理器
	pb.addInitHandlers(pipeline)       // 1-5: 初始化阶段
	pb.addValidationHandlers(pipeline) // 6-10: 验证阶段
	pb.addProcessingHandlers(pipeline) // 11-15: 处理阶段

	pb.logger.Infof("Amazon管道构建完成，共 %d 个处理器", pipeline.GetHandlerCount())
	return pipeline
}

// addInitHandlers 添加初始化阶段处理器
func (pb *PipelineBuilder) addInitHandlers(pipeline *PipelineService) {
	// 1. 店铺信息处理器
	pipeline.AddHandler(handler.NewStoreInfoHandler())

	// 2. 数据解析处理器
	pipeline.AddHandler(handler.NewDataParserHandler())

	// 3. 产品数据处理器
	pipeline.AddHandler(handler.NewProductDataHandler())

	// 4. 产品类型推荐处理器
	pipeline.AddHandler(handler.NewProductTypeHandler())

	// 5. 属性映射处理器
	pipeline.AddHandler(handler.NewAttributeMapperHandler())
}

// addValidationHandlers 添加验证阶段处理器
func (pb *PipelineBuilder) addValidationHandlers(pipeline *PipelineService) {
	// 6. 验证处理器
	pipeline.AddHandler(handler.NewValidationHandler())
}

// addProcessingHandlers 添加处理阶段处理器
func (pb *PipelineBuilder) addProcessingHandlers(pipeline *PipelineService) {
	// 7. 图片处理器
	pipeline.AddHandler(handler.NewImageHandler())

	// 8. 变体处理器
	pipeline.AddHandler(handler.NewVariantHandler())

	// 9. Listing创建处理器
	pipeline.AddHandler(handler.NewListingHandler())

	// 10. 价格设置处理器
	pipeline.AddHandler(handler.NewPricingHandler())

	// 11. 库存设置处理器
	pipeline.AddHandler(handler.NewInventoryHandler())
}
