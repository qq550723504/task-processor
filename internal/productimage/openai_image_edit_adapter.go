package productimage

import (
	"context"
	"fmt"

	"task-processor/internal/ai"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type openAICompatibleImageGenerator interface {
	EditImage(ctx context.Context, req *ai.ImageEditRequest) (*ai.ImageResponse, error)
	GetDefaultModel() string
}

type openAICompatibleImageGeneratorWithRoute interface {
	EditImageWithRoute(ctx context.Context, req *ai.ImageEditRequest, selection openaiclient.ImageRouteSelection) (*ai.ImageResponse, error)
}

type openAIImageEditClientAdapter struct {
	client openAICompatibleImageGenerator
}

func newOpenAIImageEditClientAdapter(client openAICompatibleImageGenerator) imageEditClient {
	return openAIImageEditClientAdapter{client: client}
}

func (a openAIImageEditClientAdapter) EditImage(ctx context.Context, req imageEditRequest) (*imageEditResponse, error) {
	return convertOpenAIImageEditResponse(a.client.EditImage(ctx, &ai.ImageEditRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		Image:          req.Image,
		ImageURL:       req.ImageURL,
		ResponseFormat: req.ResponseFormat,
		N:              req.N,
		Size:           req.Size,
	}))
}

func (a openAIImageEditClientAdapter) EditImageWithRoute(ctx context.Context, req imageEditRequest, route SceneGenerationRoute) (*imageEditResponse, error) {
	client, ok := a.client.(openAICompatibleImageGeneratorWithRoute)
	if !ok {
		return nil, fmt.Errorf("route-bound image client is not supported")
	}
	return convertOpenAIImageEditResponse(client.EditImageWithRoute(ctx, &ai.ImageEditRequest{
		Model:          req.Model,
		Prompt:         req.Prompt,
		Image:          req.Image,
		ImageURL:       req.ImageURL,
		ResponseFormat: req.ResponseFormat,
		N:              req.N,
		Size:           req.Size,
	}, openaiclient.ImageRouteSelection{
		CredentialReference:  route.CredentialReference,
		ConfigurationVersion: route.ConfigurationVersion,
	}))
}

func convertOpenAIImageEditResponse(response *ai.ImageResponse, err error) (*imageEditResponse, error) {
	if err != nil || response == nil {
		return nil, err
	}
	result := &imageEditResponse{
		Data: make([]imageEditData, 0, len(response.Data)),
	}
	for _, item := range response.Data {
		result.Data = append(result.Data, imageEditData{
			B64JSON:       item.B64JSON,
			URL:           item.URL,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	return result, nil
}

func (a openAIImageEditClientAdapter) GetDefaultModel() string {
	return a.client.GetDefaultModel()
}
