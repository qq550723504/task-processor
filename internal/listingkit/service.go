package listingkit

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/amazonlisting"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/infra/clients/management"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type service struct {
	repo                           Repository
	studioSessionRepo              StudioSessionRepository
	productSvc                     ProductService
	imageSvc                       ImageService
	sdsSyncSvc                     SDSSyncService
	uploadStore                    ImageUploadStore
	uploadedImageRepo              UploadedImageRepository
	assembler                      Assembler
	sheinCategoryResolver          sheinpub.CategoryResolver
	sheinResolutionCacheStore      sheinpub.ResolutionCacheStore
	sheinManagementClient          *management.ClientManager
	sheinAttributeResolver         sheinpub.AttributeResolver
	sheinSaleAttributeResolver     sheinpub.SaleAttributeResolver
	sheinPricingPolicy             sheinpub.PricingPolicy
	sheinProductAPIBuilder         sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder           sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder       sheinpub.TranslateAPIBuilder
	sheinContentOptimizer          openaiclient.ChatCompleter
	studioPromptDiversifier        openaiclient.ChatCompleter
	studioImageGenerator           openaiclient.ImageGenerator
	aiCredentialStore              AIClientCredentialStore
	assetRepo                      AssetRepository
	reviewRepo                     GenerationReviewRepository
	assetRecipeResolver            AssetRecipeResolver
	assetBundleBuilder             AssetBundleBuilder
	assetGenerator                 AssetGenerationService
	taskSubmitter                  TaskSubmitter
	sheinPublishWorkflowClient     SheinPublishWorkflowClient
	sheinPublishWorkflowEnabled    bool
	standardProductWorkflowClient  StandardProductWorkflowClient
	standardProductWorkflowEnabled bool
	platformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	platformAdaptWorkflowEnabled   bool
	storeProfileRepo               StoreProfileRepository
	routingSettingsRepo            StoreRoutingSettingsRepository
	requestDefaults                generateRequestDefaults
	sheinSubmitLocks               *submitLockManager
	sheinSettingsMu                sync.RWMutex
	sheinSettings                  SheinSettings
}

type ServiceCoreDependencies struct {
	Repository                     Repository
	StudioSessionRepository        StudioSessionRepository
	ProductService                 ProductService
	ImageService                   ImageService
	SDSSyncService                 SDSSyncService
	ImageUploadStore               ImageUploadStore
	UploadedImageRepository        UploadedImageRepository
	StoreProfileRepository         StoreProfileRepository
	StoreRoutingSettingsRepository StoreRoutingSettingsRepository
	TaskSubmitter                  TaskSubmitter
	AIClientCredentialStore        AIClientCredentialStore
}

type ServiceAssetDependencies struct {
	Assembler              Assembler
	AssetRepository        AssetRepository
	ReviewRepository       GenerationReviewRepository
	AssetRecipeResolver    AssetRecipeResolver
	AssetBundleBuilder     AssetBundleBuilder
	AssetGenerationService AssetGenerationService
}

type ServiceSheinDependencies struct {
	SheinDefaultStoreID        int64
	SheinManagementClient      *management.ClientManager
	SheinCategoryResolver      sheinpub.CategoryResolver
	SheinResolutionCacheStore  sheinpub.ResolutionCacheStore
	SheinAttributeResolver     sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	SheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	SheinContentOptimizer      openaiclient.ChatCompleter
	StudioPromptDiversifier    openaiclient.ChatCompleter
	StudioImageGenerator       openaiclient.ImageGenerator
}

type ServiceWorkflowDependencies struct {
	SheinPublishWorkflowClient     SheinPublishWorkflowClient
	SheinPublishWorkflowEnabled    bool
	StandardProductWorkflowClient  StandardProductWorkflowClient
	StandardProductWorkflowEnabled bool
	PlatformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	PlatformAdaptWorkflowEnabled   bool
}

