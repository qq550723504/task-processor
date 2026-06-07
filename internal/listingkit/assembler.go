package listingkit

import (
	"context"
	"time"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type assembler struct {
	amazonBuilder              AmazonDraftBuilder
	sheinCategoryResolver      sheinpub.CategoryResolver
	sheinAttributeResolver     sheinpub.AttributeResolver
	sheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	sheinPricingPolicy         sheinpub.PricingPolicy
	sheinTitleOptimizer        openaiclient.ChatCompleter
}

func NewAssembler(amazonBuilder AmazonDraftBuilder) Assembler {
	return NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: amazonBuilder})
}

type AssemblerConfig struct {
	AmazonBuilder              AmazonDraftBuilder
	SheinCategoryResolver      sheinpub.CategoryResolver
	SheinAttributeResolver     sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinTitleOptimizer        openaiclient.ChatCompleter
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
		sheinPricingPolicy:         config.SheinPricingPolicy,
		sheinTitleOptimizer:        config.SheinTitleOptimizer,
	}
}

func (a *assembler) Assemble(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *ListingKitResult {
	now := time.Now()
	result := initResult(task)
	result.UpdatedAt = now
	result.CanonicalProduct = canonical
	result.ImageAssets = image
	result.Summary = buildSummary(task, canonical, image)

	if task == nil || task.Request == nil {
		return result
	}

	for _, platform := range task.Request.Platforms {
		switch platform {
		case "amazon":
			result.Amazon = &AmazonPackage{Draft: a.amazonBuilder.Build(task.Request, canonical, image)}
		case "shein":
			result.Shein = sheinpub.NewAssembler(a.buildSheinAssemblerConfig()).Build(buildSheinPublishRequestForTask(task, task.Request), canonical, image)
			refreshSheinReviewState(result.Shein, collectReviewNotes(canonical, image)...)
		case "temu":
			result.Temu = buildTemuPackage(task.Request, canonical, image)
		case "walmart":
			result.Walmart = buildWalmartPackage(task.Request, canonical, image)
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
	var ctxIdentity openaiclient.Identity
	if task != nil {
		ctxIdentity = openaiclient.Identity{TenantID: task.TenantID, UserID: task.UserID}
	}
	return &sheinpub.BuildRequest{
		Country:            req.Country,
		Language:           req.Language,
		Text:               req.Text,
		BrandHint:          req.BrandHint,
		TargetCategoryHint: req.TargetCategoryHint,
		SheinStoreID:       req.SheinStoreID,
		Context:            openaiclient.WithIdentity(WithTenantID(context.Background(), ctxIdentity.TenantID), ctxIdentity),
	}
}

func buildSummary(task *Task, canonical *canonical.Product, image *productimage.ImageProcessResult) *GenerationSummary {
	summary := &GenerationSummary{}
	if task != nil && task.Request != nil {
		summary.SourceType = detectSourceType(task.Request)
		summary.ImageCount = len(task.Request.ImageURLs)
	}
	if canonical != nil {
		summary.VariantCount = len(canonical.Variants)
		summary.NeedsReview = canonical.NeedsReview
	}
	if image != nil && image.Review != nil && image.Review.NeedsReview {
		summary.NeedsReview = true
		summary.Warnings = append(summary.Warnings, image.Review.Reasons...)
	}
	return summary
}

func (a *assembler) buildSheinAssemblerConfig() sheinpub.AssemblerConfig {
	return sheinpub.AssemblerConfig{
		CategoryResolver:      a.sheinCategoryResolver,
		AttributeResolver:     a.sheinAttributeResolver,
		SaleAttributeResolver: a.sheinSaleAttributeResolver,
		PricingPolicy:         a.sheinPricingPolicy,
		TitleOptimizer:        a.sheinTitleOptimizer,
	}
}
