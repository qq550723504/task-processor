package imageagentworker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	openaiclient "task-processor/internal/integration/openai"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
	productimage "task-processor/internal/product/image"
)

func TestBuildImageCapabilitiesFailsClosedOnEveryMissingDependency(t *testing.T) {
	complete := completeProviderDependencies()
	tests := map[string]func(*providerDependencies){
		"subject provider":          func(deps *providerDependencies) { deps.Subject = nil },
		"white background provider": func(deps *providerDependencies) { deps.WhiteBackground = nil },
		"scene provider":            func(deps *providerDependencies) { deps.Scene = nil },
		"review provider":           func(deps *providerDependencies) { deps.Review = nil },
	}
	for name, remove := range tests {
		t.Run(name, func(t *testing.T) {
			deps := complete
			remove(&deps)
			_, err := buildImageCapabilities(deps, &stubProfileResolver{})
			require.Error(t, err)
		})
	}

	_, err := buildImageCapabilities(complete, nil)
	require.Error(t, err)
}

func TestBuildImageCapabilitiesRejectsTypedNilDependencies(t *testing.T) {
	deps := completeProviderDependencies()
	var subject *stubProductImageProvider
	deps.Subject = subject

	_, err := buildImageCapabilities(deps, &stubProfileResolver{})
	require.Error(t, err)
}

func TestBuildImageCapabilitiesInstallsProductImageBoundaryValidation(t *testing.T) {
	deps := completeProviderDependencies()
	deps.Subject = &stubProductImageProvider{subjectCandidate: productimage.Candidate{
		Asset: productimage.Asset{
			URL:           "https://source.example/item.png",
			SourceURL:     "https://source.example/item.png",
			SourceAssetID: "source-1",
			Role:          productimage.RoleSubject,
			Width:         100,
			Height:        100,
			Operations:    []string{"extract_subject"},
		},
	}}

	capabilities, err := buildImageCapabilities(deps, &stubProfileResolver{})
	require.NoError(t, err)
	_, err = capabilities.SubjectExtractor.Extract(context.Background(), productimage.ExtractRequest{
		Source: productimage.Asset{
			URL:           "https://source.example/item.png",
			SourceURL:     "https://source.example/item.png",
			SourceAssetID: "source-1",
			Role:          productimage.RoleSource,
			Width:         100,
			Height:        100,
			Operations:    []string{"source"},
		},
		Product: productimage.ProductContext{ProductKey: "product-1", Title: "Product"},
	})
	require.ErrorIs(t, err, productimage.ErrOutputValidation)
}

func TestBuildImageCapabilitiesRoutesEveryQuoteToItsExecutingProvider(t *testing.T) {
	deps := completeProviderDependencies()
	capabilities, err := buildImageCapabilities(deps, &stubProfileResolver{})
	require.NoError(t, err)

	tests := []struct {
		operation string
		provider  *stubProductImageProvider
	}{
		{operation: "extract_subject", provider: deps.Subject.(*stubProductImageProvider)},
		{operation: "render_white_background", provider: deps.WhiteBackground.(*stubProductImageProvider)},
		{operation: "render_scene", provider: deps.Scene.(*stubProductImageProvider)},
		{operation: "review", provider: deps.Review.(*stubProductImageProvider)},
	}
	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			request := productimage.UsageQuoteRequest{Operation: test.operation, InputFingerprint: "input-v1", MaximumOutputs: 2}
			quote, quoteErr := capabilities.UsageQuoter.QuoteUsage(context.Background(), request)
			require.NoError(t, quoteErr)
			require.Equal(t, test.provider.quote, quote)
			require.Equal(t, request, test.provider.quoteRequest)
		})
	}

	_, err = capabilities.UsageQuoter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{Operation: "unknown", InputFingerprint: "input-v1"})
	require.ErrorIs(t, err, productimage.ErrCapabilityUnsupported)
}