type ServiceConfig struct {
	Core     ServiceCoreDependencies
	Assets   ServiceAssetDependencies
	Shein    ServiceSheinDependencies
	Workflow ServiceWorkflowDependencies

	// Deprecated: use Core.Repository.
	Repository                     Repository
	// Deprecated: use Core.StudioSessionRepository.
	StudioSessionRepository        StudioSessionRepository
	// Deprecated: use Core.ProductService.
	ProductService                 ProductService
	// Deprecated: use Core.ImageService.
	ImageService                   ImageService
	// Deprecated: use Core.SDSSyncService.
	SDSSyncService                 SDSSyncService
	// Deprecated: use Core.ImageUploadStore.
	ImageUploadStore               ImageUploadStore
	// Deprecated: use Core.UploadedImageRepository.
	UploadedImageRepository        UploadedImageRepository
	// Deprecated: use Assets.Assembler.
	Assembler                      Assembler
	// Deprecated: use Assets.AssetRepository.
	AssetRepository                AssetRepository
	// Deprecated: use Assets.ReviewRepository.
	ReviewRepository               GenerationReviewRepository
	// Deprecated: use Assets.AssetRecipeResolver.
	AssetRecipeResolver            AssetRecipeResolver
	// Deprecated: use Assets.AssetBundleBuilder.
	AssetBundleBuilder             AssetBundleBuilder
	// Deprecated: use Assets.AssetGenerationService.
	AssetGenerationService         AssetGenerationService
	// Deprecated: use Core.TaskSubmitter.
	TaskSubmitter                  TaskSubmitter
	// Deprecated: use Workflow.SheinPublishWorkflowClient.
	SheinPublishWorkflowClient     SheinPublishWorkflowClient
	// Deprecated: use Workflow.SheinPublishWorkflowEnabled.
	SheinPublishWorkflowEnabled    bool
	// Deprecated: use Workflow.StandardProductWorkflowClient.
	StandardProductWorkflowClient  StandardProductWorkflowClient
	// Deprecated: use Workflow.StandardProductWorkflowEnabled.
	StandardProductWorkflowEnabled bool
	// Deprecated: use Workflow.PlatformAdaptWorkflowClient.
	PlatformAdaptWorkflowClient    PlatformAdaptWorkflowClient
	// Deprecated: use Workflow.PlatformAdaptWorkflowEnabled.
	PlatformAdaptWorkflowEnabled   bool
	// Deprecated: use Core.StoreProfileRepository.
	StoreProfileRepository         StoreProfileRepository
	// Deprecated: use Core.StoreRoutingSettingsRepository.
	StoreRoutingSettingsRepository StoreRoutingSettingsRepository
	// Deprecated: use Shein.SheinDefaultStoreID.
	SheinDefaultStoreID            int64
	// Deprecated: use Shein.SheinManagementClient.
	SheinManagementClient          *management.ClientManager
	// Deprecated: use Shein.SheinCategoryResolver.
	SheinCategoryResolver          sheinpub.CategoryResolver
	// Deprecated: use Shein.SheinResolutionCacheStore.
	SheinResolutionCacheStore      sheinpub.ResolutionCacheStore
	// Deprecated: use Shein.SheinAttributeResolver.
	SheinAttributeResolver         sheinpub.AttributeResolver
	// Deprecated: use Shein.SheinSaleAttributeResolver.
	SheinSaleAttributeResolver     sheinpub.SaleAttributeResolver
	// Deprecated: use Shein.SheinPricingPolicy.
	SheinPricingPolicy             sheinpub.PricingPolicy
	// Deprecated: use Shein.SheinProductAPIBuilder.
	SheinProductAPIBuilder         sheinpub.ProductAPIBuilder
	// Deprecated: use Shein.SheinImageAPIBuilder.
	SheinImageAPIBuilder           sheinpub.ImageAPIBuilder
	// Deprecated: use Shein.SheinTranslateAPIBuilder.
	SheinTranslateAPIBuilder       sheinpub.TranslateAPIBuilder
	// Deprecated: use Shein.SheinContentOptimizer.
	SheinContentOptimizer          openaiclient.ChatCompleter
	// Deprecated: use Shein.StudioPromptDiversifier.
	StudioPromptDiversifier        openaiclient.ChatCompleter
	// Deprecated: use Shein.StudioImageGenerator.
	StudioImageGenerator           openaiclient.ImageGenerator
	// Deprecated: use Core.AIClientCredentialStore.
	AIClientCredentialStore        AIClientCredentialStore
}

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	config.normalizeLegacyFields()
	if config.Core.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if config.Core.ProductService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}
	config.applyDefaults()
	defaultSettings := defaultSheinSettings(config.Shein.SheinDefaultStoreID, config.Shein.SheinPricingPolicy)
	return &service{
		repo:                           config.Core.Repository,
		studioSessionRepo:              config.Core.StudioSessionRepository,
		productSvc:                     config.Core.ProductService,
		imageSvc:                       config.Core.ImageService,
		sdsSyncSvc:                     config.Core.SDSSyncService,
		uploadStore:                    config.Core.ImageUploadStore,
		uploadedImageRepo:              config.Core.UploadedImageRepository,
		assembler:                      config.Assets.Assembler,
		sheinCategoryResolver:          config.Shein.SheinCategoryResolver,
		sheinResolutionCacheStore:      config.Shein.SheinResolutionCacheStore,
		sheinManagementClient:          config.Shein.SheinManagementClient,
		sheinAttributeResolver:         config.Shein.SheinAttributeResolver,
		sheinSaleAttributeResolver:     config.Shein.SheinSaleAttributeResolver,
		sheinPricingPolicy:             config.Shein.SheinPricingPolicy,
		sheinProductAPIBuilder:         config.Shein.SheinProductAPIBuilder,
		sheinImageAPIBuilder:           config.Shein.SheinImageAPIBuilder,
		sheinTranslateAPIBuilder:       config.Shein.SheinTranslateAPIBuilder,
		sheinContentOptimizer:          config.Shein.SheinContentOptimizer,
		studioPromptDiversifier:        config.Shein.StudioPromptDiversifier,
		studioImageGenerator:           config.Shein.StudioImageGenerator,
		aiCredentialStore:              config.Core.AIClientCredentialStore,
		assetRepo:                      config.Assets.AssetRepository,
		reviewRepo:                     config.Assets.ReviewRepository,
		assetRecipeResolver:            config.Assets.AssetRecipeResolver,
		assetBundleBuilder:             config.Assets.AssetBundleBuilder,
		assetGenerator:                 config.Assets.AssetGenerationService,
		taskSubmitter:                  config.Core.TaskSubmitter,
		sheinPublishWorkflowClient:     config.Workflow.SheinPublishWorkflowClient,
		sheinPublishWorkflowEnabled:    config.Workflow.SheinPublishWorkflowEnabled,
		standardProductWorkflowClient:  config.Workflow.StandardProductWorkflowClient,
		standardProductWorkflowEnabled: config.Workflow.StandardProductWorkflowEnabled,
		platformAdaptWorkflowClient:    config.Workflow.PlatformAdaptWorkflowClient,
		platformAdaptWorkflowEnabled:   config.Workflow.PlatformAdaptWorkflowEnabled,
		storeProfileRepo:               config.Core.StoreProfileRepository,
		routingSettingsRepo:            config.Core.StoreRoutingSettingsRepository,
		requestDefaults: generateRequestDefaults{
			sheinDefaultStoreID: config.Shein.SheinDefaultStoreID,
		},
		sheinSubmitLocks: newSubmitLockManager(),
		sheinSettings:    defaultSettings,
	}, nil
}

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
	if config.Core.StoreRoutingSettingsRepository == nil {
		config.Core.StoreRoutingSettingsRepository = newInMemoryStoreRoutingSettingsRepository()
	}
}

