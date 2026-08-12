package httpapi

import (
	"fmt"

	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/worker"
	productimage "task-processor/internal/productimage"
	productimagepipeline "task-processor/internal/productimage/pipeline"
)

type Module struct {
	Handler               RouteHandler
	Pool                  worker.WorkerPool
	Service               productimage.Service
	Closers               []func() error
	SubjectExtractor      productimage.SubjectExtractor
	WhiteBackgroundRender productimage.WhiteBackgroundRenderer
	SceneRenderer         productimage.SceneRenderer
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	sourceParser, err := productimage.NewSourceParser(input.InputParser)
	if err != nil {
		return nil, fmt.Errorf("create image source parser: %w", err)
	}
	contextAnalyzer, err := productimage.NewProductContextAnalyzer(input.Understanding)
	if err != nil {
		return nil, fmt.Errorf("create image context analyzer: %w", err)
	}

	imageRepo, closers, err := buildTaskRepository(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}

	imageInspector, err := productimage.NewDownloadedImageInspector(input.ImageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create downloaded image inspector: %w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(input.ImageWorkDir, input.Config.Watermark, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("create watermark-aware image cleaner: %w", err)
	}
	modelProvider, err := buildModelProvider(input.Config, input.LLMManager, input.OpenAIManager, input.ImageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create productimage model provider: %w", err)
	}
	if input.Config != nil && input.Config.AICapability.ProductImageSceneEnabled {
		if input.OpenAIManager == nil || !input.OpenAIManager.HasConfigResolver() {
			return nil, fmt.Errorf("create governed productimage scene generator: resolver-backed OpenAI manager is not configured")
		}
		if modelProvider == nil {
			return nil, fmt.Errorf("create governed productimage scene generator: model provider is not configured")
		}
		governedScene, governanceErr := buildGovernedProductImageSceneGenerator(input.Config, modelProvider.SceneGenerator(), input.AICredentialResolver, input.AIInvocationRecorder, input.Logger)
		if governanceErr != nil {
			return nil, fmt.Errorf("create governed productimage scene generator: %w", governanceErr)
		}
		allowed := tenantIDSet(input.Config.AICapability.ProductImageSceneAllowedTenantIDs)
		var faithfulEditor productimage.FaithfulEditor
		if editor := modelProvider.FaithfulEditor(); editor != nil {
			faithfulEditor = &tenantAllowlistedFaithfulEditor{
				inner: &governedFaithfulEditor{
					inner: editor, router: BuildProductImageSceneCapabilityRouter(input.AICredentialResolver, input.Config.AICapability.ProductImageSceneAllowedTenantIDs), recorder: input.AIInvocationRecorder, logger: input.Logger,
				},
				allowed: allowed,
			}
		}
		var reviewModel productimage.ImageReviewModel
		if review := modelProvider.ReviewModel(); review != nil {
			reviewModel = &tenantAllowlistedReviewModel{
				inner: &governedReviewModel{
					inner: review, router: BuildProductImageSceneCapabilityRouter(input.AICredentialResolver, input.Config.AICapability.ProductImageSceneAllowedTenantIDs), recorder: input.AIInvocationRecorder, logger: input.Logger,
				},
				allowed: allowed,
			}
		}
		modelProvider = productimage.NewModelProvider(faithfulEditor, &tenantAllowlistedSceneGenerator{inner: governedScene, allowed: allowed}, reviewModel)
	}

	var subjectExtractor productimage.SubjectExtractor
	var whiteBgRenderer productimage.WhiteBackgroundRenderer
	var sceneRenderer productimage.SceneRenderer
	if !shouldUseModelBackedImagePipeline(modelProvider) || modelProvider.FaithfulEditor() == nil {
		subjectExtractor, err = buildSubjectExtractor(input.Config, input.ImageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create subject extractor: %w", err)
		}
		whiteBgRenderer, err = buildWhiteBackgroundRenderer(input.Config, input.ImageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create white background renderer: %w", err)
		}
	}
	if modelProvider == nil || modelProvider.SceneGenerator() == nil {
		if !shouldUseModelBackedImagePipeline(modelProvider) {
			sceneRenderer, err = buildSceneRenderer(input.ImageWorkDir)
			if err != nil {
				return nil, fmt.Errorf("create scene renderer: %w", err)
			}
		}
	}
	resolvedComponents := resolveImagePipelineComponents(modelProvider, subjectExtractor, whiteBgRenderer, sceneRenderer)
	subjectExtractor = resolvedComponents.subjectExtractor
	whiteBgRenderer = resolvedComponents.whiteBgRenderer
	sceneRenderer = resolvedComponents.sceneRenderer

	imageCapabilities := productimage.StrictServiceCapabilities()
	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:             "product_image_tasks",
		TaskRepo:              imageRepo,
		ModelProvider:         modelProvider,
		Capabilities:          &imageCapabilities,
		SourceParser:          sourceParser,
		ContextAnalyzer:       contextAnalyzer,
		ImageInspector:        imageInspector,
		ImageRanker:           productimage.NewDefaultImageRanker(),
		SubjectExtractor:      subjectExtractor,
		ImageCleaner:          imageCleaner,
		WhiteBgRenderer:       whiteBgRenderer,
		SceneRenderer:         sceneRenderer,
		AssetPublisher:        buildAssetPublisher(input.Config, input.Logger),
		CleanupTemporaryFiles: input.Config.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   input.Config.ProductImage.Lifecycle.ReuseExistingAssets,
		// Identity is enforced by governed model stages for allowlisted tenants.
		// Keep task creation compatible with legacy callers (for example Amazon)
		// that do not enter the authenticated canary path.
		RequireAIIdentity: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create image service: %w", err)
	}

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, input.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create image processor: %w", err)
	}
	imagePool := httpbootstrap.NewWorkerPool(imageProcessor, input.Config)
	imageSubmitter := &httpbootstrap.PoolSubmitter{Pool: imagePool}
	imageSvc.SetTaskSubmitter(imageSubmitter)
	imageProcessor.SetTaskSubmitter(imageSubmitter)

	imageHandler, err := NewHandler(imageSvc)
	if err != nil {
		return nil, fmt.Errorf("create image handler: %w", err)
	}

	return &Module{
		Handler:               imageHandler,
		Pool:                  imagePool,
		Service:               imageSvc,
		Closers:               closers,
		SubjectExtractor:      subjectExtractor,
		WhiteBackgroundRender: whiteBgRenderer,
		SceneRenderer:         sceneRenderer,
	}, nil
}
