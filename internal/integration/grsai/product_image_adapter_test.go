package grsai

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"task-processor/internal/ai"
	openaiprovider "task-processor/internal/integration/openai"
	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestNewProductImageAdapterPinsGRSAIClientToExactRoute(t *testing.T) {
	t.Parallel()

	upstream := &grsaiProductImageGeneratorStub{response: &ai.ImageResponse{Data: []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(grsaiProductImagePNG(t))}}}}
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Client: upstream,
		Adapter: openaiprovider.ProductImageAdapterConfig{
			ReviewClient: &grsaiProductImageChatStub{}, Provider: "grsai", ImageModel: "nano-banana", ReviewModel: "gpt-5-mini",
			RouteReference: "route-a", CredentialReference: "credential-a", ConfigurationVersion: "config-v1", PricingVersion: "pricing-v1",
			Prompts: openaiprovider.ProductImagePrompts{
				Subject: "isolate product", WhiteBackground: "place product on white", Scene: "render product scene", Review: "review image", Version: "prompts-v1",
			},
			MaximumSceneOutputs: 2, ImageCostMicrosPerOutput: 10, ReviewCostMicros: 5, CostUpperBoundKnown: true,
		},
	})
	require.NoError(t, err)
	subject, err := productimage.NewSubjectCapability(adapter)
	require.NoError(t, err)

	_, err = subject.Extract(context.Background(), productimage.ExtractRequest{
		Source:  productimage.Asset{URL: "https://source.example/item.png", MediaType: "image/png", SourceURL: "https://source.example/item.png", SourceAssetID: "source-1", Role: productimage.RoleSource, Width: 10, Height: 10, Operations: []string{"ingest"}},
		Product: productimage.ProductContext{ProductKey: "product-1"},
	})

	require.NoError(t, err)
	require.Equal(t, 1, upstream.editCalls)
	require.Equal(t, "nano-banana", upstream.lastEdit.Model)
}

func TestPinnedProductImageClientRejectsRouteDriftBeforeGRSAIDispatch(t *testing.T) {
	t.Parallel()

	upstream := &grsaiProductImageGeneratorStub{}
	pinned, err := newPinnedProductImageClient(upstream, "nano-banana", "credential-a", "config-v1")
	require.NoError(t, err)

	_, err = pinned.EditImageWithRoute(context.Background(), &ai.ImageEditRequest{Model: "nano-banana"}, openaiprovider.ImageRouteSelection{CredentialReference: "credential-b", ConfigurationVersion: "config-v1"})

	require.Error(t, err)
	require.Zero(t, upstream.editCalls)
}

func TestNewProductImageAdapterRejectsCompetingImageClient(t *testing.T) {
	t.Parallel()

	upstream := &grsaiProductImageGeneratorStub{}
	_, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Client:  upstream,
		Adapter: openaiprovider.ProductImageAdapterConfig{ImageClient: upstream},
	})
	require.Error(t, err)
}

type grsaiProductImageGeneratorStub struct {
	response  *ai.ImageResponse
	lastEdit  *ai.ImageEditRequest
	editCalls int
}

func (*grsaiProductImageGeneratorStub) GenerateImage(context.Context, *ai.ImageGenerateRequest) (*ai.ImageResponse, error) {
	return nil, nil
}
func (s *grsaiProductImageGeneratorStub) EditImage(_ context.Context, request *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	s.editCalls++
	cloned := *request
	s.lastEdit = &cloned
	return s.response, nil
}
func (*grsaiProductImageGeneratorStub) GetDefaultModel() string            { return "nano-banana" }
func (*grsaiProductImageGeneratorStub) SupportsAsyncImageGeneration() bool { return true }
func (*grsaiProductImageGeneratorStub) SubmitImageGeneration(context.Context, *ai.ImageGenerateRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, nil
}
func (*grsaiProductImageGeneratorStub) SubmitImageEdit(context.Context, *ai.ImageEditRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, nil
}
func (*grsaiProductImageGeneratorStub) QueryImageGeneration(context.Context, string) (*ai.ImageAsyncQueryResponse, error) {
	return nil, nil
}

type grsaiProductImageChatStub struct{}

func (*grsaiProductImageChatStub) CreateChatCompletion(context.Context, *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
	return nil, nil
}
func (*grsaiProductImageChatStub) Generate(context.Context, string) (string, error) { return "", nil }
func (*grsaiProductImageChatStub) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}
func (*grsaiProductImageChatStub) GetDefaultModel() string { return "gpt-5-mini" }

func grsaiProductImagePNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 2, 2))
	canvas.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, canvas))
	return encoded.Bytes()
}