func TestBuildImageCapabilitiesPreservesExactResolver(t *testing.T) {
	resolver := &stubProfileResolver{profile: imagepolicy.ProductImageProfile{
		Key:           imagepolicy.PolicyKey{Marketplace: "marketplace-a", Country: "sg", Family: "home", SceneCategory: "studio"},
		PolicyVersion: "policy-v1",
	}}
	capabilities, err := buildImageCapabilities(completeProviderDependencies(), resolver)
	require.NoError(t, err)
	input := imagepolicy.ProfileInput{Marketplace: "marketplace-a", Country: "sg", Family: "home", SceneCategory: "studio"}
	profile, err := capabilities.ProfileResolver.Resolve(input)
	require.NoError(t, err)
	require.Equal(t, resolver.profile, profile)
	require.Equal(t, input, resolver.input)
}

func TestBuildProductionImageCapabilitiesQuotesCurrentRouteAndRejectsStaleAuthorization(t *testing.T) {
	clientConfig := openaiclient.NewClientConfig("test-key", "gpt-image-test", "https://provider.example.test/v1", 30)
	manager, err := openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients: map[string]*openaiclient.ClientConfig{
			imageAgentOpenAIClientName:       clientConfig,
			imageAgentReviewOpenAIClientName: openaiclient.NewClientConfig("test-key", "gpt-vision-test", "https://provider.example.test/v1", 30),
		},
		DefaultClient: imageAgentOpenAIClientName,
	})
	require.NoError(t, err)
	capabilities, err := buildProductionImageCapabilities(imageCapabilityRuntime{OpenAIManager: manager})
	require.NoError(t, err)

	quote, err := capabilities.UsageQuoter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{
		Operation: "render_scene", InputFingerprint: "slot-fingerprint-v1", MaximumOutputs: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "openai", quote.Provider)
	require.Equal(t, "gpt-image-test", quote.Model)
	require.Equal(t, imageAgentOpenAIClientName, quote.CredentialReference)
	require.NotEmpty(t, quote.ConfigurationVersion)
	require.False(t, quote.CostUpperBoundKnown)
	reviewQuote, err := capabilities.UsageQuoter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{
		Operation: "review", InputFingerprint: "slot-fingerprint-v1", MaximumOutputs: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "gpt-vision-test", reviewQuote.Model)
	require.Equal(t, imageAgentReviewOpenAIClientName, reviewQuote.CredentialReference)

	provider, err := newRoutedOpenAIProductImageProvider(manager)
	require.NoError(t, err)
	quote.ConfigurationVersion = "stale-configuration"
	_, err = provider.adapter(context.Background(), "render_scene", &quote)
	require.ErrorIs(t, err, productimage.ErrCapabilityUnsupported)
}

func completeProviderDependencies() providerDependencies {
	return providerDependencies{
		Subject:         &stubProductImageProvider{quote: productimage.UsageQuote{Operation: "extract_subject", Fingerprint: "subject-v1"}},
		WhiteBackground: &stubProductImageProvider{quote: productimage.UsageQuote{Operation: "render_white_background", Fingerprint: "white-v1"}},
		Scene:           &stubProductImageProvider{quote: productimage.UsageQuote{Operation: "render_scene", Fingerprint: "scene-v1"}},
		Review:          &stubProductImageProvider{quote: productimage.UsageQuote{Operation: "review", Fingerprint: "review-v1"}},
	}
}

type stubProductImageProvider struct {
	subjectCandidate productimage.Candidate
	quoteRequest     productimage.UsageQuoteRequest
	quote            productimage.UsageQuote
}

func (p *stubProductImageProvider) Extract(context.Context, productimage.ExtractRequest) (productimage.Candidate, error) {
	return p.subjectCandidate, nil
}

func (*stubProductImageProvider) RenderWhiteBackground(context.Context, productimage.RenderRequest) (productimage.Candidate, error) {
	return productimage.Candidate{}, nil
}

func (*stubProductImageProvider) RenderScene(context.Context, productimage.SceneRequest) ([]productimage.Candidate, error) {
	return nil, nil
}

func (*stubProductImageProvider) Review(context.Context, productimage.ReviewRequest) (productimage.Review, error) {
	return productimage.Review{}, nil
}

func (p *stubProductImageProvider) QuoteUsage(_ context.Context, request productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	p.quoteRequest = request
	return p.quote, nil
}

type stubProfileResolver struct {
	input   imagepolicy.ProfileInput
	profile imagepolicy.ProductImageProfile
}

func (r *stubProfileResolver) Resolve(input imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error) {
	r.input = input
	return r.profile, nil
}
