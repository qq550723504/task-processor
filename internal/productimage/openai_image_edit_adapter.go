package productimage

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type openAICompatibleImageGenerator interface {
	EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error)
	GetDefaultModel() string
}

type openAIImageEditClientAdapter struct {
	client openAICompatibleImageGenerator
}

func newOpenAIImageEditClientAdapter(client openAICompatibleImageGenerator) imageEditClient {
	return openAIImageEditClientAdapter{client: client}
}

func (a openAIImageEditClientAdapter) EditImage(ctx context.Context, req imageEditRequest) (*imageEditResponse, error) {
	response, err := a.client.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		Image:          req.Image,
		ImageURL:       req.ImageURL,
		ResponseFormat: req.ResponseFormat,
		N:              req.N,
		Size:           req.Size,
	})
	if err != nil || response == nil {
		return nil, err
	}
	result := &imageEditResponse{
		Data: make([]imageEditData, 0, len(response.Data)),
	}
	for _, item := range response.Data {
		result.Data = append(result.Data, imageEditData{
			B64JSON:       item.B64JSON,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	return result, nil
}

func (a openAIImageEditClientAdapter) GetDefaultModel() string {
	return a.client.GetDefaultModel()
}
