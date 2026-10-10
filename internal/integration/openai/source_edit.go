package openai

import (
	"context"
	"task-processor/internal/ai"
	productimage "task-processor/internal/product/image"
)

func (a *ProductImageAdapter) EditSources(ctx context.Context, request productimage.SourceEditRequest) (productimage.Candidate, error) {
	if a == nil || a.config.Provider != "grsai" || a.config.ImageModel != "gpt-image-2.5" {
		return productimage.Candidate{}, productimage.ErrCapabilityUnsupported
	}
	cloned, err := productimage.CloneSourceEditRequest(request)
	if err != nil {
		return productimage.Candidate{}, err
	}
	if err := a.authorize(cloned.Authorization, productimage.SourceEditOperation, 1); err != nil {
		return productimage.Candidate{}, err
	}
	zero := 0
	input := &ai.ImageEditRequest{Model: a.config.ImageModel, Prompt: cloned.Prompt, Image: cloned.ReferenceBytes[0], ImageContentType: cloned.Sources[0].MediaType, MaxRetries: &zero, N: 1, Size: "1024x1024", Quality: "auto", ResponseFormat: "url"}
	for i := 1; i < len(cloned.Sources); i++ {
		input.ReferenceImages = append(input.ReferenceImages, ai.ImageInlineReference{Bytes: cloned.ReferenceBytes[i], MediaType: cloned.Sources[i].MediaType})
	}
	response, err := a.editImage(ctx, input)
	if err != nil {
		return productimage.Candidate{}, err
	}
	candidates, err := a.imageCandidates(ctx, response, cloned.Sources[0], productimage.RoleScene, productimage.SourceEditOperation, 1)
	if err != nil {
		return productimage.Candidate{}, err
	}
	if len(candidates) != 1 {
		return productimage.Candidate{}, productimage.ErrOutputValidation
	}
	candidate := candidates[0]
	if candidate.Asset.Width != 1024 || candidate.Asset.Height != 1024 {
		return productimage.Candidate{}, productimage.ErrOutputValidation
	}
	candidate.Metadata.PromptVersion = cloned.PromptVersion
	return candidate, nil
}
