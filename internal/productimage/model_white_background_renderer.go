package productimage

import (
	"context"
	"fmt"
	"strconv"
)

type modelWhiteBackgroundRenderer struct {
	editor FaithfulEditor
}

func NewModelWhiteBackgroundRenderer(editor FaithfulEditor) WhiteBackgroundRenderer {
	return &modelWhiteBackgroundRenderer{editor: editor}
}

func (r *modelWhiteBackgroundRenderer) Render(ctx context.Context, asset *ImageAsset, context *ProductContext) (*ImageAsset, error) {
	if r.editor == nil {
		return nil, fmt.Errorf("faithful editor is not configured")
	}

	result, err := r.editor.Edit(ctx, &FaithfulEditRequest{
		SourceAsset:    asset,
		ProductContext: context,
		Operation:      "render_white_background",
		PromptRef:      "productimage/white-background/default",
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Asset == nil {
		return nil, fmt.Errorf("faithful editor returned no asset")
	}

	if result.Asset.Metadata == nil {
		result.Asset.Metadata = map[string]string{}
	}
	if result.Metadata != nil {
		result.Asset.Metadata["model_provider"] = result.Metadata.Provider
		result.Asset.Metadata["model_family"] = result.Metadata.ModelFamily
		result.Asset.Metadata["generation_mode"] = result.Metadata.GenerationMode
		result.Asset.Metadata["prompt_ref"] = result.Metadata.PromptRef
		if result.Metadata.ReviewConfidence > 0 {
			result.Asset.Metadata["review_confidence"] = strconv.FormatFloat(result.Metadata.ReviewConfidence, 'f', -1, 64)
		}
	}

	return result.Asset, nil
}
