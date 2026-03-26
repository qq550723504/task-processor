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

type RawJsonDataHandler struct {
	fetcher appProduct.ProductFetcher
	logger  *logrus.Entry
}

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
		logger.Errorf("create product fetcher failed, fallback to local fetcher: %v", err)
		fetcher = domainProduct.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
	}

	if stats := fetcher.GetStats(); stats != nil {
		logger.Infof("SHEIN product fetcher type: %v", stats["type"])
	}

	return &RawJsonDataHandler{fetcher: fetcher, logger: logger}
}

func (h *RawJsonDataHandler) Name() string {
	return "raw_json_data"
}

func isProductNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*model.ProductNotFoundError); ok {
		return true
	}

	errorStr := strings.ToLower(err.Error())
	productNotFoundPatterns := []string{
		"page not found",
		"404",
		"asin",
	}
	for _, pattern := range productNotFoundPatterns {
		if strings.Contains(errorStr, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func (h *RawJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	req := &domainProduct.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		ProductID:  ctx.Task.ProductID,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}

	amazonProduct, err := h.fetcher.FetchProduct(ctx.Context, req)
	if err != nil {
		if isProductNotFoundError(err) {
			return shein.NewNonRetryableError("Amazon product not found", err)
		}
		return shein.NewRetryableError("fetch product data failed", err)
	}

	ctx.SetAmazonProduct(amazonProduct)
	return nil
}

func (h *RawJsonDataHandler) Shutdown() {
	h.logger.Debug("RawJsonDataHandler closed")
}
