package productdata

import (
	"strings"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// RawJsonDataHandler 获取原始Json数据处理器
type RawJsonDataHandler struct {
	fetcher appProduct.ProductFetcher
	logger  *logrus.Entry
}

// NewRawJsonDataHandler 创建新的获取原始Json数据处理器（支持分布式获取器）
func NewRawJsonDataHandler(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor domainProduct.AmazonScraper,
	rabbitmqClient *rabbitmq.Client,
) *RawJsonDataHandler {
	logger := coreLogger.GetGlobalLogger("RawJsonDataHandler")

	factory := appProduct.NewFetcherFactory()
	fetcher, err := factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
	if err != nil {
		logger.Errorf("创建产品获取器失败，使用本地获取器: %v", err)
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	if amazonProcessor != nil {
		logger.Info("[SHEIN] 使用共享的 Amazon 爬虫实例")
	} else {
		logger.Info("[SHEIN] Amazon 爬虫未提供，仅使用缓存模式")
	}

	logger.Infof("✅ SHEIN产品获取器创建成功，类型: %s", factory.GetRecommendedFetcher(cfg))

	return &RawJsonDataHandler{fetcher: fetcher, logger: logger}
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

	if _, ok := err.(*model.ProductNotFoundError); ok {
		return true
	}

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
	h.logger.Infof("开始获取原始JSON数据: ProductID=%s, Region=%s", ctx.Task.ProductID, ctx.Task.Region)

	req := &domainProduct.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.SourcePlatform,
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(ctx.Context, req)
	if err != nil {
		if isProductNotFoundError(err) {
			h.logger.Warnf("产品不存在，不需要重试: ProductID=%s, Error=%v", ctx.Task.ProductID, err)
			return shein.NewNonRetryableError("Amazon产品不存在", err)
		}
		return shein.NewRetryableError("获取产品数据失败", err)
	}

	ctx.AmazonProduct = amazonProduct

	return nil
}

// Shutdown 关闭处理器，释放资源
func (h *RawJsonDataHandler) Shutdown() {
	h.logger.Debug("RawJsonDataHandler 关闭（Amazon处理器由外部管理）")
}
