package productdata

import (
	"context"
	"fmt"
	"strings"

	appProduct "task-processor/internal/app/crawler/fetcher"
	"task-processor/internal/app/ports"
	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/pkg/perf"
	"task-processor/internal/product"
	shein "task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

type VariantJsonDataHandler struct {
	logger         *logrus.Entry
	productFetcher appProduct.ProductFetcher
	amazonConfig   *config.AmazonConfig
}

func NewVariantJsonDataHandler(
	rawJsonDataClient product.RawJsonDataClient,
	cfg *config.Config,
	amazonProcessor ports.ProductSource,
	rabbitmqClient *rabbitmq.Client,
) *VariantJsonDataHandler {
	logger := coreLogger.GetGlobalLogger("VariantJsonDataHandler")
	factory := appProduct.NewFetcherFactory()

	var fetcher appProduct.ProductFetcher
	var err error
	if amazonProcessor != nil {
		fetcher, err = factory.CreateFetcherFromConfig(cfg, rawJsonDataClient, amazonProcessor, rabbitmqClient)
		if err != nil {
			logger.Errorf("create product fetcher failed, fallback to local fetcher: %v", err)
			fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, amazonProcessor)
		}
	} else {
		fetcher = product.NewProductFetcher(rawJsonDataClient, &cfg.Amazon, nil)
	}

	return &VariantJsonDataHandler{logger: logger, productFetcher: fetcher, amazonConfig: &cfg.Amazon}
}

func (h *VariantJsonDataHandler) Name() string {
	return "variant_json_data"
}

func (h *VariantJsonDataHandler) Handle(ctx *shein.TaskContext) error {
	tracker := perf.NewTracker("variant_json_data", h.logger)
	defer tracker.Finish()

	if ctx.Task == nil {
		return fmt.Errorf("task is nil")
	}

	mainProductAsin := ctx.Task.ProductID
	variantAsins := getAsinListFromContext(ctx, mainProductAsin, h.logger)
	if len(variantAsins) == 0 {
		h.logger.Infof("no variants found for product %s", mainProductAsin)
		ctx.SetVariants([]model.Product{})
		return nil
	}
	if len(variantAsins) > 100 {
		return shein.NewNonRetryableError("too many variant ASINs", nil)
	}

	tracker.StartStep("fetch_variants")
	req := &product.FetchRequest{
		TenantID:   ctx.Task.TenantID,
		Platform:   ctx.Task.GetSourcePlatformOrDefault(),
		Region:     ctx.Task.Region,
		StoreID:    ctx.Task.StoreID,
		CategoryID: ctx.Task.CategoryID,
		Creator:    ctx.Task.Creator,
	}
	variants, err := h.productFetcher.FetchVariants(context.Background(), req, variantAsins)
	if err != nil {
		return fmt.Errorf("fetch variants failed: %w", err)
	}
	tracker.EndStep()

	variantList := make([]model.Product, 0, len(variants))
	for _, v := range variants {
		if v != nil {
			variantList = append(variantList, *v)
		}
	}
	ctx.SetVariants(variantList)
	h.logger.Infof("loaded variants: %d/%d", len(variantList), len(variantAsins))
	return nil
}

func getAsinListFromContext(ctx *shein.TaskContext, mainProductAsin string, logger *logrus.Entry) []string {
	if len(ctx.AsinSkuMap) > 0 {
		return getAsinListFromMap(ctx.AsinSkuMap, mainProductAsin)
	}

	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
		asins := make([]string, 0, len(ctx.AmazonProduct.Variations))
		for _, variation := range ctx.AmazonProduct.Variations {
			if variation.Asin != "" {
				asins = append(asins, variation.Asin)
			}
		}
		return asins
	}

	logger.Debug("no variant ASINs found in context")
	return []string{}
}

func getAsinListFromMap(asinSkuMap map[string]string, mainProductAsin string) []string {
	if len(asinSkuMap) == 0 {
		return []string{}
	}

	asinList := make([]string, 0, len(asinSkuMap))
	normalizedMainAsin := strings.TrimSpace(strings.ToUpper(mainProductAsin))
	for asin := range asinSkuMap {
		normalizedAsin := strings.TrimSpace(strings.ToUpper(asin))
		if normalizedAsin == "" {
			continue
		}
		asinList = append(asinList, asin)
		_ = normalizedMainAsin
	}
	return asinList
}

func (h *VariantJsonDataHandler) Shutdown() {
	h.logger.Debug("VariantJsonDataHandler closed")
}

func GetVariantByAsinFromVariants(variants *[]model.Product, asin string) *model.Product {
	if variants == nil {
		return nil
	}
	for _, variant := range *variants {
		if variant.Asin == asin {
			return &variant
		}
	}
	return nil
}
