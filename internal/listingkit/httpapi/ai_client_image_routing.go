package httpapi

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type listingKitRoutedImageClient struct {
	defaultModel string
	defaultImage openaiclient.ImageGenerator
	gptImage2    openaiclient.ImageGenerator
	nanobanana   openaiclient.ImageGenerator
}

func (c *listingKitRoutedImageClient) GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	client, nextReq, err := c.resolve(req)
	if err != nil {
		return nil, err
	}
	return client.GenerateImage(ctx, nextReq)
}

func (c *listingKitRoutedImageClient) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	client, nextReq, err := c.resolveEdit(req)
	if err != nil {
		return nil, err
	}
	return client.EditImage(ctx, nextReq)
}

func (c *listingKitRoutedImageClient) GetDefaultModel() string {
	return c.defaultModel
}

func (c *listingKitRoutedImageClient) resolve(req *openaiclient.ImageGenerateRequest) (openaiclient.ImageGenerator, *openaiclient.ImageGenerateRequest, error) {
	selector := c.defaultModel
	if req != nil && strings.TrimSpace(req.Model) != "" {
		selector = req.Model
	}
	client, useConfiguredModel, err := c.resolveBySelector(selector)
	if err != nil {
		return nil, nil, err
	}
	if !useConfiguredModel || req == nil {
		return client, req, nil
	}
	cloned := *req
	cloned.Model = ""
	return client, &cloned, nil
}

func (c *listingKitRoutedImageClient) resolveEdit(req *openaiclient.ImageEditRequest) (openaiclient.ImageGenerator, *openaiclient.ImageEditRequest, error) {
	selector := c.defaultModel
	if req != nil && strings.TrimSpace(req.Model) != "" {
		selector = req.Model
	}
	client, useConfiguredModel, err := c.resolveBySelector(selector)
	if err != nil {
		return nil, nil, err
	}
	if !useConfiguredModel || req == nil {
		return client, req, nil
	}
	cloned := *req
	cloned.Model = ""
	return client, &cloned, nil
}

func (c *listingKitRoutedImageClient) resolveBySelector(selector string) (openaiclient.ImageGenerator, bool, error) {
	switch normalizeListingKitImageSelector(selector) {
	case listingKitImageModelSelectorGPTImage2:
		return c.gptImage2, true, nil
	case listingKitImageModelSelectorNano:
		return c.nanobanana, true, nil
	default:
		if c.defaultImage == nil {
			return nil, false, errListingKitAIClientNotConfigured(listingKitImageClientName)
		}
		return c.defaultImage, false, nil
	}
}

func normalizeListingKitImageSelector(selector string) string {
	normalized := strings.ToLower(strings.TrimSpace(selector))
	switch {
	case normalized == listingKitImageModelSelectorGPTImage2:
		return listingKitImageModelSelectorGPTImage2
	case strings.Contains(normalized, "banana"):
		return listingKitImageModelSelectorNano
	default:
		return normalized
	}
}

func enforceListingKitImageClientTimeout(clientName string, cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	switch clientName {
	case listingKitImageClientName, listingKitImageClientNameGPTImage2, listingKitImageClientNameNanobanana:
		if cfg.Timeout >= listingKitStudioImageMinTimeout {
			return cfg
		}
		cloned := *cfg
		cloned.Timeout = listingKitStudioImageMinTimeout
		return &cloned
	default:
		return cfg
	}
}
