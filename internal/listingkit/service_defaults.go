package listingkit

import (
	"strings"
	"time"

	"task-processor/internal/amazonlisting"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/sdslogin"
)

func (config *ServiceConfig) applyDefaults() {
	config.ensureSheinResolvers()
	config.ensureAssembler()
	config.ensureAssetDependencies()
	config.ensureCoreRepositories()
	config.ensureSheinDefaults()
}

func (config *ServiceConfig) ensureSheinResolvers() {
	if config.Shein.SheinCategoryResolver == nil {
		config.Shein.SheinCategoryResolver = sheinpub.NewCategoryResolver(nil)
	}
	if config.Shein.SheinAttributeResolver == nil {
		config.Shein.SheinAttributeResolver = sheinpub.NewAttributeResolver(nil, nil)
	}
	if config.Shein.SheinSaleAttributeResolver == nil {
		config.Shein.SheinSaleAttributeResolver = sheinpub.NewSaleAttributeResolver(nil, nil)
	}
}

func (config *ServiceConfig) ensureAssembler() {
	if config.Assets.Assembler != nil {
		return
	}
	config.Assets.Assembler = NewAssemblerWithConfig(AssemblerConfig{
		AmazonBuilder:              newAmazonDraftBuilder(),
		SheinCategoryResolver:      config.Shein.SheinCategoryResolver,
		SheinAttributeResolver:     config.Shein.SheinAttributeResolver,
		SheinSaleAttributeResolver: config.Shein.SheinSaleAttributeResolver,
		SheinPricingPolicy:         config.Shein.SheinPricingPolicy,
		SheinTitleOptimizer:        config.Shein.SheinContentOptimizer,
	})
}

func (config *ServiceConfig) ensureAssetDependencies() {
	if config.Assets.AssetRepository == nil {
		config.Assets.AssetRepository = assetrepo.NewMemRepository()
	}
	if config.Assets.ReviewRepository == nil {
		config.Assets.ReviewRepository = reviewstore.NewMemRepository()
	}
	if config.Assets.AssetRecipeResolver == nil {
		config.Assets.AssetRecipeResolver = newDefaultAssetRecipeResolver()
	}
	if config.Assets.AssetBundleBuilder == nil {
		config.Assets.AssetBundleBuilder = newDefaultAssetBundleBuilder()
	}
	if config.Assets.AssetGenerationService == nil {
		config.Assets.AssetGenerationService = newDefaultAssetGenerationService()
	}
}

func (config *ServiceConfig) ensureCoreRepositories() {
	if config.Core.StoreProfileRepository == nil {
		config.Core.StoreProfileRepository = newInMemoryStoreProfileRepository()
	}
}

func (config *ServiceConfig) ensureSheinDefaults() {
	if config.Shein.StudioPromptDiversifier == nil {
		config.Shein.StudioPromptDiversifier = config.Shein.SheinContentOptimizer
	}
}

var _ SDSLoginStatusProvider = (*sdslogin.Service)(nil)

func defaultSheinSettings(storeID int64, policy sheinpub.PricingPolicy) SheinSettings {
	rule := sheinpub.PricingRule{
		SourceCurrency:   "CNY",
		TargetCurrency:   "USD",
		ExchangeRate:     7.2,
		MarkupMultiplier: 2,
		MinimumPrice:     9.99,
		RoundTo:          0.01,
	}
	if policy.Currency != "" {
		rule.TargetCurrency = strings.ToUpper(strings.TrimSpace(policy.Currency))
	}
	if policy.MarkupRate > 0 {
		rule.MarkupMultiplier = 1 + policy.MarkupRate
	}
	if policy.MinimumPrice > 0 {
		rule.MinimumPrice = policy.MinimumPrice
	}
	if policy.RoundTo > 0 {
		rule.RoundTo = policy.RoundTo
	}
	now := time.Now()
	return SheinSettings{
		DefaultStoreID:    storeID,
		Site:              "US",
		WarehouseCode:     "DEFAULT",
		DefaultStock:      100,
		DefaultSubmitMode: "publish",
		Pricing:           rule,
		UpdatedAt:         &now,
	}
}

type amazonDraftBuilder struct {
	assembler amazonlisting.Assembler
}

func newAmazonDraftBuilder() AmazonDraftBuilder {
	return &amazonDraftBuilder{assembler: amazonlisting.NewAssembler()}
}

func newDefaultAssetRecipeResolver() assetrecipe.Resolver {
	return assetrecipe.NewStaticResolver()
}

func newDefaultAssetBundleBuilder() assetbundle.Builder {
	return assetbundle.NewBuilder()
}

func newDefaultAssetGenerationService() assetgeneration.Service {
	return assetgeneration.NewService(assetgeneration.Config{})
}

func (b *amazonDraftBuilder) Build(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft {
	task := &amazonlisting.Task{
		ID: "listingkit-amazon-preview",
		Request: &amazonlisting.GenerateRequest{
			Marketplace:        "amazon",
			Country:            req.Country,
			Language:           req.Language,
			ImageURLs:          append([]string(nil), req.ImageURLs...),
			Text:               req.Text,
			ProductURL:         req.ProductURL,
			TargetCategoryHint: req.TargetCategoryHint,
			BrandHint:          req.BrandHint,
			Options: &amazonlisting.GenerateOptions{
				ProcessImages: req.Options != nil && req.Options.ProcessImages,
			},
		},
	}
	return b.assembler.Assemble(task, canonical, image)
}