func (config *ServiceConfig) ensureSheinDefaults() {
	if config.Shein.StudioPromptDiversifier == nil {
		config.Shein.StudioPromptDiversifier = config.Shein.SheinContentOptimizer
	}
}

func (config *ServiceConfig) normalizeLegacyFields() {
	if config == nil {
		return
	}
	config.normalizeLegacyCoreFields()
	config.normalizeLegacyAssetFields()
	config.normalizeLegacySheinFields()
	config.normalizeLegacyWorkflowFields()
}

func (config *ServiceConfig) normalizeLegacyCoreFields() {
	if config.Core.Repository == nil {
		config.Core.Repository = config.Repository
	}
	if config.Core.StudioSessionRepository == nil {
		config.Core.StudioSessionRepository = config.StudioSessionRepository
	}
	if config.Core.ProductService == nil {
		config.Core.ProductService = config.ProductService
	}
	if config.Core.ImageService == nil {
		config.Core.ImageService = config.ImageService
	}
	if config.Core.SDSSyncService == nil {
		config.Core.SDSSyncService = config.SDSSyncService
	}
	if config.Core.ImageUploadStore == nil {
		config.Core.ImageUploadStore = config.ImageUploadStore
	}
	if config.Core.UploadedImageRepository == nil {
		config.Core.UploadedImageRepository = config.UploadedImageRepository
	}
	if config.Core.StoreProfileRepository == nil {
		config.Core.StoreProfileRepository = config.StoreProfileRepository
	}
	if config.Core.StoreRoutingSettingsRepository == nil {
		config.Core.StoreRoutingSettingsRepository = config.StoreRoutingSettingsRepository
	}
	if config.Core.TaskSubmitter == nil {
		config.Core.TaskSubmitter = config.TaskSubmitter
	}
	if config.Core.AIClientCredentialStore == nil {
		config.Core.AIClientCredentialStore = config.AIClientCredentialStore
	}
}

