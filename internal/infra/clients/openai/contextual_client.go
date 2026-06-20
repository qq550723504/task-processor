package openai

import (
	"context"
)

type contextualChatClient struct {
	manager *Manager
	name    string
}

func (c *contextualChatClient) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.CreateChatCompletion(ctx, req)
}

func (c *contextualChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return "", err
	}
	return client.Generate(ctx, prompt)
}

func (c *contextualChatClient) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return "", err
	}
	return client.AnalyzeImage(ctx, imageURL, prompt)
}

func (c *contextualChatClient) GetDefaultModel() string {
	client, err := c.manager.resolveClient(context.Background(), c.name)
	if err != nil {
		return ""
	}
	return client.GetDefaultModel()
}

type contextualImageClient struct {
	manager *Manager
	name    string
}

func (c *contextualImageClient) GenerateImage(ctx context.Context, req *ImageGenerateRequest) (*ImageResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.GenerateImage(ctx, req)
}

func (c *contextualImageClient) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.EditImage(ctx, req)
}

func (c *contextualImageClient) GetDefaultModel() string {
	client, err := c.manager.resolveClient(context.Background(), c.name)
	if err != nil {
		return ""
	}
	return client.GetDefaultModel()
}

func (c *contextualImageClient) SupportsAsyncImageGeneration() bool {
	client, err := c.manager.resolveClient(context.Background(), c.name)
	if err != nil {
		return false
	}
	return client.SupportsAsyncImageGeneration()
}

func (c *contextualImageClient) SubmitImageGeneration(ctx context.Context, req *ImageGenerateRequest) (*ImageAsyncSubmitResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageGeneration(ctx, req)
}

func (c *contextualImageClient) SubmitImageEdit(ctx context.Context, req *ImageEditRequest) (*ImageAsyncSubmitResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageEdit(ctx, req)
}

func (c *contextualImageClient) QueryImageGeneration(ctx context.Context, jobID string) (*ImageAsyncQueryResponse, error) {
	client, err := c.manager.resolveClient(ctx, c.name)
	if err != nil {
		return nil, err
	}
	return client.QueryImageGeneration(ctx, jobID)
}
