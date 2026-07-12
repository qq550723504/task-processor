package skc

import (
	"fmt"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/aicache"
	sheinattribute "task-processor/internal/shein/api/attribute"
	productapi "task-processor/internal/shein/api/product"
	sheintranslate "task-processor/internal/shein/api/translate"
	"task-processor/internal/shein/namelimit"
	sheinsale "task-processor/internal/shein/product/attribute/sale"
)

type SKCBuildInput struct {
	ProductData         *productapi.Product
	SaleAttributeOutput *sheinsale.SaleAttributeOutput
	AttributeTemplates  *sheinattribute.AttributeTemplateInfo
	Runtime             *SKCRuntimeInput
	VariantBuild        *SKCVariantBuildInput
	Validation          *SKCValidationInput
}

type SKCRuntimeInput struct {
	Region             string
	AmazonProduct      *model.Product
	Variants           []model.Product
	AttributeTemplates *sheinattribute.AttributeTemplateInfo
	AsinSkuMap         map[string]string
	TranslateAPI       *sheintranslate.Client
	AICache            *aicache.Cache
	NameLengthLimits   namelimit.Limits
}

type SKCVariantBuildInput struct {
	CategoryID        int
	AttributeAPI      *sheinattribute.Client
	SaleAttributeData shein.ResultSaleAttribute
	WarehouseCode     string
}

type SKCValidationInput struct {
	StrategyData       shein.ResultSaleAttribute
	AttributeTemplates *sheinattribute.AttributeTemplateInfo
}

func NewSKCBuildInput(ctx *shein.TaskContext) *SKCBuildInput {
	input := &SKCBuildInput{
		ProductData:        ctx.ProductData,
		AttributeTemplates: ctx.AttributeTemplates,
		Runtime:            newSKCRuntimeInput(ctx),
		VariantBuild:       newSKCVariantBuildInput(ctx),
		Validation:         newSKCValidationInput(ctx),
	}
	if ctx.SaleSpecResult != nil {
		input.SaleAttributeOutput = sheinsale.NewSaleAttributeOutput(*ctx.SaleSpecResult)
	}
	return input
}

func newSKCRuntimeInput(ctx *shein.TaskContext) *SKCRuntimeInput {
	input := &SKCRuntimeInput{
		AttributeTemplates: ctx.AttributeTemplates,
		AmazonProduct:      ctx.AmazonProduct,
		AsinSkuMap:         ctx.AsinSkuMap,
		TranslateAPI:       ctx.TranslateAPI,
		AICache:            ctx.AICache,
		NameLengthLimits:   ctx.ProductNameLengthLimits,
	}
	if ctx.Task != nil {
		input.Region = ctx.Task.Region
	}
	if ctx.Variants != nil {
		input.Variants = append([]model.Product(nil), (*ctx.Variants)...)
	}
	return input
}

func newSKCVariantBuildInput(ctx *shein.TaskContext) *SKCVariantBuildInput {
	input := &SKCVariantBuildInput{AttributeAPI: ctx.AttributeAPI}
	if ctx.ProductData != nil {
		input.CategoryID = ctx.ProductData.CategoryID
	}
	if ctx.SaleSpecResult != nil {
		input.SaleAttributeData = *ctx.SaleSpecResult
	}
	if ctx.Warehouses != nil && len(ctx.Warehouses.Data) > 0 {
		input.WarehouseCode = ctx.Warehouses.Data[0].WarehouseCode
	}
	return input
}

func newSKCValidationInput(ctx *shein.TaskContext) *SKCValidationInput {
	input := &SKCValidationInput{AttributeTemplates: ctx.AttributeTemplates}
	if ctx.SaleSpecResult != nil {
		input.StrategyData = *ctx.SaleSpecResult
	}
	return input
}

func (in *SKCBuildInput) Validate() error {
	if in.ProductData == nil {
		return fmt.Errorf("product data is not initialized")
	}
	if in.SaleAttributeOutput == nil {
		return fmt.Errorf("sale attribute output is not initialized")
	}
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if in.Runtime == nil {
		return fmt.Errorf("SKC runtime input is not initialized")
	}
	if in.VariantBuild == nil {
		return fmt.Errorf("SKC variant build input is not initialized")
	}
	if in.Validation == nil {
		return fmt.Errorf("SKC validation input is not initialized")
	}
	if err := in.Runtime.Validate(); err != nil {
		return err
	}
	if err := in.VariantBuild.Validate(); err != nil {
		return err
	}
	return in.Validation.Validate()
}

func (in *SKCRuntimeInput) Validate() error {
	if in.AmazonProduct == nil {
		return fmt.Errorf("amazon product is not initialized")
	}
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if in.TranslateAPI == nil {
		return fmt.Errorf("translate API is not initialized")
	}
	return nil
}

func (in *SKCVariantBuildInput) Validate() error {
	if in.CategoryID == 0 {
		return fmt.Errorf("category ID is not initialized")
	}
	if in.AttributeAPI == nil {
		return fmt.Errorf("attribute API is not initialized")
	}
	if len(in.SaleAttributeData.Variants) == 0 {
		return fmt.Errorf("sale attribute data contains no variants")
	}
	if in.WarehouseCode == "" {
		return fmt.Errorf("warehouse code is not initialized")
	}
	return nil
}

func (in *SKCValidationInput) Validate() error {
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if len(in.StrategyData.Variants) == 0 {
		return fmt.Errorf("sale attribute data contains no variants")
	}
	return nil
}
