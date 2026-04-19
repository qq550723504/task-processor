package productimage

import (
	"context"
	"fmt"
)

type RemoteSceneGeneratorConfig struct {
	WorkDir string
	Client  SceneGenerationClient
}

type remoteSceneGenerator struct {
	runtime *realImageComponents
	client  SceneGenerationClient
}

func NewRemoteSceneGenerator(config RemoteSceneGeneratorConfig) (SceneGenerator, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("scene generation client is not configured")
	}
	rt, err := newRealImageComponents(config.WorkDir)
	if err != nil {
		return nil, err
	}
	return &remoteSceneGenerator{
		runtime: rt,
		client:  config.Client,
	}, nil
}

func (g *remoteSceneGenerator) GenerateScene(ctx context.Context, req *SceneGenerationRequest) (*SceneGenerationResult, error) {
	if req == nil || req.SourceAsset == nil {
		return nil, fmt.Errorf("scene generation request requires source asset")
	}
	data, filename, err := g.runtime.loadAssetBytes(req.SourceAsset)
	if err != nil {
		return nil, err
	}
	renderedImages, err := g.client.GenerateScene(ctx, data, req.SourceAsset.SourceURL, *req)
	if err != nil {
		return nil, err
	}
	assets := make([]ImageAsset, 0, len(renderedImages))
	var generationMetadata *GenerationMetadata
	for idx, image := range renderedImages {
		optimized, err := g.runtime.processor.OptimizeForAmazon(image.ImageData)
		if err != nil {
			return nil, err
		}
		path, info, err := g.runtime.writeProcessed(filename, fmt.Sprintf("scene-model-%d", idx+1), optimized)
		if err != nil {
			return nil, err
		}
		metadata := cloneMetadata(image.Metadata)
		metadata["local_path"] = path
		metadata["format"] = info.Format
		metadata["scene_mode"] = "model"
		resolvedPrompt := resolvedPromptFromRemoteMetadata(metadata, "scene_generation", req.PromptRef)
		metadata = applyPromptObservabilityMetadata(metadata, resolvedPrompt)
		assets = append(assets, ImageAsset{
			URL:        path,
			Type:       AssetTypeGalleryImage,
			SourceURL:  req.SourceAsset.SourceURL,
			Width:      info.Width,
			Height:     info.Height,
			Operations: []string{"render_scene_model"},
			Metadata:   metadata,
		})
		if generationMetadata == nil {
			generationMetadata = generationMetadataFromResult(metadata, "scene_generation", req.PromptRef)
		}
	}
	return &SceneGenerationResult{
		Assets:   assets,
		Metadata: generationMetadata,
	}, nil
}
