package productimage

import (
	"context"
	"fmt"
)

type RemoteFaithfulEditorConfig struct {
	WorkDir         string
	Segmenter       SegmentationClient
	WhiteBackground WhiteBackgroundClient
}

type remoteFaithfulEditor struct {
	runtime         *realImageComponents
	segmenter       SegmentationClient
	whiteBackground WhiteBackgroundClient
}

func NewRemoteFaithfulEditor(config RemoteFaithfulEditorConfig) (FaithfulEditor, error) {
	if config.Segmenter == nil && config.WhiteBackground == nil {
		return nil, fmt.Errorf("remote faithful editor requires at least one remote model client")
	}
	rt, err := newRealImageComponents(config.WorkDir)
	if err != nil {
		return nil, err
	}
	return &remoteFaithfulEditor{
		runtime:         rt,
		segmenter:       config.Segmenter,
		whiteBackground: config.WhiteBackground,
	}, nil
}

func (e *remoteFaithfulEditor) Edit(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error) {
	if req == nil {
		return nil, fmt.Errorf("faithful edit request cannot be nil")
	}
	switch req.Operation {
	case "extract_subject":
		return e.extractSubject(ctx, req)
	case "render_white_background":
		return e.renderWhiteBackground(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported faithful edit operation: %s", req.Operation)
	}
}

func (e *remoteFaithfulEditor) extractSubject(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error) {
	if e.segmenter == nil {
		return nil, fmt.Errorf("segmenter is not configured")
	}
	sourceAsset := req.SourceAsset
	if sourceAsset == nil || sourceAsset.URL == "" {
		return nil, fmt.Errorf("source asset is required for subject extraction")
	}
	data, filename, err := e.runtime.loadAssetBytes(sourceAsset)
	if err != nil {
		return nil, err
	}
	result, err := e.segmenter.SegmentSubject(ctx, data, sourceAsset.SourceURL)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.ImageData) == 0 {
		return nil, fmt.Errorf("segmenter returned no image data")
	}
	optimized, err := e.runtime.processor.OptimizeForAmazon(result.ImageData)
	if err != nil {
		return nil, err
	}
	path, info, err := e.runtime.writeProcessed(filename, "subject-model", optimized)
	if err != nil {
		return nil, err
	}

	metadata := cloneMetadata(result.Metadata)
	metadata["local_path"] = path
	metadata["format"] = info.Format
	if result.BBox != "" {
		metadata["subject_box"] = result.BBox
	}
	if req.ProductContext != nil && req.ProductContext.ProductType != "" {
		metadata["product_type"] = req.ProductContext.ProductType
	}
	generationMetadata := generationMetadataFromResult(metadata, "extract_subject", req.PromptRef)
	asset := &ImageAsset{
		URL:        path,
		Type:       AssetTypeSubjectCutout,
		SourceURL:  sourceAsset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: []string{"extract_subject_model"},
		Metadata:   metadata,
	}
	return &FaithfulEditResult{Asset: asset, Metadata: generationMetadata}, nil
}

func (e *remoteFaithfulEditor) renderWhiteBackground(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error) {
	if e.whiteBackground == nil {
		return nil, fmt.Errorf("white background client is not configured")
	}
	sourceAsset := req.SourceAsset
	if sourceAsset == nil {
		return nil, fmt.Errorf("source asset is required for white background rendering")
	}
	data, filename, err := e.runtime.loadAssetBytes(sourceAsset)
	if err != nil {
		return nil, err
	}
	result, err := e.whiteBackground.RenderWhiteBackground(ctx, data, sourceAsset.SourceURL)
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.ImageData) == 0 {
		return nil, fmt.Errorf("white background client returned no image data")
	}
	optimized, err := e.runtime.processor.OptimizeForAmazon(result.ImageData)
	if err != nil {
		return nil, err
	}
	path, info, err := e.runtime.writeProcessed(filename, "white-bg-model", optimized)
	if err != nil {
		return nil, err
	}

	metadata := cloneMetadata(result.Metadata)
	metadata["local_path"] = path
	metadata["format"] = info.Format
	metadata["background"] = "white"
	metadata["background_mode"] = "model"
	generationMetadata := generationMetadataFromResult(metadata, "render_white_background", req.PromptRef)
	asset := &ImageAsset{
		URL:        path,
		Type:       AssetTypeWhiteBgImage,
		SourceURL:  sourceAsset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: []string{"render_white_bg_model"},
		Metadata:   metadata,
	}
	return &FaithfulEditResult{Asset: asset, Metadata: generationMetadata}, nil
}

func generationMetadataFromResult(metadata map[string]string, operation string, promptRef string) *GenerationMetadata {
	normalizedPromptRef := normalizeProductImagePromptKey(promptRef, "")
	modelMetadata := &GenerationMetadata{
		Provider:       metadata["provider"],
		ModelFamily:    metadata["model_family"],
		GenerationMode: metadata["generation_mode"],
		PromptRef:      normalizedPromptRef,
	}
	if modelMetadata.Provider == "" {
		modelMetadata.Provider = "remote_model_service"
	}
	if modelMetadata.ModelFamily == "" {
		modelMetadata.ModelFamily = metadata["model"]
	}
	if modelMetadata.GenerationMode == "" {
		modelMetadata.GenerationMode = operation
	}
	return modelMetadata
}
