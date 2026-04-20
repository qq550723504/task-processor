package productimage

import (
	"context"
	"fmt"
)

type modelSceneRenderer struct {
	generator SceneGenerator
}

func NewModelSceneRenderer(generator SceneGenerator) SceneRenderer {
	return &modelSceneRenderer{generator: generator}
}

func (r *modelSceneRenderer) Render(ctx context.Context, asset *ImageAsset, context *ProductContext) ([]ImageAsset, error) {
	if r.generator == nil {
		return nil, fmt.Errorf("scene generator is not configured")
	}

	result, err := r.generator.GenerateScene(ctx, buildSceneGenerationRequest(asset, context))
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Assets) == 0 {
		return nil, fmt.Errorf("scene generator returned no assets")
	}

	if result.Metadata != nil {
		for idx := range result.Assets {
			if result.Assets[idx].Metadata == nil {
				result.Assets[idx].Metadata = map[string]string{}
			}
			result.Assets[idx].Metadata = applyGenerationMetadataMap(result.Assets[idx].Metadata, result.Metadata)
		}
	}

	return result.Assets, nil
}
