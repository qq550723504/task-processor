package httpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
)

const studioBackgroundRemovalPrompt = "Remove the image background precisely. Preserve the complete foreground artwork, text, thin lines, holes, and internal white areas. Return a PNG with a real alpha channel and no checkerboard, white, or colored replacement background."

type listingKitBackgroundRemover struct {
	client openaiclient.ImageGenerator
}

func adaptListingKitBackgroundRemover(client openaiclient.ImageGenerator) listingkit.StudioBackgroundRemover {
	if client == nil {
		return nil
	}
	return listingKitBackgroundRemover{client: client}
}

func (r listingKitBackgroundRemover) Remove(ctx context.Context, input []byte, contentType string) (*listingkit.StudioBackgroundRemovalResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("background removal client is not configured")
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("background removal input is empty")
	}
	return r.remove(ctx, &openaiclient.ImageEditRequest{
		Prompt:           studioBackgroundRemovalPrompt,
		Image:            input,
		ImageContentType: strings.TrimSpace(contentType),
		ResponseFormat:   "b64_json",
		N:                1,
	})
}

func (r listingKitBackgroundRemover) RemoveFromURL(ctx context.Context, imageURL string) (*listingkit.StudioBackgroundRemovalResult, error) {
	if r.client == nil {
		return nil, fmt.Errorf("background removal client is not configured")
	}
	if strings.TrimSpace(imageURL) == "" {
		return nil, fmt.Errorf("background removal image url is empty")
	}
	return r.remove(ctx, &openaiclient.ImageEditRequest{
		Prompt:         studioBackgroundRemovalPrompt,
		ImageURL:       strings.TrimSpace(imageURL),
		ImageURLs:      []string{strings.TrimSpace(imageURL)},
		ResponseFormat: "b64_json",
		N:              1,
	})
}

func (r listingKitBackgroundRemover) remove(ctx context.Context, req *openaiclient.ImageEditRequest) (*listingkit.StudioBackgroundRemovalResult, error) {
	response, err := r.client.EditImage(ctx, req)
	if err != nil {
		return nil, err
	}
	if response == nil || len(response.Data) == 0 {
		return nil, fmt.Errorf("background removal returned no image data")
	}
	first := response.Data[0]
	data, outputType, err := loadBackgroundRemovalImage(ctx, first)
	if err != nil {
		return nil, err
	}
	return &listingkit.StudioBackgroundRemovalResult{
		Data:        data,
		ContentType: outputType,
		Model:       strings.TrimSpace(r.client.GetDefaultModel()),
		RequestID:   strings.TrimSpace(response.RequestID),
		RawResponse: strings.TrimSpace(response.RawResponse),
		Usage:       listingkit.AIUsage(response.Usage),
	}, nil
}

func loadBackgroundRemovalImage(ctx context.Context, image openaiclient.ImageData) ([]byte, string, error) {
	if strings.TrimSpace(image.B64JSON) != "" {
		data, err := base64.StdEncoding.DecodeString(image.B64JSON)
		if err != nil {
			return nil, "", fmt.Errorf("decode background removal image: %w", err)
		}
		return data, "image/png", nil
	}
	if strings.TrimSpace(image.URL) == "" {
		return nil, "", fmt.Errorf("background removal image contains neither b64_json nor url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download background removal image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("download background removal image returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read background removal image: %w", err)
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "image/png"
	}
	return data, contentType, nil
}

type strictListingKitConfiguredImageClient struct {
	clientName string
	resolver   openaiclient.ClientConfigResolver
	fallback   *openaiclient.ClientConfig
	mu         sync.Mutex
	cache      map[string]openaiclient.ImageGenerator
	build      func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error)
}

func (c *strictListingKitConfiguredImageClient) GenerateImage(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.GenerateImage(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) EditImage(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.EditImage(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) GetDefaultModel() string {
	return ""
}

func (c *strictListingKitConfiguredImageClient) SupportsAsyncImageGeneration() bool {
	client, err := c.resolve(context.Background())
	if err != nil || client == nil {
		return false
	}
	return client.SupportsAsyncImageGeneration()
}

func (c *strictListingKitConfiguredImageClient) SubmitImageGeneration(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageGeneration(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) SubmitImageEdit(ctx context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.SubmitImageEdit(ctx, req)
}

func (c *strictListingKitConfiguredImageClient) QueryImageGeneration(ctx context.Context, jobID string) (*openaiclient.ImageAsyncQueryResponse, error) {
	client, err := c.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return client.QueryImageGeneration(ctx, jobID)
}

func (c *strictListingKitConfiguredImageClient) resolve(ctx context.Context) (openaiclient.ImageGenerator, error) {
	return resolveStrictListingKitImageClient(ctx, c.clientName, c.resolver, c.fallback, &c.mu, c.cache, c.build)
}

func resolveStrictListingKitImageClient(
	ctx context.Context,
	clientName string,
	resolver openaiclient.ClientConfigResolver,
	fallback *openaiclient.ClientConfig,
	mu *sync.Mutex,
	cache map[string]openaiclient.ImageGenerator,
	build func(cfg *openaiclient.ClientConfig) (openaiclient.ImageGenerator, error),
) (openaiclient.ImageGenerator, error) {
	if resolver == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	resolved, err := resolver.ResolveClientConfig(ctx, clientName, fallback)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Config == nil {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	config := enforceListingKitImageClientTimeout(normalizeListingKitClientName(clientName), resolved.Config)
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.Model) == "" {
		return nil, errListingKitAIClientNotConfigured(clientName)
	}
	cacheKey := strings.TrimSpace(resolved.CacheKey)
	if cacheKey == "" {
		cacheKey = fmt.Sprintf("%s:%s:%s:%s", normalizeListingKitClientName(clientName), config.APIKey, config.BaseURL, config.Model)
	}
	mu.Lock()
	defer mu.Unlock()
	if client := cache[cacheKey]; client != nil {
		return client, nil
	}
	client, err := build(config)
	if err != nil {
		return nil, fmt.Errorf("create listingkit ai client %q: %w", normalizeListingKitClientName(clientName), err)
	}
	cache[cacheKey] = client
	return client, nil
}
