package httpapi

import "fmt"

func (b CoreRepositoryBuilders) Validate() error {
	switch {
	case b.Task == nil:
		return fmt.Errorf("core repository builder task is required")
	case b.StudioAsyncJob == nil:
		return fmt.Errorf("core repository builder studio async job is required")
	case b.StudioBatch == nil:
		return fmt.Errorf("core repository builder studio batch is required")
	case b.StudioBatchRun == nil:
		return fmt.Errorf("core repository builder studio batch run is required")
	case b.StudioBatchTaskLink == nil:
		return fmt.Errorf("core repository builder studio batch task link is required")
	case b.SheinSync == nil:
		return fmt.Errorf("core repository builder shein sync is required")
	case b.Subscription == nil:
		return fmt.Errorf("core repository builder subscription is required")
	case b.Asset == nil:
		return fmt.Errorf("core repository builder asset is required")
	case b.Review == nil:
		return fmt.Errorf("core repository builder review is required")
	case b.StudioSession == nil:
		return fmt.Errorf("core repository builder studio session is required")
	case b.UploadedImage == nil:
		return fmt.Errorf("core repository builder uploaded image is required")
	case b.StoreProfile == nil:
		return fmt.Errorf("core repository builder store profile is required")
	case b.SheinResolutionCache == nil:
		return fmt.Errorf("core repository builder shein resolution cache is required")
	default:
		return nil
	}
}

func (b AdminRepositoryBuilders) Validate() error {
	switch {
	case b.Store == nil:
		return fmt.Errorf("admin repository builder store is required")
	case b.StoreStatistics == nil:
		return fmt.Errorf("admin repository builder store statistics is required")
	case b.DispatchEvent == nil:
		return fmt.Errorf("admin repository builder dispatch event is required")
	case b.ImportTask == nil:
		return fmt.Errorf("admin repository builder import task is required")
	case b.FilterRule == nil:
		return fmt.Errorf("admin repository builder filter rule is required")
	case b.ProfitRule == nil:
		return fmt.Errorf("admin repository builder profit rule is required")
	case b.PricingRule == nil:
		return fmt.Errorf("admin repository builder pricing rule is required")
	case b.OperationStrategy == nil:
		return fmt.Errorf("admin repository builder operation strategy is required")
	case b.SensitiveWord == nil:
		return fmt.Errorf("admin repository builder sensitive word is required")
	case b.GenerationTopicOverride == nil:
		return fmt.Errorf("admin repository builder generation topic override is required")
	case b.GenerationTopicPolicy == nil:
		return fmt.Errorf("admin repository builder generation topic policy is required")
	case b.ProductImportMapping == nil:
		return fmt.Errorf("admin repository builder product import mapping is required")
	case b.Category == nil:
		return fmt.Errorf("admin repository builder category is required")
	case b.ProductData == nil:
		return fmt.Errorf("admin repository builder product data is required")
	default:
		return nil
	}
}

func (h BuildServiceHooks) Validate() error {
	switch {
	case h.SheinPricingPolicyBuilder == nil:
		return fmt.Errorf("build service hook shein pricing policy is required")
	case h.ImageUploadStoreBuilder == nil:
		return fmt.Errorf("build service hook image upload store is required")
	case h.LegacyTenantResolverConfigurator == nil:
		return fmt.Errorf("build service hook legacy tenant resolver is required")
	case h.SheinCategoryLLMClientBuilder == nil:
		return fmt.Errorf("build service hook shein category llm client is required")
	case h.SheinSaleAttributeLLMBuilder == nil:
		return fmt.Errorf("build service hook shein sale attribute llm client is required")
	case h.SheinCategoryResolverBuilder == nil:
		return fmt.Errorf("build service hook shein category resolver is required")
	case h.SheinAttributeResolverBuilder == nil:
		return fmt.Errorf("build service hook shein attribute resolver is required")
	case h.SheinSaleAttributeResolverBuilder == nil:
		return fmt.Errorf("build service hook shein sale attribute resolver is required")
	case h.SheinProductAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein product api builder is required")
	case h.SheinImageAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein image api builder is required")
	case h.SheinTranslateAPIBuilderFactory == nil:
		return fmt.Errorf("build service hook shein translate api builder is required")
	case h.SheinAPIClientFactoryBuilder == nil:
		return fmt.Errorf("build service hook shein api client factory is required")
	case h.StudioImageGeneratorBuilder == nil:
		return fmt.Errorf("build service hook studio image generator is required")
	case h.DefaultSheinStoreIDResolver == nil:
		return fmt.Errorf("build service hook default shein store id resolver is required")
	case h.ConfigureZitadelAuth == nil:
		return fmt.Errorf("build service hook configure zitadel auth is required")
	case h.ConfigureAuthorization == nil:
		return fmt.Errorf("build service hook configure authorization is required")
	default:
		return nil
	}
}

func (in BuildServiceInput) Validate() error {
	if in.Config == nil {
		return fmt.Errorf("build service config is required")
	}
	if err := in.Repositories.Core.Validate(); err != nil {
		return err
	}
	if err := in.Repositories.Admin.Validate(); err != nil {
		return err
	}
	return in.Hooks.Validate()
}
