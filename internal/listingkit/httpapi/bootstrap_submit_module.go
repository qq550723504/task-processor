package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

type submitModuleHooks struct {
	SheinPricingPolicyBuilder         func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder           func(*config.Config, *logrus.Logger) (listingkit.ImageUploadStore, error)
	SheinCategoryLLMClientBuilder     func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinSaleAttributeLLMBuilder      func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinCategoryResolverBuilder      func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver
	SheinAttributeResolverBuilder     func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver
	SheinSaleAttributeResolverBuilder func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver
	SheinProductAPIBuilderFactory     func(listingadmin.StoreRepository) sheinpub.ProductAPIBuilder
	SheinImageAPIBuilderFactory       func(listingadmin.StoreRepository) sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilderFactory   func(listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder
	SheinAPIClientFactoryBuilder      func(listingadmin.StoreRepository) listingkit.SheinAPIClientFactory
}

type submitModuleInput struct {
	Config               *config.Config
	Logger               *logrus.Logger
	AICredentialStore    aiCredentialStore
	Hooks                submitModuleHooks
	StoreRepository      listingadmin.StoreRepository
	ResolutionCacheStore sheinpub.ResolutionCacheStore
}

type submitAssetDependencies struct {
	assembler        listingkit.Assembler
	imageUploadStore listingkit.ImageUploadStore
}

type submitSheinDependencies struct {
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	sizeHeaderResolver    sheinpub.SizeAttributeHeaderResolver
	pricingPolicy         sheinpub.PricingPolicy
	productAPIBuilder     sheinpub.ProductAPIBuilder
	imageAPIBuilder       sheinpub.ImageAPIBuilder
	translateAPIBuilder   sheinpub.TranslateAPIBuilder
	apiClientFactory      listingkit.SheinAPIClientFactory
	contentOptimizer      openaiclient.ChatCompleter
}

type submitModule struct {
	assets submitAssetDependencies
	shein  submitSheinDependencies
}

func newSubmitModuleHooks(hooks BuildServiceHooks) submitModuleHooks {
	return submitModuleHooks{
		SheinPricingPolicyBuilder:         hooks.SheinPricingPolicyBuilder,
		ImageUploadStoreBuilder:           hooks.ImageUploadStoreBuilder,
		SheinCategoryLLMClientBuilder:     hooks.SheinCategoryLLMClientBuilder,
		SheinSaleAttributeLLMBuilder:      hooks.SheinSaleAttributeLLMBuilder,
		SheinCategoryResolverBuilder:      hooks.SheinCategoryResolverBuilder,
		SheinAttributeResolverBuilder:     hooks.SheinAttributeResolverBuilder,
		SheinSaleAttributeResolverBuilder: hooks.SheinSaleAttributeResolverBuilder,
		SheinProductAPIBuilderFactory:     hooks.SheinProductAPIBuilderFactory,
		SheinImageAPIBuilderFactory:       hooks.SheinImageAPIBuilderFactory,
		SheinTranslateAPIBuilderFactory:   hooks.SheinTranslateAPIBuilderFactory,
		SheinAPIClientFactoryBuilder:      hooks.SheinAPIClientFactoryBuilder,
	}
}

func newSubmitModuleInput(input BuildServiceInput, repos *builtRepositories) submitModuleInput {
	return submitModuleInput{
		Config:               input.Config,
		Logger:               input.Logger,
		AICredentialStore:    input.AICredentialStore,
		Hooks:                newSubmitModuleHooks(input.Hooks),
		StoreRepository:      repos.storeRepository,
		ResolutionCacheStore: repos.resolutionCacheStore,
	}
}

func buildSubmitModule(in submitModuleInput) (submitModule, error) {
	var sheinCategoryLLMClient openaiclient.ChatCompleter
	if in.Hooks.SheinCategoryLLMClientBuilder != nil {
		sheinCategoryLLMClient = in.Hooks.SheinCategoryLLMClientBuilder(in.Config, in.AICredentialStore)
	}

	var sheinSaleAttributeLLMClient openaiclient.ChatCompleter
	if in.Hooks.SheinSaleAttributeLLMBuilder != nil {
		sheinSaleAttributeLLMClient = in.Hooks.SheinSaleAttributeLLMBuilder(in.Config, in.AICredentialStore)
	}

	var sheinCategoryResolver sheinpub.CategoryResolver
	if in.Hooks.SheinCategoryResolverBuilder != nil {
		sheinCategoryResolver = in.Hooks.SheinCategoryResolverBuilder(in.StoreRepository, sheinCategoryLLMClient, in.ResolutionCacheStore)
	}

	var sheinAttributeResolver sheinpub.AttributeResolver
	if in.Hooks.SheinAttributeResolverBuilder != nil {
		sheinAttributeResolver = in.Hooks.SheinAttributeResolverBuilder(in.StoreRepository, sheinSaleAttributeLLMClient, in.ResolutionCacheStore)
	}

	var sheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	if in.Hooks.SheinSaleAttributeResolverBuilder != nil {
		sheinSaleAttributeResolver = in.Hooks.SheinSaleAttributeResolverBuilder(in.StoreRepository, sheinSaleAttributeLLMClient, in.ResolutionCacheStore)
	}
	sheinSizeHeaderResolver := sheinpub.NewSizeAttributeHeaderResolver(sheinSaleAttributeLLMClient)

	var sheinProductAPIBuilder sheinpub.ProductAPIBuilder
	if in.Hooks.SheinProductAPIBuilderFactory != nil {
		sheinProductAPIBuilder = in.Hooks.SheinProductAPIBuilderFactory(in.StoreRepository)
	}

	var sheinImageAPIBuilder sheinpub.ImageAPIBuilder
	if in.Hooks.SheinImageAPIBuilderFactory != nil {
		sheinImageAPIBuilder = in.Hooks.SheinImageAPIBuilderFactory(in.StoreRepository)
	}

	var sheinTranslateAPIBuilder sheinpub.TranslateAPIBuilder
	if in.Hooks.SheinTranslateAPIBuilderFactory != nil {
		sheinTranslateAPIBuilder = in.Hooks.SheinTranslateAPIBuilderFactory(in.StoreRepository)
	}

	var sheinAPIClientFactory listingkit.SheinAPIClientFactory
	if in.Hooks.SheinAPIClientFactoryBuilder != nil {
		sheinAPIClientFactory = in.Hooks.SheinAPIClientFactoryBuilder(in.StoreRepository)
	}

	var sheinPricingPolicy sheinpub.PricingPolicy
	if in.Hooks.SheinPricingPolicyBuilder != nil {
		sheinPricingPolicy = in.Hooks.SheinPricingPolicyBuilder(in.Config)
	}

	var imageUploadStore listingkit.ImageUploadStore
	if in.Hooks.ImageUploadStoreBuilder != nil {
		var err error
		imageUploadStore, err = in.Hooks.ImageUploadStoreBuilder(in.Config, in.Logger)
		if err != nil {
			return submitModule{}, fmt.Errorf("build ListingKit image upload store: %w", err)
		}
	}

	sheinDependencies := submitSheinDependencies{
		categoryResolver:      sheinCategoryResolver,
		attributeResolver:     sheinAttributeResolver,
		saleAttributeResolver: sheinSaleAttributeResolver,
		sizeHeaderResolver:    sheinSizeHeaderResolver,
		pricingPolicy:         sheinPricingPolicy,
		productAPIBuilder:     sheinProductAPIBuilder,
		imageAPIBuilder:       sheinImageAPIBuilder,
		translateAPIBuilder:   sheinTranslateAPIBuilder,
		apiClientFactory:      sheinAPIClientFactory,
		contentOptimizer:      sheinCategoryLLMClient,
	}
	if err := sheinDependencies.validate(); err != nil {
		return submitModule{}, err
	}

	module := submitModule{
		assets: submitAssetDependencies{
			assembler: listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{
				SheinCategoryResolver:      sheinCategoryResolver,
				SheinAttributeResolver:     sheinAttributeResolver,
				SheinSaleAttributeResolver: sheinSaleAttributeResolver,
				SheinSizeHeaderResolver:    sheinSizeHeaderResolver,
				SheinPricingPolicy:         sheinPricingPolicy,
				SheinTitleOptimizer:        sheinCategoryLLMClient,
			}),
			imageUploadStore: imageUploadStore,
		},
		shein: sheinDependencies,
	}
	return module, nil
}

func (d submitSheinDependencies) validate() error {
	switch {
	case d.categoryResolver == nil:
		return fmt.Errorf("SHEIN category resolver builder returned nil")
	case d.attributeResolver == nil:
		return fmt.Errorf("SHEIN attribute resolver builder returned nil")
	case d.saleAttributeResolver == nil:
		return fmt.Errorf("SHEIN sale-attribute resolver builder returned nil")
	case d.productAPIBuilder == nil:
		return fmt.Errorf("SHEIN product API builder factory returned nil")
	case d.imageAPIBuilder == nil:
		return fmt.Errorf("SHEIN image API builder factory returned nil")
	case d.translateAPIBuilder == nil:
		return fmt.Errorf("SHEIN translate API builder factory returned nil")
	case d.apiClientFactory == nil:
		return fmt.Errorf("SHEIN API client factory builder returned nil")
	default:
		return nil
	}
}
