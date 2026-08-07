package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

type listingKitRoutedImageClient struct {
	defaultModel      string
	defaultImage      openaiclient.ImageGenerator
	gptImage2         openaiclient.ImageGenerator
	nanobanana        openaiclient.ImageGenerator
	backgroundRemoval openaiclient.ImageGenerator
	hasResolver       bool
}

type listingKitImageRoute struct {
	RoutingKey          string
	CredentialReference string
	UsesConfiguredModel bool
}

func buildListingKitRoutedImageClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
	nanoClient := buildStrictListingKitNanobananaImageClient(cfg, resolver, listingKitImageClientNameNanobanana)
	gptClient := buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientNameGPTImage2)
	backgroundRemovalClient := buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientNameBackgroundRemoval)
	defaultClient := nanoClient
	if resolver == nil {
		defaultClient = buildStrictListingKitImageClient(cfg, resolver, listingKitImageClientName)
	}
	return &listingKitRoutedImageClient{
		defaultModel:      listingKitImageModelSelectorGPTImage2,
		defaultImage:      defaultClient,
		gptImage2:         gptClient,
		nanobanana:        nanoClient,
		backgroundRemoval: backgroundRemovalClient,
		hasResolver:       resolver != nil,
	}
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

func (c *listingKitRoutedImageClient) SupportsAsyncImageGeneration() bool {
	client, _, err := c.resolveBySelector(c.defaultModel)
	if err != nil || client == nil {
		return false
	}
	return client.SupportsAsyncImageGeneration()
}

func (c *listingKitRoutedImageClient) SubmitImageGeneration(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	if routeContext := listingkit.AIAsyncImageQueryContextFromContext(ctx); routeContext.CredentialReference != "" || routeContext.ConfigurationVersion != "" {
		client, useConfiguredModel, err := c.resolveByAsyncJob(routeContext.CredentialReference, requestModel(req, c.defaultModel))
		if err != nil {
			return nil, err
		}
		nextReq := req
		if useConfiguredModel && req != nil {
			cloned := *req
			cloned.Model = ""
			nextReq = &cloned
		}
		if version := routeContext.ConfigurationVersion; version != "" {
			versioned, ok := client.(interface {
				SubmitImageGenerationForConfigurationVersion(context.Context, string, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error)
			})
			if !ok {
				return nil, fmt.Errorf("async image client does not support configuration version recovery")
			}
			return versioned.SubmitImageGenerationForConfigurationVersion(ctx, version, nextReq)
		}
		return client.SubmitImageGeneration(ctx, nextReq)
	}
	client, nextReq, err := c.resolve(req)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageGeneration(ctx, nextReq)
}

func (c *listingKitRoutedImageClient) SubmitImageEdit(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	if routeContext := listingkit.AIAsyncImageQueryContextFromContext(ctx); routeContext.CredentialReference != "" || routeContext.ConfigurationVersion != "" {
		client, useConfiguredModel, err := c.resolveByAsyncJob(routeContext.CredentialReference, requestEditModel(req, c.defaultModel))
		if err != nil {
			return nil, err
		}
		nextReq := req
		if useConfiguredModel && req != nil {
			cloned := *req
			cloned.Model = ""
			nextReq = &cloned
		}
		if version := routeContext.ConfigurationVersion; version != "" {
			versioned, ok := client.(interface {
				SubmitImageEditForConfigurationVersion(context.Context, string, *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error)
			})
			if !ok {
				return nil, fmt.Errorf("async image client does not support configuration version recovery")
			}
			return versioned.SubmitImageEditForConfigurationVersion(ctx, version, nextReq)
		}
		return client.SubmitImageEdit(ctx, nextReq)
	}
	client, nextReq, err := c.resolveEdit(req)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageEdit(ctx, nextReq)
}

func requestModel(req *openaiclient.ImageGenerateRequest, fallback string) string {
	if req != nil && strings.TrimSpace(req.Model) != "" {
		return req.Model
	}
	return fallback
}

func requestEditModel(req *openaiclient.ImageEditRequest, fallback string) string {
	if req != nil && strings.TrimSpace(req.Model) != "" {
		return req.Model
	}
	return fallback
}

func (c *listingKitRoutedImageClient) QueryImageGeneration(ctx context.Context, jobID string) (*openaiclient.ImageAsyncQueryResponse, error) {
	client, _, err := c.resolveBySelector(c.defaultModel)
	if err != nil {
		return nil, err
	}
	return client.QueryImageGeneration(ctx, jobID)
}

