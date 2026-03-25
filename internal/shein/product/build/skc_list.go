package build

import (
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	productapi "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/skc"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

type BuildSkcListHandler struct {
	imageDownloader interface {
		DownloadImage(url string) ([]byte, error)
	}
	strategyHandler *skc.AttributeStrategyHandler
	skcBuilder      *skc.SKCBuilder
}

func NewBuildSkcListHandler(imageDownloader interface {
	DownloadImage(url string) ([]byte, error)
}, client openaiClient.ChatCompleter) *BuildSkcListHandler {
	imageProcessor := image.NewImageProcessor(imageDownloader)
	attributeMapper := attribute.NewAttributeMapper()
	variantMatcher := variant.NewVariantMatcher()
	skuBuilder := sku.NewSKUBuilder(variantMatcher)
	skcBuilder := skc.NewSKCBuilder(imageProcessor, attributeMapper, variantMatcher, skuBuilder, client)
	strategyHandler := skc.NewAttributeStrategyHandler()
	return &BuildSkcListHandler{imageDownloader: imageDownloader, strategyHandler: strategyHandler, skcBuilder: skcBuilder}
}

func (h *BuildSkcListHandler) Name() string {
	return "build_skc_list"
}

func (h *BuildSkcListHandler) Handle(ctx *shein.TaskContext) error {
	skcInput := skc.NewSKCBuildInput(ctx)
	if err := skcInput.Validate(); err != nil {
		return err
	}

	output, err := h.skcBuilder.BuildSKCListWithSpecAdaptation(skcInput, ctx, h.strategyHandler)
	if err != nil {
		return err
	}
	if output.IsEmpty() {
		return shein.NewNonRetryableError("SKC list is empty", nil)
	}

	ctx.UpdateProductData(func(productData *productapi.Product) {
		productData.SKCList = output.SKCList
		productData.CustomAttributeRelation = output.CustomAttributeRelations
	})
	return nil
}
