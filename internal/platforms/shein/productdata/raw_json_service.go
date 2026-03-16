package productdata

import (
	"strings"
	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	domainProduct "task-processor/internal/domain/product"
	"task-processor/internal/infra/rabbitmq"
	shein "task-processor/internal/platforms/shein"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandler 获取原始Json数据处理器
type RawJsonDataHandler struct {
	fetcher appProduct.ProductFetcher
}

// NewRawJsonDataHandler 创建新的获取原始Json数据处理器（支持分布式获取器）
func NewRawJsonDataHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor any,
	rabbitmqClient *rabbitmq.Client,
) *RawJsonDataHandler {
	logger := logrus.WithField("handler", "RawJsonDataHandler")

	var ap *amazon.AmazonProcessor
	if amazonProcessor != nil {
		if processor, ok := amazonProcessor.(*amazon.AmazonProcessor); ok {
			ap = processor
			logger.Info("[SHEIN] 使用共享的 Amazon 爬虫实例")
		}
	} else if cfg != nil {
		ap = amazon.NewAmazonProcessor(cfg)
		logger.Info("[SHEIN] Amazon 爬虫已启用")
	}

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, ap, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, ap)
	}

	logger.Infof("✅ SHEIN产品获取器创建成功，类型: %s", factory.GetRecommendedFetcher(cfg))

	return &RawJsonDataHandler{fetcher: fetcher}
}

// Name 返回处理器名称
func (h *RawJsonDataHandler) Name() string {
	return "获取原始Json数据"
}

// isProductNotFoundError 检查错误是否为产品不存在错误
func isProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为 ProductNotFoundError 类型
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return true
	}

	// 检查错误信息（使用包含匹配，而不是精确匹配）
	errorStr := strings.ToLower(err.Error())
	productNotFoundPatterns := []string{
		"产品页面不存在",
		"产品页面缺少必要元素",
		"page not found",
		"页面不存在(404)",
		"页面不存在",
		"产品不存在",
		"asin无效",
		"产品已下架",
	}

	for _, pattern := range productNotFoundPatterns {
		if strings.Contains(errorStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func (h *RawJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	logrus.Infof("开始获取原始JSON数据: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)

	// 使用公共ProductFetcher获取产品数据
	req := &domainProduct.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.SourcePlatform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(req)
	if err != nil {
		// 检查是否为产品不存在错误
		if isProductNotFoundError(err) {
			logrus.Warnf("产品不存在，不需要重试: ProductID=%s, Error=%v", ctx.Task.ProductID, err)
			return shein.NewNonRetryableError("Amazon产品不存在", err)
		}
		// 其他错误（如超时、网络错误）可以重试
		return shein.NewRetryableError("获取产品数据失败", err)
	}

	// 将原始JSON数据存储到上下文中
	ctx.AmazonProduct = amazonProduct

	return nil
}

// Shutdown 关闭处理器，释放资源（现在由共享的Amazon处理器管理）
func (h *RawJsonDataHandler) Shutdown() {
	// Amazon处理器由外部管理，不需要在这里关闭
	logrus.Debug("RawJsonDataHandler 关闭（Amazon处理器由外部管理）")
}