func (c *listingKitRoutedImageClient) QueryImageGenerationForRoutingKey(ctx context.Context, routingKey, jobID string) (*openaiclient.ImageAsyncQueryResponse, error) {
	queryContext := listingkit.AIAsyncImageQueryContextFromContext(ctx)
	client, _, err := c.resolveByAsyncJob(queryContext.CredentialReference, routingKey)
	if err != nil {
		return nil, err
	}
	if version := queryContext.ConfigurationVersion; version != "" {
		versioned, ok := client.(interface {
			QueryImageGenerationForConfigurationVersion(context.Context, string, string) (*openaiclient.ImageAsyncQueryResponse, error)
		})
		if !ok {
			return nil, fmt.Errorf("async image client does not support configuration version recovery")
		}
		return versioned.QueryImageGenerationForConfigurationVersion(ctx, version, jobID)
	}
	return client.QueryImageGeneration(ctx, jobID)
}

func (c *listingKitRoutedImageClient) resolveByAsyncJob(credentialReference, routingKey string) (openaiclient.ImageGenerator, bool, error) {
	switch strings.TrimSpace(credentialReference) {
	case listingKitImageClientName:
		if c.defaultImage == nil {
			return nil, false, errListingKitAIClientNotConfigured(listingKitImageClientName)
		}
		return c.defaultImage, false, nil
	case listingKitImageClientNameGPTImage2:
		return c.gptImage2, true, nil
	case listingKitImageClientNameNanobanana:
		return c.nanobanana, true, nil
	case listingKitImageClientNameBackgroundRemoval:
		if c.backgroundRemoval == nil {
			return nil, false, errListingKitAIClientNotConfigured(listingKitImageClientNameBackgroundRemoval)
		}
		return c.backgroundRemoval, true, nil
	default:
		return c.resolveBySelector(routingKey)
	}
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
	route := resolveListingKitImageRoute(selector, c.hasResolver)
	switch route.CredentialReference {
	case listingKitImageClientNameGPTImage2:
		return c.gptImage2, route.UsesConfiguredModel, nil
	case listingKitImageClientNameNanobanana:
		return c.nanobanana, route.UsesConfiguredModel, nil
	case listingKitImageClientNameBackgroundRemoval:
		if c.backgroundRemoval == nil {
			return nil, false, errListingKitAIClientNotConfigured(listingKitImageClientNameBackgroundRemoval)
		}
		return c.backgroundRemoval, route.UsesConfiguredModel, nil
	default:
		if c.defaultImage == nil {
			return nil, false, errListingKitAIClientNotConfigured(listingKitImageClientName)
		}
		return c.defaultImage, route.UsesConfiguredModel, nil
	}
}

func resolveListingKitImageRoute(selector string, hasResolver bool) listingKitImageRoute {
	routingKey := strings.TrimSpace(selector)
	normalized := normalizeListingKitImageSelector(routingKey)
	switch normalized {
	case "", listingKitImageModelSelectorGPTImage2:
		return listingKitImageRoute{
			RoutingKey:          listingKitImageModelSelectorGPTImage2,
			CredentialReference: listingKitImageClientNameGPTImage2,
			UsesConfiguredModel: true,
		}
	case listingKitImageModelSelectorNano:
		return listingKitImageRoute{
			RoutingKey:          listingKitImageModelSelectorNano,
			CredentialReference: listingKitImageClientNameNanobanana,
			UsesConfiguredModel: true,
		}
	case listingKitImageModelSelectorBackgroundRemoval:
		return listingKitImageRoute{
			RoutingKey:          listingKitImageModelSelectorBackgroundRemoval,
			CredentialReference: listingKitImageClientNameBackgroundRemoval,
			UsesConfiguredModel: true,
		}
	default:
		credentialReference := listingKitImageClientName
		if hasResolver {
			credentialReference = listingKitImageClientNameNanobanana
		}
		return listingKitImageRoute{
			RoutingKey:          routingKey,
			CredentialReference: credentialReference,
		}
	}
}

func normalizeListingKitImageSelector(selector string) string {
	normalized := strings.ToLower(strings.TrimSpace(selector))
	switch {
	case normalized == listingKitImageModelSelectorGPTImage2:
		return listingKitImageModelSelectorGPTImage2
	case strings.Contains(normalized, "banana"):
		return listingKitImageModelSelectorNano
	case normalized == listingKitImageModelSelectorBackgroundRemoval || strings.Contains(normalized, "background-remov"):
		return listingKitImageModelSelectorBackgroundRemoval
	default:
		return normalized
	}
}

func enforceListingKitImageClientTimeout(clientName string, cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	switch clientName {
	case listingKitImageClientName, listingKitImageClientNameGPTImage2, listingKitImageClientNameNanobanana, listingKitImageClientNameBackgroundRemoval:
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
