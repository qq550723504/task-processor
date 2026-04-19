package productimage

import (
	"context"
	"fmt"

	"task-processor/internal/prompt"
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
		PromptRef:      prompt.KProductImageWhiteBackgroundDefault,
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
	result.Asset.Metadata = applyGenerationMetadataMap(result.Asset.Metadata, result.Metadata)

	return result.Asset, nil
}
