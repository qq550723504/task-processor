package httpapi

import (
	"time"

	"task-processor/internal/core/config"
	productimage "task-processor/internal/productimage"
)

type imagePipelineComponentOptions struct {
	segmenter       *remoteImageServiceOptions
	whiteBackground *remoteImageServiceOptions
}

func newImagePipelineComponentOptions(cfg *config.Config) imagePipelineComponentOptions {
	if cfg == nil {
		return imagePipelineComponentOptions{}
	}
	return imagePipelineComponentOptions{
		segmenter:       remoteImageServiceOptionsFromConfig(cfg.ProductImage.Segmenter),
		whiteBackground: remoteImageServiceOptionsFromConfig(cfg.ProductImage.WhiteBackground),
	}
}

func buildSubjectExtractor(options imagePipelineComponentOptions, imageWorkDir string) (productimage.SubjectExtractor, error) {
	if options.segmenter == nil {
		return productimage.NewHybridSubjectExtractor(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPSegmentationClient(productimage.HTTPSegmentationClientConfig{
		Endpoint: options.segmenter.endpoint,
		APIKey:   options.segmenter.apiKey,
		Timeout:  time.Duration(options.segmenter.timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridSubjectExtractor(imageWorkDir, client)
}

func buildWhiteBackgroundRenderer(options imagePipelineComponentOptions, imageWorkDir string) (productimage.WhiteBackgroundRenderer, error) {
	if options.whiteBackground == nil {
		return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPWhiteBackgroundClient(productimage.HTTPWhiteBackgroundClientConfig{
		Endpoint: options.whiteBackground.endpoint,
		APIKey:   options.whiteBackground.apiKey,
		Timeout:  time.Duration(options.whiteBackground.timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, client)
}

func buildSceneRenderer(imageWorkDir string) (productimage.SceneRenderer, error) {
	return productimage.NewDefaultSceneRenderer(imageWorkDir)
}

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
