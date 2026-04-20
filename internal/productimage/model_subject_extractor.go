package productimage

import (
	"context"
	"fmt"

	"task-processor/internal/prompt"
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
		PromptRef:      prompt.KProductImageSubjectExtract,
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
