package httpapi

import "fmt"

func (b CoreRepositories) Validate() error {
	switch {
	case b.Task == nil:
		return fmt.Errorf("repository core.task is required")
	case b.SheinSync == nil:
		return fmt.Errorf("repository core.shein_sync is required")
	case b.Subscription == nil:
		return fmt.Errorf("repository core.subscription is required")
	case b.GenerationUsageLedger == nil:
		return fmt.Errorf("repository core.generation_usage_ledger is required")
	case b.MemberInvitationAudit == nil:
		return fmt.Errorf("repository core.member_invitation_audit is required")
	case b.ApprovedAsset == nil:
		return fmt.Errorf("repository core.approved_asset is required")
	case b.Review == nil:
		return fmt.Errorf("repository core.review is required")
	case b.UploadedImage == nil:
		return fmt.Errorf("repository core.uploaded_image is required")
	case b.StoreProfile == nil:
		return fmt.Errorf("repository core.store_profile is required")
	case b.SheinResolutionCache == nil:
		return fmt.Errorf("repository core.shein_resolution_cache is required")
	default:
		return nil
	}
}

func (b AdminRepositories) Validate() error {
	switch {
	case b.Store == nil:
		return fmt.Errorf("repository admin.store is required")
	case b.StoreStatistics == nil:
		return fmt.Errorf("repository admin.store_statistics is required")
	case b.DispatchEvent == nil:
		return fmt.Errorf("repository admin.dispatch_event is required")
	case b.ImportTask == nil:
		return fmt.Errorf("repository admin.import_task is required")
	case b.FilterRule == nil:
		return fmt.Errorf("repository admin.filter_rule is required")
	case b.ProfitRule == nil:
		return fmt.Errorf("repository admin.profit_rule is required")
	case b.PricingRule == nil:
		return fmt.Errorf("repository admin.pricing_rule is required")
	case b.OperationStrategy == nil:
		return fmt.Errorf("repository admin.operation_strategy is required")
	case b.ScheduledTaskConfig == nil:
		return fmt.Errorf("repository admin.scheduled_task_config is required")
	case b.SensitiveWord == nil:
		return fmt.Errorf("repository admin.sensitive_word is required")
	case b.GenerationTopicOverride == nil:
		return fmt.Errorf("repository admin.generation_topic_override is required")
	case b.GenerationTopicPolicy == nil:
		return fmt.Errorf("repository admin.generation_topic_policy is required")
	case b.ProductImportMapping == nil:
		return fmt.Errorf("repository admin.product_import_mapping is required")
	case b.Category == nil:
		return fmt.Errorf("repository admin.category is required")
	case b.ProductData == nil:
		return fmt.Errorf("repository admin.product_data is required")
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
