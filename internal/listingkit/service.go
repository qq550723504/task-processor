package listingkit

import (
	"fmt"
	"strings"

	"task-processor/internal/amazonlisting"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

type service struct {
	repo                       Repository
	productSvc                 ProductService
	imageSvc                   ImageService
	sdsSyncSvc                 SDSSyncService
	uploadStore                ImageUploadStore
	assembler                  Assembler
	sheinCategoryResolver      sheinpub.CategoryResolver
	sheinAttributeResolver     sheinpub.AttributeResolver
	sheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	sheinPricingPolicy         sheinpub.PricingPolicy
	sheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	sheinContentOptimizer      openaiclient.ChatCompleter
	studioImageGenerator       openaiclient.ImageGenerator
	assetRepo                  AssetRepository
	reviewRepo                 GenerationReviewRepository
	assetRecipeResolver        AssetRecipeResolver
	assetBundleBuilder         AssetBundleBuilder
	assetGenerator             AssetGenerationService
	taskSubmitter              TaskSubmitter
	requestDefaults            generateRequestDefaults
}

type ServiceConfig struct {
	Repository                 Repository
	ProductService             ProductService
	ImageService               ImageService
	SDSSyncService             SDSSyncService
	ImageUploadStore           ImageUploadStore
	Assembler                  Assembler
	AssetRepository            AssetRepository
	ReviewRepository           GenerationReviewRepository
	AssetRecipeResolver        AssetRecipeResolver
	AssetBundleBuilder         AssetBundleBuilder
	AssetGenerationService     AssetGenerationService
	TaskSubmitter              TaskSubmitter
	SheinDefaultStoreID        int64
	SheinCategoryResolver      sheinpub.CategoryResolver
	SheinAttributeResolver     sheinpub.AttributeResolver
	SheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	SheinPricingPolicy         sheinpub.PricingPolicy
	SheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	SheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	SheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	SheinContentOptimizer      openaiclient.ChatCompleter
	StudioImageGenerator       openaiclient.ImageGenerator
}

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Repository == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if config.ProductService == nil {
		return nil, fmt.Errorf("product service cannot be nil")
	}
	if config.Assembler == nil {
		if config.SheinCategoryResolver == nil {
			config.SheinCategoryResolver = sheinpub.NewCategoryResolver(nil)
		}
		if config.SheinAttributeResolver == nil {
			config.SheinAttributeResolver = sheinpub.NewAttributeResolver(nil, nil)
		}
		if config.SheinSaleAttributeResolver == nil {
			config.SheinSaleAttributeResolver = sheinpub.NewSaleAttributeResolver(nil, nil)
		}
		config.Assembler = NewAssemblerWithConfig(AssemblerConfig{
			AmazonBuilder:              newAmazonDraftBuilder(),
			SheinCategoryResolver:      config.SheinCategoryResolver,
			SheinAttributeResolver:     config.SheinAttributeResolver,
			SheinSaleAttributeResolver: config.SheinSaleAttributeResolver,
			SheinPricingPolicy:         config.SheinPricingPolicy,
		})
	}
	if config.SheinCategoryResolver == nil {
		config.SheinCategoryResolver = sheinpub.NewCategoryResolver(nil)
	}
	if config.SheinAttributeResolver == nil {
		config.SheinAttributeResolver = sheinpub.NewAttributeResolver(nil, nil)
	}
	if config.SheinSaleAttributeResolver == nil {
		config.SheinSaleAttributeResolver = sheinpub.NewSaleAttributeResolver(nil, nil)
	}
	if config.AssetRepository == nil {
		config.AssetRepository = assetrepo.NewMemRepository()
	}
	if config.ReviewRepository == nil {
		config.ReviewRepository = reviewstore.NewMemRepository()
	}
	if config.AssetRecipeResolver == nil {
		config.AssetRecipeResolver = newDefaultAssetRecipeResolver()
	}
	if config.AssetBundleBuilder == nil {
		config.AssetBundleBuilder = newDefaultAssetBundleBuilder()
	}
	if config.AssetGenerationService == nil {
		config.AssetGenerationService = newDefaultAssetGenerationService()
	}
	return &service{
		repo:                       config.Repository,
		productSvc:                 config.ProductService,
		imageSvc:                   config.ImageService,
		sdsSyncSvc:                 config.SDSSyncService,
		uploadStore:                config.ImageUploadStore,
		assembler:                  config.Assembler,
		sheinCategoryResolver:      config.SheinCategoryResolver,
		sheinAttributeResolver:     config.SheinAttributeResolver,
		sheinSaleAttributeResolver: config.SheinSaleAttributeResolver,
		sheinPricingPolicy:         config.SheinPricingPolicy,
		sheinProductAPIBuilder:     config.SheinProductAPIBuilder,
		sheinImageAPIBuilder:       config.SheinImageAPIBuilder,
		sheinTranslateAPIBuilder:   config.SheinTranslateAPIBuilder,
		sheinContentOptimizer:      config.SheinContentOptimizer,
		studioImageGenerator:       config.StudioImageGenerator,
		assetRepo:                  config.AssetRepository,
		reviewRepo:                 config.ReviewRepository,
		assetRecipeResolver:        config.AssetRecipeResolver,
		assetBundleBuilder:         config.AssetBundleBuilder,
		assetGenerator:             config.AssetGenerationService,
		taskSubmitter:              config.TaskSubmitter,
		requestDefaults: generateRequestDefaults{
			sheinDefaultStoreID: config.SheinDefaultStoreID,
		},
	}, nil
}

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
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

func (b *amazonDraftBuilder) Build(req *GenerateRequest, canonical *productenrich.CanonicalProduct, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft {
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