func (config *ServiceConfig) normalizeLegacyAssetFields() {
	if config.Assets.Assembler == nil {
		config.Assets.Assembler = config.Assembler
	}
	if config.Assets.AssetRepository == nil {
		config.Assets.AssetRepository = config.AssetRepository
	}
	if config.Assets.ReviewRepository == nil {
		config.Assets.ReviewRepository = config.ReviewRepository
	}
	if config.Assets.AssetRecipeResolver == nil {
		config.Assets.AssetRecipeResolver = config.AssetRecipeResolver
	}
	if config.Assets.AssetBundleBuilder == nil {
		config.Assets.AssetBundleBuilder = config.AssetBundleBuilder
	}
	if config.Assets.AssetGenerationService == nil {
		config.Assets.AssetGenerationService = config.AssetGenerationService
	}
}

func (config *ServiceConfig) normalizeLegacySheinFields() {
	if config.Shein.SheinDefaultStoreID == 0 {
		config.Shein.SheinDefaultStoreID = config.SheinDefaultStoreID
	}
	if config.Shein.SheinManagementClient == nil {
		config.Shein.SheinManagementClient = config.SheinManagementClient
	}
	if config.Shein.SheinCategoryResolver == nil {
		config.Shein.SheinCategoryResolver = config.SheinCategoryResolver
	}
	if config.Shein.SheinResolutionCacheStore == nil {
		config.Shein.SheinResolutionCacheStore = config.SheinResolutionCacheStore
	}
	if config.Shein.SheinAttributeResolver == nil {
		config.Shein.SheinAttributeResolver = config.SheinAttributeResolver
	}
	if config.Shein.SheinSaleAttributeResolver == nil {
		config.Shein.SheinSaleAttributeResolver = config.SheinSaleAttributeResolver
	}
	if config.Shein.SheinPricingPolicy == (sheinpub.PricingPolicy{}) {
		config.Shein.SheinPricingPolicy = config.SheinPricingPolicy
	}
	if config.Shein.SheinProductAPIBuilder == nil {
		config.Shein.SheinProductAPIBuilder = config.SheinProductAPIBuilder
	}
	if config.Shein.SheinImageAPIBuilder == nil {
		config.Shein.SheinImageAPIBuilder = config.SheinImageAPIBuilder
	}
	if config.Shein.SheinTranslateAPIBuilder == nil {
		config.Shein.SheinTranslateAPIBuilder = config.SheinTranslateAPIBuilder
	}
	if config.Shein.SheinContentOptimizer == nil {
		config.Shein.SheinContentOptimizer = config.SheinContentOptimizer
	}
	if config.Shein.StudioPromptDiversifier == nil {
		config.Shein.StudioPromptDiversifier = config.StudioPromptDiversifier
	}
	if config.Shein.StudioImageGenerator == nil {
		config.Shein.StudioImageGenerator = config.StudioImageGenerator
	}
}

