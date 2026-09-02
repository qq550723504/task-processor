package listingkit

import (
	"context"
	"time"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

type assembler struct {
	amazonBuilder              AmazonDraftBuilder
	sheinCategoryResolver      sheinpub.CategoryResolver
	sheinAttributeResolver     sheinpub.AttributeResolver
	sheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	sheinSizeHeaderResolver    sheinpub.SizeAttributeHeaderResolver
	sheinPricingPolicy         sheinpub.PricingPolicy
	sheinTitleOptimizer        AIChatCompleter
}

func NewAssembler(amazonBuilder AmazonDraftBuilder) Assembler {
	return NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: amazonBuilder})
}

type AssemblerConfig struct {
	AmazonBuilder              AmazonDraftBuilder
	SheinCategoryResolver      sheinpub.CategoryResolver
	SheinAttributeResolver     sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinSizeHeaderResolver    sheinpub.SizeAttributeHeaderResolver
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinTitleOptimizer        AIChatCompleter
}

func NewAssemblerWithConfig(config AssemblerConfig) Assembler {
	amazonBuilder := config.AmazonBuilder
	if amazonBuilder == nil {
		amazonBuilder = newAmazonDraftBuilder()
	}
	return &assembler{
		amazonBuilder:              amazonBuilder,
		sheinCategoryResolver:      config.SheinCategoryResolver,
		sheinAttributeResolver:     config.SheinAttributeResolver,
		sheinSaleAttributeResolver: config.SheinSaleAttributeResolver,
		sheinSizeHeaderResolver:    config.SheinSizeHeaderResolver,
		sheinPricingPolicy:         config.SheinPricingPolicy,
		sheinTitleOptimizer:        config.SheinTitleOptimizer,
	}
}

func (a *assembler) Assemble(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) *ListingKitResult {
	return a.assemble(task, product, approved)
}

func (a *assembler) AssembleForTargets(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) *ListingKitResult {
	return a.assemble(task, product, approved)
}

func (a *assembler) assemble(task *Task, product *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) *ListingKitResult {
	now := time.Now()
	result := initResult(task)
	result.UpdatedAt = now
	var canonicalProduct *canonical.Product
	if product != nil {
		cloned, err := cloneProductSnapshot(*product)
		if err == nil {
			result.CatalogProduct = &cloned
			canonicalProduct = canonicalProductFromApprovedAssets(cloned, approved)
		}
	}
	result.ApprovedAssetInventory = cloneApprovedAssetInventory(approved)
	result.CanonicalProduct = canonicalProduct
	result.Summary = buildSummary(task, canonicalProduct)

	if task == nil || task.Request == nil {
		return result
	}

	for _, platform := range task.Request.Platforms {
		switch platform {
		case "amazon":
			result.Amazon = &AmazonPackage{Draft: a.amazonBuilder.Build(task.Request, canonicalProduct)}
		case "shein":
			result.Shein = sheinpub.NewAssembler(a.buildSheinAssemblerConfig()).Build(buildSheinPublishRequestForTask(task, task.Request), canonicalProduct)
			refreshSheinReviewState(result.Shein, common.CollectReviewNotes(canonicalProduct)...)
		case "temu":
			result.Temu = buildTemuPackage(task.Request, canonicalProduct)
		case "walmart":
			result.Walmart = buildWalmartPackage(task.Request, canonicalProduct)
		}
	}

	return result
}

func buildSheinPublishRequest(req *GenerateRequest) *sheinpub.BuildRequest {
	return buildSheinPublishRequestForTask(nil, req)
}

func buildSheinPublishRequestForTask(task *Task, req *GenerateRequest) *sheinpub.BuildRequest {
	if req == nil {
		return &sheinpub.BuildRequest{}
	}
	var ctxIdentity RequestIdentity
	if task != nil {
		ctxIdentity = RequestIdentity{TenantID: task.TenantID, UserID: task.UserID}
	}
	// 使用 task 的 TenantID 构建 context,避免使用 context.Background()
	ctx := context.Background()
	if ctxIdentity.TenantID != "" {
		ctx = WithTenantID(ctx, ctxIdentity.TenantID)
	}
	productSize := ""
	if req.Options != nil && req.Options.SDS != nil {
		productSize = req.Options.SDS.ProductSize
	}
	return &sheinpub.BuildRequest{
		Country:            req.Country,
		Language:           req.Language,
		Text:               req.Text,
		BrandHint:          req.BrandHint,
		TargetCategoryHint: req.TargetCategoryHint,
		SheinStoreID:       req.SheinStoreID,
		ProductSize:        productSize,
		Context:            WithRequestIdentity(ctx, ctxIdentity),
	}
}

func buildSummary(task *Task, canonical *canonical.Product) *GenerationSummary {
	summary := &GenerationSummary{}
	if task != nil && task.Request != nil {
		summary.SourceType = detectSourceType(task.Request)
	}
	if canonical != nil {
		summary.ImageCount = len(canonical.Images)
		summary.VariantCount = len(canonical.Variants)
		summary.NeedsReview = canonical.NeedsReview
	}
	return summary
}

func (a *assembler) buildSheinAssemblerConfig() sheinpub.AssemblerConfig {
	return sheinpub.AssemblerConfig{
		CategoryResolver:            a.sheinCategoryResolver,
		AttributeResolver:           a.sheinAttributeResolver,
		SaleAttributeResolver:       a.sheinSaleAttributeResolver,
		SizeAttributeHeaderResolver: a.sheinSizeHeaderResolver,
		PricingPolicy:               a.sheinPricingPolicy,
		TitleOptimizer:              a.sheinTitleOptimizer,
	}
}
