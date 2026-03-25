package sku

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/image"

	"github.com/sirupsen/logrus"
)

// SkuParallelBuilder builds SKU payloads while offloading image work in parallel.
type SkuParallelBuilder struct {
	itemBuilder            *SkuItemBuilder
	parallelImageProcessor *image.ParallelImageProcessor
	logger                 *logrus.Entry
}

func NewSkuParallelBuilder(itemBuilder *SkuItemBuilder, maxWorkers int) *SkuParallelBuilder {
	return &SkuParallelBuilder{
		itemBuilder:            itemBuilder,
		parallelImageProcessor: image.NewParallelImageProcessor(maxWorkers),
		logger:                 logger.GetGlobalLogger("SkuParallelBuilder"),
	}
}

// BuildSkusWithParallelImages first prepares all images in parallel, then builds
// the remaining SKU fields serially and applies the image results at the end.
func (spb *SkuParallelBuilder) BuildSkusWithParallelImages(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	aiSkus []temucontext.AIGeneratedSku,
) ([]models.Sku, error) {
	if len(variants) == 0 {
		return []models.Sku{}, nil
	}

	runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
	if err != nil {
		spb.logger.Errorf("failed to build sku runtime: %v", err)
		runtime = &temucontext.SKUBuildRuntime{}
	}

	spb.logger.Infof("start building %d skus with parallel image processing", len(variants))

	spb.logger.Info("step 1/3: process variant images in parallel")
	imageResults, err := spb.parallelImageProcessor.ProcessVariantImagesParallel(temuCtx, variants)
	if err != nil {
		return nil, fmt.Errorf("parallel image processing failed: %w", err)
	}

	spb.logger.Info("step 2/3: build sku payloads without images")
	skus := make([]models.Sku, len(variants))
	for i, variant := range variants {
		var aiSku temucontext.AIGeneratedSku
		if i < len(aiSkus) {
			aiSku = aiSkus[i]
		}

		skus[i] = spb.buildSkuWithoutImages(runtime, temuCtx, variant, aiSku)
		spb.logger.Infof("sku[%d] base payload built for asin=%s", i, variant.Asin)
	}

	spb.logger.Info("step 3/3: apply processed image results")
	spb.parallelImageProcessor.ApplyImageResults(skus, imageResults)

	spb.logger.Infof("parallel sku build completed: count=%d", len(skus))
	return skus, nil
}

func (spb *SkuParallelBuilder) buildSkuWithoutImages(
	runtime *temucontext.SKUBuildRuntime,
	temuCtx *temucontext.TemuTaskContext,
	variant *model.Product,
	aiSku temucontext.AIGeneratedSku,
) models.Sku {
	input, err := buildSKUVariantBuildInputWithRuntime(runtime, variant, aiSku)
	if err != nil {
		spb.logger.Errorf("failed to build sku variant input: %v", err)
		return models.Sku{}
	}

	pricingInfo := spb.itemBuilder.buildSkuPricingInfo(input.Runtime, temuCtx, input.Variant)
	outSkuSN := spb.itemBuilder.generateSkuFromRuntime(input.Runtime, input.Variant.Asin)
	temuCtx.SetAsinSkuMap(spb.itemBuilder.saveAsinSkuMappingWithRuntime(input.Runtime, outSkuSN, input.Variant.Asin))

	specList := spb.itemBuilder.buildSkuSpecList(input.AISKU)
	packagingInfo := spb.itemBuilder.buildSkuPackagingInfo(input.Variant, input.AISKU)
	expressInfo := spb.itemBuilder.buildSkuExpressInfo(input.Variant, input.AISKU)

	return models.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         []models.ImageInfo{},
		CarouselGallery:          []models.ImageInfo{},
		FoodIngredientGallery:    []models.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", pricingInfo.quantity),
		ProductExpressInfo:       expressInfo.productExpressInfo,
		SupplierPriceStr:         pricingInfo.supplierPriceStr,
		OutSkuSN:                 outSkuSN,
		MultiplePackage:          packagingInfo.multiplePackage,
		OriginNetContentNumber:   packagingInfo.originNetContentNumber,
		NetContentUnitCode:       packagingInfo.netContentUnitCode,
		MaxRetailPriceStr:        pricingInfo.marketPriceStr,
		SupplierPrice:            pricingInfo.finalSalePrice,
		SkuPriceDocuments:        make(map[string]any),
		MarketPrice:              pricingInfo.marketPrice,
		MarketPriceStr:           pricingInfo.marketPriceStr,
	}
}
