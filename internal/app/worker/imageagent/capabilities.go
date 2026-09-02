package imageagentworker

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	openaiclient "task-processor/internal/integration/openai"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

// ProfileResolver is the narrow policy dependency consumed by ImageAgent.
// Implementations resolve only an explicit, fully classified policy key.
type ProfileResolver interface {
	Resolve(imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error)
}

// ImageCapabilities is the complete provider-neutral image capability set
// required by the ImageAgent worker.
type ImageCapabilities struct {
	SubjectExtractor        productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer           productimage.SceneRenderer
	Reviewer                productimage.Reviewer
	UsageQuoter             productimage.UsageQuoter
	ProfileResolver         ProfileResolver
}

type subjectProvider interface {
	productimage.SubjectExtractor
	productimage.UsageQuoter
}

type whiteBackgroundProvider interface {
	productimage.WhiteBackgroundRenderer
	productimage.UsageQuoter
}

type sceneProvider interface {
	productimage.SceneRenderer
	productimage.UsageQuoter
}

type reviewProvider interface {
	productimage.Reviewer
	productimage.UsageQuoter
}

// providerDependencies contains already configured Integration adapters. It
// intentionally accepts neither the historical application config nor a
// provider SDK so configuration parsing cannot leak into this App boundary.
type providerDependencies struct {
	Subject         subjectProvider
	WhiteBackground whiteBackgroundProvider
	Scene           sceneProvider
	Review          reviewProvider
}

// buildProductionImageCapabilities assembles only the new typed image ports.
// Provider routing is resolved per activity identity and never falls back to
// the retired ProductImage runtime.
func buildProductionImageCapabilities(runtime imageCapabilityRuntime) (ImageCapabilities, error) {
	provider, err := newRoutedOpenAIProductImageProvider(runtime.OpenAIManager)
	if err != nil {
		return ImageCapabilities{}, err
	}
	resolver, err := loadEmbeddedImagePolicyResolver()
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("load image agent policy resolver: %w", err)
	}
	return buildImageCapabilities(providerDependencies{
		Subject: provider, WhiteBackground: provider, Scene: provider, Review: provider,
	}, resolver)
}

const imageAgentOpenAIClientName = "image_gpt_image_2"

type routedOpenAIProductImageProvider struct {
	manager *openaiclient.Manager
}

func newRoutedOpenAIProductImageProvider(manager *openaiclient.Manager) (*routedOpenAIProductImageProvider, error) {
	if manager == nil {
		return nil, fmt.Errorf("image agent OpenAI manager is required")
	}
	return &routedOpenAIProductImageProvider{manager: manager}, nil
}

func (p *routedOpenAIProductImageProvider) Extract(ctx context.Context, request productimage.ExtractRequest) (productimage.Candidate, error) {
	adapter, err := p.adapter(ctx, "extract_subject", request.Authorization)
	if err != nil {
		return productimage.Candidate{}, err
	}
	return adapter.Extract(ctx, request)
}

func (p *routedOpenAIProductImageProvider) RenderWhiteBackground(ctx context.Context, request productimage.RenderRequest) (productimage.Candidate, error) {
	adapter, err := p.adapter(ctx, "render_white_background", request.Authorization)
	if err != nil {
		return productimage.Candidate{}, err
	}
	return adapter.RenderWhiteBackground(ctx, request)
}

func (p *routedOpenAIProductImageProvider) RenderScene(ctx context.Context, request productimage.SceneRequest) ([]productimage.Candidate, error) {
	adapter, err := p.adapter(ctx, "render_scene", request.Authorization)
	if err != nil {
		return nil, err
	}
	return adapter.RenderScene(ctx, request)
}

func (p *routedOpenAIProductImageProvider) Review(ctx context.Context, request productimage.ReviewRequest) (productimage.Review, error) {
	adapter, err := p.adapter(ctx, "review", request.Authorization)
	if err != nil {
		return productimage.Review{}, err
	}
	return adapter.Review(ctx, request)
}

func (p *routedOpenAIProductImageProvider) QuoteUsage(ctx context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	adapter, err := p.adapter(ctx, request.Operation, nil)
	if err != nil {
		return productimage.UsageQuote{}, err
	}
	return adapter.QuoteUsage(ctx, request)
}

