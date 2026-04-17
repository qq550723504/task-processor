package listingkit

import (
	"time"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type assembler struct {
	amazonBuilder              AmazonDraftBuilder
	sheinCategoryResolver      SheinCategoryResolver
	sheinAttributeResolver     SheinAttributeResolver
	sheinSaleAttributeResolver SheinSaleAttributeResolver
}

func NewAssembler(amazonBuilder AmazonDraftBuilder) Assembler {
	return NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: amazonBuilder})
}

type AssemblerConfig struct {
	AmazonBuilder              AmazonDraftBuilder
	SheinCategoryResolver      SheinCategoryResolver
	SheinAttributeResolver     SheinAttributeResolver
	SheinSaleAttributeResolver SheinSaleAttributeResolver
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
	}
}

func (a *assembler) Assemble(task *Task, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *ListingKitResult {
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
			result.Shein = buildSheinPackage(task.Request, canonical, image, a.sheinCategoryResolver, a.sheinAttributeResolver, a.sheinSaleAttributeResolver)
		case "temu":
			result.Temu = buildTemuPackage(task.Request, canonical, image)
		case "walmart":
			result.Walmart = buildWalmartPackage(task.Request, canonical, image)
		}
	}

	return result
}

func buildSummary(task *Task, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *GenerationSummary {
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
