package grsai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/ai"
	productimage "task-processor/internal/product/image"
)

func TestNewProductImageAdapterInjectsExactRoutePinnedGRSAIClient(t *testing.T) {
	t.Parallel()

	upstream := &grsaiProductImageGeneratorStub{}
	var pinned ai.RouteBoundImageGenerator
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Client: upstream, ImageModel: "nano-banana", RouteReference: "route-a",
		CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
		Build: func(client ai.RouteBoundImageGenerator) (ProductImageProvider, error) {
			pinned = client
			return &grsaiProductImageProviderStub{quote: productimage.UsageQuote{
				Operation: "extract_subject", RouteReference: "route-a", Model: "nano-banana",
				CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
			}}, nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	require.NotNil(t, pinned)

	_, err = pinned.EditImageWithRoute(context.Background(), &ai.ImageEditRequest{Model: "nano-banana"}, ai.ImageRouteSelection{
		CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, upstream.editCalls)
}

func TestPinnedProductImageClientRejectsRouteDriftBeforeGRSAIDispatch(t *testing.T) {
	t.Parallel()

	upstream := &grsaiProductImageGeneratorStub{}
	pinned, err := newPinnedProductImageClient(upstream, "nano-banana", "credential-a", "config-v1")
	require.NoError(t, err)

	_, err = pinned.EditImageWithRoute(context.Background(), &ai.ImageEditRequest{Model: "nano-banana"}, ai.ImageRouteSelection{
		CredentialReference: "credential-b", ConfigurationVersion: "config-v1",
	})
	require.Error(t, err)
	require.Zero(t, upstream.editCalls)
}

func TestGRSAIProductImageAdapterRejectsQuoteRouteDrift(t *testing.T) {
	t.Parallel()

	delegate := &grsaiProductImageProviderStub{quote: productimage.UsageQuote{
		Operation: "render_scene", RouteReference: "route-b", Model: "nano-banana",
		CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
	}}
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Client: &grsaiProductImageGeneratorStub{}, ImageModel: "nano-banana", RouteReference: "route-a",
		CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
		Build: func(ai.RouteBoundImageGenerator) (ProductImageProvider, error) { return delegate, nil },
	})
	require.NoError(t, err)

	_, err = adapter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{
		Operation: "render_scene", InputFingerprint: "input-v1", MaximumOutputs: 1,
	})
	require.ErrorIs(t, err, productimage.ErrCapabilityUnsupported)
}

func TestNewProductImageAdapterRejectsMissingFactoryAndTypedNilDelegate(t *testing.T) {
	t.Parallel()

	base := ProductImageAdapterConfig{
		Client: &grsaiProductImageGeneratorStub{}, ImageModel: "nano-banana", RouteReference: "route-a",
		CredentialReference: "credential-a", ConfigurationVersion: "config-v1",
	}
	_, err := NewProductImageAdapter(base)
	require.Error(t, err)

	var provider *grsaiProductImageProviderStub
	base.Build = func(ai.RouteBoundImageGenerator) (ProductImageProvider, error) { return provider, nil }
	_, err = NewProductImageAdapter(base)
	require.Error(t, err)
}

type grsaiProductImageGeneratorStub struct{ editCalls int }

func (*grsaiProductImageGeneratorStub) GenerateImage(context.Context, *ai.ImageGenerateRequest) (*ai.ImageResponse, error) {
	return nil, nil
}
func (s *grsaiProductImageGeneratorStub) EditImage(context.Context, *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	s.editCalls++
	return &ai.ImageResponse{}, nil
}
func (*grsaiProductImageGeneratorStub) GetDefaultModel() string            { return "nano-banana" }
func (*grsaiProductImageGeneratorStub) SupportsAsyncImageGeneration() bool { return false }
func (*grsaiProductImageGeneratorStub) SubmitImageGeneration(context.Context, *ai.ImageGenerateRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}
func (*grsaiProductImageGeneratorStub) SubmitImageEdit(context.Context, *ai.ImageEditRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}
func (*grsaiProductImageGeneratorStub) QueryImageGeneration(context.Context, string) (*ai.ImageAsyncQueryResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}

type grsaiProductImageProviderStub struct{ quote productimage.UsageQuote }

func (*grsaiProductImageProviderStub) Extract(context.Context, productimage.ExtractRequest) (productimage.Candidate, error) {
	return productimage.Candidate{}, nil
}
func (*grsaiProductImageProviderStub) RenderWhiteBackground(context.Context, productimage.RenderRequest) (productimage.Candidate, error) {
	return productimage.Candidate{}, nil
}
func (*grsaiProductImageProviderStub) RenderScene(context.Context, productimage.SceneRequest) ([]productimage.Candidate, error) {
	return nil, nil
}
func (*grsaiProductImageProviderStub) Review(context.Context, productimage.ReviewRequest) (productimage.Review, error) {
	return productimage.Review{}, nil
}
func (p *grsaiProductImageProviderStub) QuoteUsage(context.Context, productimage.UsageQuoteRequest) (productimage.UsageQuote, error) {
	return p.quote, nil
}
