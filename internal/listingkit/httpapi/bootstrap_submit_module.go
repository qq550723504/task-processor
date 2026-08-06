package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
)

type submitModuleHooks struct {
	SheinPricingPolicyBuilder         func(*config.Config) sheinpub.PricingPolicy
	ImageUploadStoreBuilder           func(*config.Config, *logrus.Logger) listingkit.ImageUploadStore
	SheinCategoryLLMClientBuilder     func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinSaleAttributeLLMBuilder      func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ChatCompleter
	SheinCategoryResolverBuilder      func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver
	SheinAttributeResolverBuilder     func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver
	SheinSaleAttributeResolverBuilder func(listingadmin.StoreRepository, openaiclient.ChatCompleter, sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver
	SheinProductAPIBuilderFactory     func(listingadmin.StoreRepository) sheinpub.ProductAPIBuilder
	SheinImageAPIBuilderFactory       func(listingadmin.StoreRepository) sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilderFactory   func(listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder
	SheinAPIClientFactoryBuilder      func(listingadmin.StoreRepository) listingkit.SheinAPIClientFactory
	StudioImageGeneratorBuilder       func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator
	StudioAICapabilityRouterBuilder   func(openaiclient.ClientConfigResolver) aicapability.Router
}

type submitModuleInput struct {
	Config               *config.Config
	Logger               *logrus.Logger
	AICredentialStore    aiCredentialStore
	AIInvocationRecorder aicapability.InvocationRecorder
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
	defaultStoreID        int64
}

type submitStudioDependencies struct {
	imageGenerator listingkit.AIImageGenerator
}

type submitModule struct {
	assets submitAssetDependencies
	shein  submitSheinDependencies
	studio submitStudioDependencies
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
		StudioImageGeneratorBuilder:       hooks.StudioImageGeneratorBuilder,
		StudioAICapabilityRouterBuilder:   hooks.StudioAICapabilityRouterBuilder,
	}
}

func newSubmitModuleInput(input BuildServiceInput, repos *builtRepositories) submitModuleInput {
	return submitModuleInput{
		Config:               input.Config,
		Logger:               input.Logger,
		AICredentialStore:    input.AICredentialStore,
		AIInvocationRecorder: input.AIInvocationRecorder,
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
		imageUploadStore = in.Hooks.ImageUploadStoreBuilder(in.Config, in.Logger)
	}

	var studioImageGenerator openaiclient.ImageGenerator
	if in.Hooks.StudioImageGeneratorBuilder != nil {
		studioImageGenerator = in.Hooks.StudioImageGeneratorBuilder(in.Config, in.AICredentialStore)
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
		shein: submitSheinDependencies{
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
		},
		studio: submitStudioDependencies{
			imageGenerator: adaptListingKitAIImageGenerator(studioImageGenerator),
		},
	}
	mode := aicapability.RoutingModeLegacy
	if in.Config != nil {
		var err error
		mode, err = aicapability.ParseRoutingMode(in.Config.AICapability.StudioImageRoutingMode)
		if err != nil {
			return submitModule{}, fmt.Errorf("parse Studio AI capability routing mode: %w", err)
		}
	}
	if mode == aicapability.RoutingModeLegacy {
		return module, nil
	}
	if in.AICredentialStore == nil || in.Hooks.StudioAICapabilityRouterBuilder == nil || in.AIInvocationRecorder == nil || module.studio.imageGenerator == nil {
		return submitModule{}, fmt.Errorf("Studio AI capability routing requires credential resolver, router builder, invocation recorder, and legacy image generator")
	}
	router := in.Hooks.StudioAICapabilityRouterBuilder(in.AICredentialStore)
	if router == nil {
		return submitModule{}, fmt.Errorf("Studio AI capability routing requires router builder to return a router")
	}
	adapter, err := listingkit.NewStudioAIImageCapabilityAdapter(listingkit.StudioAIImageCapabilityAdapterConfig{
		Legacy:   module.studio.imageGenerator,
		Router:   router,
		Recorder: in.AIInvocationRecorder,
		Mode:     mode,
		OnRecordError: func(record aicapability.InvocationRecord, err error) {
			if in.Logger != nil {
				in.Logger.WithError(err).WithFields(logrus.Fields{
					"invocation_id": record.InvocationID,
					"tenant_id":     record.TenantID,
					"capability":    record.Capability,
					"operation":     record.Operation,
				}).Warn("record Studio AI capability invocation")
			}
		},
	})
	if err != nil {
		return submitModule{}, fmt.Errorf("create Studio AI capability adapter: %w", err)
	}
	module.studio.imageGenerator = adapter
	return module, nil
}
