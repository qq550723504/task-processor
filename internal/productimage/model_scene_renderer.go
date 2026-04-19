package productimage

import (
	"context"
	"fmt"
	"strconv"

	"task-processor/internal/prompt"
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

	result, err := r.generator.GenerateScene(ctx, &SceneGenerationRequest{
		SourceAsset:    asset,
		ProductContext: context,
		PromptRef:      prompt.KProductImageSceneDefault,
		SceneIntent:    "gallery_scene",
	})
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
			result.Assets[idx].Metadata["model_provider"] = result.Metadata.Provider
			result.Assets[idx].Metadata["model_family"] = result.Metadata.ModelFamily
			result.Assets[idx].Metadata["generation_mode"] = result.Metadata.GenerationMode
			result.Assets[idx].Metadata["prompt_ref"] = result.Metadata.PromptRef
			if result.Metadata.ReviewConfidence > 0 {
				result.Assets[idx].Metadata["review_confidence"] = strconv.FormatFloat(result.Metadata.ReviewConfidence, 'f', -1, 64)
			}
		}
	}

	return result.Assets, nil
}