func (config *ServiceConfig) normalizeLegacyWorkflowFields() {
	if config.Workflow.SheinPublishWorkflowClient == nil {
		config.Workflow.SheinPublishWorkflowClient = config.SheinPublishWorkflowClient
	}
	if !config.Workflow.SheinPublishWorkflowEnabled {
		config.Workflow.SheinPublishWorkflowEnabled = config.SheinPublishWorkflowEnabled
	}
	if config.Workflow.StandardProductWorkflowClient == nil {
		config.Workflow.StandardProductWorkflowClient = config.StandardProductWorkflowClient
	}
	if !config.Workflow.StandardProductWorkflowEnabled {
		config.Workflow.StandardProductWorkflowEnabled = config.StandardProductWorkflowEnabled
	}
	if config.Workflow.PlatformAdaptWorkflowClient == nil {
		config.Workflow.PlatformAdaptWorkflowClient = config.PlatformAdaptWorkflowClient
	}
	if !config.Workflow.PlatformAdaptWorkflowEnabled {
		config.Workflow.PlatformAdaptWorkflowEnabled = config.PlatformAdaptWorkflowEnabled
	}
}

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

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

func ConfigureSheinPublishWorkflowClient(svc Service, client SheinPublishWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	impl, ok := svc.(*service)
	if !ok {
		return fmt.Errorf("listingkit service does not support shein publish workflow configuration")
	}
	impl.sheinPublishWorkflowClient = client
	impl.sheinPublishWorkflowEnabled = enabled && client != nil
	return nil
}

func ConfigureStandardProductWorkflowClient(svc Service, client StandardProductWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	impl, ok := svc.(*service)
	if !ok {
		return fmt.Errorf("listingkit service does not support standard product workflow configuration")
	}
	impl.standardProductWorkflowClient = client
	impl.standardProductWorkflowEnabled = enabled && client != nil
	return nil
}

func ConfigurePlatformAdaptWorkflowClient(svc Service, client PlatformAdaptWorkflowClient, enabled bool) error {
	if svc == nil {
		return fmt.Errorf("listingkit service is nil")
	}
	impl, ok := svc.(*service)
	if !ok {
		return fmt.Errorf("listingkit service does not support platform adaptation workflow configuration")
	}
	impl.platformAdaptWorkflowClient = client
	impl.platformAdaptWorkflowEnabled = enabled && client != nil
	return nil
}

func (s *service) currentSheinSubmitSettings() SheinSettings {
	s.sheinSettingsMu.RLock()
	defer s.sheinSettingsMu.RUnlock()
	return s.sheinSettings
}

func normalizeGenerateRequest(req *GenerateRequest) {
	if req == nil {
		return
	}
	req.Country = strings.ToUpper(strings.TrimSpace(req.Country))
	req.Language = strings.TrimSpace(req.Language)
	if req.Country == "" {
		req.Country = "US"
	}
	if req.Language == "" {
		req.Language = "en_US"
	}
	if req.Options == nil {
		req.Options = &GenerateOptions{ProcessImages: true}
	} else if req.Options.Scene != nil {
		req.Options.ProcessImages = true
	}
	req.Platforms = normalizePlatforms(req.Platforms)
	if len(req.Platforms) == 0 {
		req.Platforms = []string{"amazon", "shein", "temu", "walmart"}
	}
}

func normalizePlatforms(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		switch normalized {
		case "amazon", "shein", "temu", "walmart":
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}

type amazonDraftBuilder struct {
	assembler amazonlisting.Assembler
}

func newAmazonDraftBuilder() AmazonDraftBuilder {
	return &amazonDraftBuilder{assembler: amazonlisting.NewAssembler()}
}

func newDefaultAssetRecipeResolver() AssetRecipeResolver {
	return assetrecipe.NewStaticResolver()
}

func newDefaultAssetBundleBuilder() AssetBundleBuilder {
	return assetbundle.NewBuilder()
}

func newDefaultAssetGenerationService() AssetGenerationService {
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
