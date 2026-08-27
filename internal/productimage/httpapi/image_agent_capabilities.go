package httpapi

import (
	"fmt"

	productimage "task-processor/internal/productimage"
)

// ImageAgentCapabilities is the production ProductImage capability slice used
// by the image-agent Temporal worker. It deliberately excludes task-service and
// HTTP handler construction.
type ImageAgentCapabilities struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer           productimage.SceneRenderer
	AssetPublisher          productimage.AssetPublisher
}

type generationCapabilities struct {
	modelProvider           productimage.ProductImageModelProvider
	subjectExtractor        productimage.SubjectExtractor
	whiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	sceneRenderer           productimage.SceneRenderer
	assetPublisher          productimage.AssetPublisher
	allowedTenants          map[string]struct{}
}

// BuildImageAgentCapabilities reuses the canonical ProductImage provider,
// governance, renderer, and storage builders while failing closed unless all
// standard image-agent slot roles have real provider-backed capabilities.
func BuildImageAgentCapabilities(input RuntimeBuildInput) (ImageAgentCapabilities, error) {
	built, err := buildGenerationCapabilities(BuildModuleInput{
		Logger: input.Logger, LLMManager: input.LLMManager, OpenAIManager: input.OpenAIManager,
		AICredentialResolver: input.AICredentialResolver, AIInvocationRecorder: input.AIInvocationRecorder,
		ImageWorkDir: input.ImageWorkDir, Options: newProductImageRuntimeOptions(input.Config),
	})
	if err != nil {
		return ImageAgentCapabilities{}, err
	}
	if built.modelProvider == nil || built.modelProvider.FaithfulEditor() == nil || built.modelProvider.SceneGenerator() == nil {
		return ImageAgentCapabilities{}, fmt.Errorf("image agent requires configured ProductImage faithful-edit and scene-generation providers")
	}
	if built.assetPublisher == nil {
		return ImageAgentCapabilities{}, fmt.Errorf("image agent requires a configured ProductImage asset publisher")
	}
	return ImageAgentCapabilities{
		SubjectExtractor: built.subjectExtractor, WhiteBackgroundRenderer: built.whiteBackgroundRenderer,
		SceneRenderer: built.sceneRenderer, AssetPublisher: built.assetPublisher,
	}, nil
}

func buildGenerationCapabilities(input BuildModuleInput) (generationCapabilities, error) {
	modelProvider, err := buildModelProvider(input.Options.modelProvider, input.LLMManager, input.OpenAIManager, input.ImageWorkDir)
	if err != nil {
		return generationCapabilities{}, fmt.Errorf("create productimage model provider: %w", err)
	}
	var allowed map[string]struct{}
	if input.Options.sceneGovernance.enabled {
		if input.OpenAIManager == nil || !input.OpenAIManager.HasConfigResolver() {
			return generationCapabilities{}, fmt.Errorf("create governed productimage scene generator: resolver-backed OpenAI manager is not configured")
		}
		if modelProvider == nil {
			return generationCapabilities{}, fmt.Errorf("create governed productimage scene generator: model provider is not configured")
		}
		governedScene, governanceErr := buildGovernedProductImageSceneGenerator(input.Options.sceneGovernance, modelProvider.SceneGenerator(), input.AICredentialResolver, input.AIInvocationRecorder, input.Logger)
		if governanceErr != nil {
			return generationCapabilities{}, fmt.Errorf("create governed productimage scene generator: %w", governanceErr)
		}
		allowed = tenantIDSet(input.Options.sceneGovernance.allowedTenantIDs)
		var faithfulEditor productimage.FaithfulEditor
		if editor := modelProvider.FaithfulEditor(); editor != nil {
			faithfulEditor = &tenantAllowlistedFaithfulEditor{inner: &governedFaithfulEditor{
				inner: editor, router: BuildProductImageSceneCapabilityRouter(input.AICredentialResolver, input.Options.sceneGovernance.allowedTenantIDs), recorder: input.AIInvocationRecorder, logger: input.Logger,
			}, allowed: allowed}
		}
		var reviewModel productimage.ImageReviewModel
		if review := modelProvider.ReviewModel(); review != nil {
			reviewModel = &tenantAllowlistedReviewModel{inner: &governedReviewModel{
				inner: review, router: BuildProductImageReviewCapabilityRouter(input.AICredentialResolver, input.Options.sceneGovernance.allowedTenantIDs), recorder: input.AIInvocationRecorder, logger: input.Logger,
			}, allowed: allowed}
		}
		modelProvider = productimage.NewModelProvider(faithfulEditor, &tenantAllowlistedSceneGenerator{inner: governedScene, allowed: allowed}, reviewModel)
	}

	var subjectExtractor productimage.SubjectExtractor
	var whiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	var sceneRenderer productimage.SceneRenderer
	if !shouldUseModelBackedImagePipeline(modelProvider) || modelProvider.FaithfulEditor() == nil {
		subjectExtractor, err = buildSubjectExtractor(input.Options.imagePipelineComponents, input.ImageWorkDir)
		if err != nil {
			return generationCapabilities{}, fmt.Errorf("create subject extractor: %w", err)
		}
		whiteBackgroundRenderer, err = buildWhiteBackgroundRenderer(input.Options.imagePipelineComponents, input.ImageWorkDir)
		if err != nil {
			return generationCapabilities{}, fmt.Errorf("create white background renderer: %w", err)
		}
	}
	if (modelProvider == nil || modelProvider.SceneGenerator() == nil) && !shouldUseModelBackedImagePipeline(modelProvider) {
		sceneRenderer, err = buildSceneRenderer(input.ImageWorkDir)
		if err != nil {
			return generationCapabilities{}, fmt.Errorf("create scene renderer: %w", err)
		}
	}
	resolved := resolveImagePipelineComponents(modelProvider, subjectExtractor, whiteBackgroundRenderer, sceneRenderer)
	return generationCapabilities{
		modelProvider: modelProvider, subjectExtractor: resolved.subjectExtractor,
		whiteBackgroundRenderer: resolved.whiteBgRenderer, sceneRenderer: resolved.sceneRenderer,
		assetPublisher: buildAssetPublisher(input.Options.assetPublisher, input.Logger), allowedTenants: allowed,
	}, nil
}
