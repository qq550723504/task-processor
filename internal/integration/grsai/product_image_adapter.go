package grsai

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"task-processor/internal/ai"
	openaiprovider "task-processor/internal/integration/openai"
)

type ProductImageAdapterConfig struct {
	Client  ai.ImageGenerator
	Adapter openaiprovider.ProductImageAdapterConfig
}

func NewProductImageAdapter(config ProductImageAdapterConfig) (*openaiprovider.ProductImageAdapter, error) {
	if nilGRSAIProductImageClient(config.Client) {
		return nil, fmt.Errorf("grsai product image client is required")
	}
	if config.Adapter.ImageClient != nil {
		return nil, fmt.Errorf("grsai product image adapter owns the image client")
	}
	pinned, err := newPinnedProductImageClient(
		config.Client, config.Adapter.ImageModel, config.Adapter.CredentialReference, config.Adapter.ConfigurationVersion,
	)
	if err != nil {
		return nil, err
	}
	config.Adapter.ImageClient = pinned
	return openaiprovider.NewProductImageAdapter(config.Adapter)
}

type pinnedProductImageClient struct {
	client               ai.ImageGenerator
	model                string
	credentialReference  string
	configurationVersion string
}

func newPinnedProductImageClient(client ai.ImageGenerator, model, credentialReference, configurationVersion string) (*pinnedProductImageClient, error) {
	if nilGRSAIProductImageClient(client) {
		return nil, fmt.Errorf("grsai product image client is required")
	}
	for name, value := range map[string]string{
		"model": model, "credential reference": credentialReference, "configuration version": configurationVersion,
	} {
		if value == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("grsai product image %s is required and must be canonical", name)
		}
	}
	if defaultModel := strings.TrimSpace(client.GetDefaultModel()); defaultModel != "" && defaultModel != model {
		return nil, fmt.Errorf("grsai product image model does not match the configured client")
	}
	return &pinnedProductImageClient{
		client: client, model: model, credentialReference: credentialReference, configurationVersion: configurationVersion,
	}, nil
}

func (c *pinnedProductImageClient) EditImageWithRoute(ctx context.Context, request *ai.ImageEditRequest, route openaiprovider.ImageRouteSelection) (*ai.ImageResponse, error) {
	if c == nil || request == nil || strings.TrimSpace(request.Model) != c.model ||
		route.CredentialReference != c.credentialReference || route.ConfigurationVersion != c.configurationVersion {
		return nil, fmt.Errorf("grsai product image route changed before dispatch")
	}
	return c.client.EditImage(ctx, request)
}

func (*pinnedProductImageClient) GenerateImage(context.Context, *ai.ImageGenerateRequest) (*ai.ImageResponse, error) {
	return nil, fmt.Errorf("grsai product image generation requires an exact route")
}

func (*pinnedProductImageClient) EditImage(context.Context, *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	return nil, fmt.Errorf("grsai product image edit requires an exact route")
}

func (c *pinnedProductImageClient) GetDefaultModel() string {
	if c == nil {
		return ""
	}
	return c.model
}

func (*pinnedProductImageClient) SupportsAsyncImageGeneration() bool { return false }

func (*pinnedProductImageClient) SubmitImageGeneration(context.Context, *ai.ImageGenerateRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}

func (*pinnedProductImageClient) SubmitImageEdit(context.Context, *ai.ImageEditRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}

func (*pinnedProductImageClient) QueryImageGeneration(context.Context, string) (*ai.ImageAsyncQueryResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}

func nilGRSAIProductImageClient(client ai.ImageGenerator) bool {
	if client == nil {
		return true
	}
	reflected := reflect.ValueOf(client)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

var _ ai.ImageGenerator = (*pinnedProductImageClient)(nil)
