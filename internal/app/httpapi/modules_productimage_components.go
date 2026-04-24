package httpapi

import "task-processor/internal/productimage"

type resolvedImagePipelineComponents struct {
	subjectExtractor productimage.SubjectExtractor
	whiteBgRenderer  productimage.WhiteBackgroundRenderer
	sceneRenderer    productimage.SceneRenderer
}

func resolveImagePipelineComponents(provider productimage.ProductImageModelProvider, subjectExtractor productimage.SubjectExtractor, whiteBgRenderer productimage.WhiteBackgroundRenderer, sceneRenderer productimage.SceneRenderer) resolvedImagePipelineComponents {
	if subjectExtractor == nil && provider != nil && provider.FaithfulEditor() != nil {
		subjectExtractor = productimage.NewModelSubjectExtractor(provider.FaithfulEditor())
	}
	if whiteBgRenderer == nil && provider != nil && provider.FaithfulEditor() != nil {
		whiteBgRenderer = productimage.NewModelWhiteBackgroundRenderer(provider.FaithfulEditor())
	}
	if sceneRenderer == nil && provider != nil && provider.SceneGenerator() != nil {
		sceneRenderer = productimage.NewModelSceneRenderer(provider.SceneGenerator())
	}
	return resolvedImagePipelineComponents{
		subjectExtractor: subjectExtractor,
		whiteBgRenderer:  whiteBgRenderer,
		sceneRenderer:    sceneRenderer,
	}
}
