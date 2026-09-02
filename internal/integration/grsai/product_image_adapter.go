package grsai

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"task-processor/internal/ai"
	productimage "task-processor/internal/product/image"
)

// ProductImageProvider is the complete provider-neutral capability surface a
// GRSAI-compatible delegate must implement.
type ProductImageProvider interface {
	productimage.SubjectExtractor
	productimage.WhiteBackgroundRenderer
	productimage.SceneRenderer
	productimage.Reviewer
	productimage.UsageQuoter
}

type ProductImageAdapterFactory func(ai.RouteBoundImageGenerator) (ProductImageProvider, error)

type ProductImageAdapterConfig struct {
	Client               ai.ImageGenerator
	ImageModel           string
	RouteReference       string
	CredentialReference  string
	ConfigurationVersion string
	Build                ProductImageAdapterFactory
}

// NewProductImageAdapter pins the concrete GRSAI client before handing it to
// an App-selected compatible capability adapter. The Integration packages do
// not depend on one another; App is the only concrete composition boundary.
func NewProductImageAdapter(config ProductImageAdapterConfig) (ProductImageProvider, error) {
	if nilGRSAIProductImageClient(config.Client) {
		return nil, fmt.Errorf("grsai product image client is required")
	}
	if config.Build == nil {
		return nil, fmt.Errorf("grsai product image adapter factory is required")
	}
	for name, value := range map[string]string{
		"model": config.ImageModel, "route reference": config.RouteReference,
		"credential reference": config.CredentialReference, "configuration version": config.ConfigurationVersion,
	} {
		if value == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("grsai product image %s is required and must be canonical", name)
		}
	}
	pinned, err := newPinnedProductImageClient(
		config.Client, config.ImageModel, config.CredentialReference, config.ConfigurationVersion,
	)
	if err != nil {
		return nil, err
	}
	delegate, err := config.Build(pinned)
	if err != nil {
		return nil, fmt.Errorf("build grsai product image delegate: %w", err)
	}
	if nilGRSAIProductImageProvider(delegate) {
		return nil, fmt.Errorf("grsai product image delegate is required")
	}
	return &productImageAdapter{
		delegate: delegate, imageModel: config.ImageModel, routeReference: config.RouteReference,
		credentialReference: config.CredentialReference, configurationVersion: config.ConfigurationVersion,
	}, nil
}

type productImageAdapter struct {
	delegate             ProductImageProvider
	imageModel           string
	routeReference       string
	credentialReference  string
	configurationVersion string
}

func (a *productImageAdapter) Extract(ctx context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	return a.delegate.Extract(ctx, request)
}

func (a *productImageAdapter) RenderWhiteBackground(ctx context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	return a.delegate.RenderWhiteBackground(ctx, request)
}

func (a *productImageAdapter) RenderScene(ctx context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	return a.delegate.RenderScene(ctx, request)
}

func (a *productImageAdapter) Review(ctx context.Context, request productimage.ReviewRequest) (productimage.Review, error) {
	return a.delegate.Review(ctx, request)
}

func (a *productImageAdapter) QuoteUsage(ctx context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	if a == nil || nilGRSAIProductImageProvider(a.delegate) {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	quote, err := a.delegate.QuoteUsage(ctx, request)
	if err != nil {
		return productimage.UsageQuote{}, err
	}
	if quote.Operation != request.Operation || quote.RouteReference != a.routeReference ||
		quote.CredentialReference != a.credentialReference || quote.ConfigurationVersion != a.configurationVersion {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	switch request.Operation {
	case "extract_subject", "render_white_background", "render_scene":
		if quote.Model != a.imageModel {
			return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
		}
	}
	return quote, nil
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

func (c *pinnedProductImageClient) EditImageWithRoute(ctx context.Context, request *ai.ImageEditRequest, route ai.ImageRouteSelection) (*ai.ImageResponse, error) {
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
	return nilGRSAIProductImageValue(client)
}

func nilGRSAIProductImageProvider(provider ProductImageProvider) bool {
	return nilGRSAIProductImageValue(provider)
}

func nilGRSAIProductImageValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

var _ ai.RouteBoundImageGenerator = (*pinnedProductImageClient)(nil)
var _ ProductImageProvider = (*productImageAdapter)(nil)
