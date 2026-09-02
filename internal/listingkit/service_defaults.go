package listingkit

import (
	"strings"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/listingkit/reviewstore"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/sdslogin"
)

func (config *ServiceConfig) applyDefaults() {
	runServiceConfigInitializers(
		config,
		(*ServiceConfig).ensureSheinSizeHeaderResolver,
		(*ServiceConfig).ensureAssembler,
		(*ServiceConfig).ensureAssetDependencies,
		(*ServiceConfig).ensureCoreRepositories,
	)
}

func (config *ServiceConfig) ensureSheinSizeHeaderResolver() {
	if config.Shein.SheinSizeHeaderResolver == nil {
		config.Shein.SheinSizeHeaderResolver = sheinpub.NewSizeAttributeHeaderResolver(config.Shein.SheinContentOptimizer)
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
		SheinSizeHeaderResolver:    config.Shein.SheinSizeHeaderResolver,
		SheinPricingPolicy:         config.Shein.SheinPricingPolicy,
		SheinTitleOptimizer:        config.Shein.SheinContentOptimizer,
	})
}

func (config *ServiceConfig) ensureAssetDependencies() {
	if config.Assets.ReviewRepository == nil {
		config.Assets.ReviewRepository = reviewstore.NewMemRepository()
	}
}

func (config *ServiceConfig) ensureCoreRepositories() {
	if config.Core.StoreProfileRepository == nil {
		config.Core.StoreProfileRepository = newInMemoryStoreProfileRepository()
	}
}

var _ SDSLoginStatusProvider = (*sdslogin.Service)(nil)

func defaultSheinSettings(policy sheinpub.PricingPolicy) SheinSettings {
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

func (b *amazonDraftBuilder) Build(req *GenerateRequest, snapshot *catalog.ProductSnapshot, approved *productasset.ApprovedAssetInventory) (*amazonlisting.AmazonListingDraft, error) {
	if req == nil || snapshot == nil || approved == nil {
		return nil, productasset.ErrApprovedAssetsNotReady
	}
	draft, err := b.assembler.Build(amazonlisting.DraftInput{
		TaskID: "listingkit-amazon-preview",
		Request: &amazonlisting.GenerateRequest{
			Marketplace:        "amazon",
			Country:            req.Country,
			Language:           req.Language,
			ProductKey:         req.ProductKey,
			TargetCategoryHint: req.TargetCategoryHint,
			BrandHint:          req.BrandHint,
		},
		Snapshot:       *snapshot,
		ApprovedAssets: *approved,
	})
	if err != nil {
		return nil, err
	}
	return draft, nil
}