func (p *routedOpenAIProductImageProvider) adapter(ctx context.Context, operation string, authorization *productimage.UsageQuote) (*openaiclient.ProductImageAdapter, error) {
	if p == nil || p.manager == nil || ctx == nil {
		return nil, productimage.ErrExternalCapabilityUnavailable
	}
	route, err := p.manager.ResolveEffectiveClientRoute(ctx, imageAgentOpenAIClientName)
	if err != nil {
		return nil, fmt.Errorf("resolve image agent provider route: %w", err)
	}
	routeReference := imageAgentRouteReference(route)
	if authorization != nil {
		quote, normalizeErr := productimage.NormalizeUsageAuthorization(*authorization, operation)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		if quote.Provider != route.ProviderID || quote.RouteReference != routeReference || quote.Model != route.ModelID ||
			quote.CredentialReference != route.CredentialReference || quote.ConfigurationVersion != route.ConfigurationVersion {
			return nil, productimage.ErrCapabilityUnsupported
		}
	}
	selection := openaiclient.ImageRouteSelection{
		CredentialReference: route.CredentialReference, ConfigurationVersion: route.ConfigurationVersion,
	}
	images, err := p.manager.GetImageClient(imageAgentOpenAIClientName)
	if err != nil {
		return nil, fmt.Errorf("resolve image agent image client: %w", err)
	}
	reviewer, err := p.manager.GetClientWithRoute(ctx, imageAgentOpenAIClientName, selection)
	if err != nil {
		return nil, fmt.Errorf("resolve image agent review client: %w", err)
	}
	return openaiclient.NewProductImageAdapter(openaiclient.ProductImageAdapterConfig{
		ImageClient: images, ReviewClient: reviewer, Prompts: openaiclient.DefaultProductImagePrompts(),
		Provider: route.ProviderID, ImageModel: route.ModelID, ReviewModel: route.ModelID,
		RouteReference: routeReference, CredentialReference: route.CredentialReference,
		ConfigurationVersion: route.ConfigurationVersion, PricingVersion: "unpriced-v1",
		MaximumSceneOutputs: 1, CostUpperBoundKnown: false,
	})
}

func imageAgentRouteReference(route openaiclient.EffectiveClientRoute) string {
	return "manager:" + strings.TrimSpace(route.CredentialReference)
}

func buildImageCapabilities(deps providerDependencies, resolver ProfileResolver) (ImageCapabilities, error) {
	if nilDependency(resolver) {
		return ImageCapabilities{}, fmt.Errorf("image agent policy resolver is required")
	}

	subject, err := productimage.NewSubjectCapability(deps.Subject)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent subject capability: %w", err)
	}
	whiteBackground, err := productimage.NewWhiteBackgroundCapability(deps.WhiteBackground)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent white-background capability: %w", err)
	}
	scene, err := productimage.NewSceneCapability(deps.Scene)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent scene capability: %w", err)
	}
	reviewer, err := productimage.NewReviewCapability(deps.Review)
	if err != nil {
		return ImageCapabilities{}, fmt.Errorf("build image agent review capability: %w", err)
	}
	quoter := &imageUsageQuoter{byOperation: map[string]productimage.UsageQuoter{
		"extract_subject":         deps.Subject,
		"render_white_background": deps.WhiteBackground,
		"render_scene":            deps.Scene,
		"review":                  deps.Review,
	}}

	return ImageCapabilities{
		SubjectExtractor: subject, WhiteBackgroundRenderer: whiteBackground,
		SceneRenderer: scene, Reviewer: reviewer, UsageQuoter: quoter,
		ProfileResolver: resolver,
	}, nil
}

// imageUsageQuoter binds each closed technical operation to the same provider
// object that executes it. Business categories and policy vocabulary never
// participate in this dispatch table.
type imageUsageQuoter struct {
	byOperation map[string]productimage.UsageQuoter
}

func (q *imageUsageQuoter) QuoteUsage(ctx context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	if q == nil {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	quoter := q.byOperation[request.Operation]
	if nilDependency(quoter) {
		return productimage.UsageQuote{}, productimage.ErrCapabilityUnsupported
	}
	return quoter.QuoteUsage(ctx, request)
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
