package data

import (
	"fmt"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/rabbitmq"
	management_api "task-processor/internal/pkg/management/api"
	shein_model "task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// SubmitRawJsonDataHandler 提交原始JSON数据到服务器缓存处理器（使用公共缓存逻辑）
type SubmitRawJsonDataHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcherInterface
}

// NewSubmitRawJsonDataHandler 创建新的提交原始JSON数据处理器（支持分布式获取器）
func NewSubmitRawJsonDataHandler(rawJsonDataClient interface {
	GetRawJsonData(req *management_api.RawJsonDataReqDTO) (*management_api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *management_api.RawJsonDataCreateReqDTO) (int64, error)
}, cfg *config.Config, amazonProcessor interface{}, rabbitmqClient *rabbitmq.Client) *SubmitRawJsonDataHandler {
	logger := logrus.WithField("handler", "SubmitRawJsonDataHandler")

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, ap)
	}

	return &SubmitRawJsonDataHandler{
		logger:  logger,
		fetcher: fetcher,
	}
}

// Name 返回处理器名称
func (h *SubmitRawJsonDataHandler) Name() string {
	return "提交原始JSON数据到服务器缓存"
}

// Handle 执行提交原始JSON数据处理
func (h *SubmitRawJsonDataHandler) Handle(ctx *shein_model.TaskContext) error {
	// 检查是否已获取产品数据
	if ctx.AmazonProduct == nil {
		h.logger.Warn("产品数据未获取，跳过提交原始JSON数据步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	if ctx.Task.ProductID == "" {
		return fmt.Errorf("产品ID为空")
	}

	// 构建缓存请求
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	// 使用公共缓存逻辑缓存产品数据
	if err := h.fetcher.CacheProduct(req, ctx.AmazonProduct); err != nil {
		h.logger.Warnf("⚠️ 缓存产品数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 产品数据已缓存: ProductID=%s", ctx.Task.ProductID)

	// 如果有变体数据，也记录变体数量
	if ctx.Variants != nil && len(*ctx.Variants) > 0 {
		h.logger.Infof("产品包含 %d 个变体，变体数据将在后续处理中提交", len(*ctx.Variants))
	}

	return nil
}

// SubmitVariantRawJsonDataHandler 提交变体原始JSON数据到服务器缓存处理器（使用公共缓存逻辑）
type SubmitVariantRawJsonDataHandler struct {
	logger  *logrus.Entry
	fetcher appProduct.ProductFetcherInterface
}

// NewSubmitVariantRawJsonDataHandler 创建新的提交变体原始JSON数据处理器（支持分布式获取器）
func NewSubmitVariantRawJsonDataHandler(rawJsonDataClient interface {
	GetRawJsonData(req *management_api.RawJsonDataReqDTO) (*management_api.RawJsonDataRespDTO, error)
	CreateRawJsonData(req *management_api.RawJsonDataCreateReqDTO) (int64, error)
}, cfg *config.Config, amazonProcessor interface{}, rabbitmqClient *rabbitmq.Client) *SubmitVariantRawJsonDataHandler {
	logger := logrus.WithField("handler", "SubmitVariantRawJsonDataHandler")

	// 提取Amazon处理器
	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
		}
	}

	// 使用工厂模式创建获取器
	factory := appProduct.NewFetcherFactory()

	// 根据配置创建获取器
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		// 降级到本地获取器
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, ap)
	}

	return &SubmitVariantRawJsonDataHandler{
		logger:  logger,
		fetcher: fetcher,
	}
}

// Name 返回处理器名称
func (h *SubmitVariantRawJsonDataHandler) Name() string {
	return "提交变体原始JSON数据到服务器缓存"
}

// Handle 执行提交变体原始JSON数据处理
func (h *SubmitVariantRawJsonDataHandler) Handle(ctx *shein_model.TaskContext) error {
	// 检查是否有变体数据
	if ctx.Variants == nil || len(*ctx.Variants) == 0 {
		h.logger.Debug("没有变体数据，跳过提交变体原始JSON数据步骤")
		return nil
	}

	// 检查任务上下文中的必要数据
	if ctx.Task == nil {
		return fmt.Errorf("任务信息为空")
	}

	// 构建缓存请求
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.Platform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	// 将 []amazon.Product 转换为 []*amazon.Product
	variants := make([]*model.Product, len(*ctx.Variants))
	for i := range *ctx.Variants {
		variants[i] = &(*ctx.Variants)[i]
	}

	// 使用公共缓存逻辑批量缓存变体数据
	if err := h.fetcher.CacheVariants(req, variants); err != nil {
		h.logger.Warnf("⚠️ 缓存变体数据失败: %v", err)
		// 缓存失败不影响主流程，只记录警告
		return nil
	}

	h.logger.Infof("✅ 变体数据已缓存: 数量=%d", len(*ctx.Variants))
	return nil
}
