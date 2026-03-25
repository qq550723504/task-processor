package sku

import (
	"fmt"
	"strings"

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
	finalSalePrice := spb.itemBuilder.priceHandler.CalculateVariantPriceWithRuntime(runtime, temuCtx, variant)
	outSkuSN := spb.itemBuilder.generateSkuFromRuntime(runtime, variant.Asin)
	temuCtx.SetAsinSkuMap(spb.itemBuilder.saveAsinSkuMappingWithRuntime(runtime, outSkuSN, variant.Asin))

	specList := spb.buildSpecList(aiSku)
	originNetContentNumber, netContentUnitCode := spb.extractNetContentInfo(variant, aiSku)
	quantity := spb.itemBuilder.priceHandler.GetDefaultStockWithRuntime(runtime)
	weight, length, width, height := spb.buildProductExpressInfo(variant, aiSku)
	multiplePackage := spb.buildMultiplePackage(aiSku)

	marketPrice := finalSalePrice * 2
	marketPriceStr := fmt.Sprintf("%.2f", float64(finalSalePrice)*2/100)

	return models.Sku{
		Spec:                     specList,
		Currency:                 "USD",
		UseEstimateSupplierPrice: true,
		DimensionGallery:         []models.ImageInfo{},
		CarouselGallery:          []models.ImageInfo{},
		FoodIngredientGallery:    []models.ImageInfo{},
		Quantity:                 fmt.Sprintf("%d", quantity),
		ProductExpressInfo: models.ProductExpressInfo{
			WeightInfo: models.WeightInfo{Weight: weight},
			VolumeInfo: models.VolumeInfo{Length: length, Width: width, Height: height},
		},
		SupplierPriceStr:       fmt.Sprintf("%.2f", float64(finalSalePrice)/100),
		OutSkuSN:               outSkuSN,
		MultiplePackage:        multiplePackage,
		OriginNetContentNumber: originNetContentNumber,
		NetContentUnitCode:     netContentUnitCode,
		MaxRetailPriceStr:      marketPriceStr,
		SupplierPrice:          finalSalePrice,
		SkuPriceDocuments:      make(map[string]any),
		MarketPrice:            marketPrice,
		MarketPriceStr:         marketPriceStr,
	}
}

func (spb *SkuParallelBuilder) buildSpecList(aiSku temucontext.AIGeneratedSku) []models.SpecInfo {
	specList := spb.itemBuilder.deduplicateSpecs(convertSpecInfos(aiSku.Spec))

	hasTemporaryIDs := false
	for i, specInfo := range specList {
		if strings.HasPrefix(specInfo.SpecID, "TEMP_") {
			spb.logger.Errorf(
				"found unresolved temporary spec id[%d]: spec_id=%s spec_name=%s parent_spec_id=%s",
				i, specInfo.SpecID, specInfo.SpecName, specInfo.ParentSpecID,
			)
			hasTemporaryIDs = true
		}
	}

	if hasTemporaryIDs {
		spb.logger.Error("unresolved temporary spec ids remain after spec resolution")
		return []models.SpecInfo{}
	}

	if err := spb.itemBuilder.specHandler.ValidateSpecs(specList); err != nil {
		spb.logger.Errorf("spec validation failed: %v", err)
		spb.logger.Error("sku payload cannot be built because the spec list is invalid")
	}

	return specList
}

func (spb *SkuParallelBuilder) buildProductExpressInfo(
	variant *model.Product,
	aiSku temucontext.AIGeneratedSku,
) (weight, length, width, height string) {
	return spb.itemBuilder.buildProductExpressInfo(variant, aiSku)
}

func (spb *SkuParallelBuilder) buildMultiplePackage(aiSku temucontext.AIGeneratedSku) models.MultiplePackage {
	return spb.itemBuilder.buildMultiplePackage(aiSku)
}

func (spb *SkuParallelBuilder) extractNetContentInfo(
	variant *model.Product,
	aiSku temucontext.AIGeneratedSku,
) (string, int) {
	return spb.itemBuilder.extractNetContentInfo(variant, aiSku)
}
