package productimage

import (
	"context"
	"fmt"
	"strconv"
)

type modelSubjectExtractor struct {
	editor FaithfulEditor
}

func NewModelSubjectExtractor(editor FaithfulEditor) SubjectExtractor {
	return &modelSubjectExtractor{editor: editor}
}

func (e *modelSubjectExtractor) Extract(ctx context.Context, imageURL string, context *ProductContext) (*ImageAsset, error) {
	if e.editor == nil {
		return nil, fmt.Errorf("faithful editor is not configured")
	}

	result, err := e.editor.Edit(ctx, &FaithfulEditRequest{
		SourceAsset: &ImageAsset{
			URL:       imageURL,
			SourceURL: imageURL,
			Type:      AssetTypeSourceImage,
		},
		ProductContext: context,
		Operation:      "extract_subject",
		PromptRef:      "productimage/subject/extract",
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
