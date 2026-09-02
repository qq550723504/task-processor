package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"task-processor/internal/ai"
	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestProductImageAdapterUsesTypedPortsAndKeepsGeneratedArtifactsInline(t *testing.T) {
	t.Parallel()

	generated := productImagePNG(t, 2, 3)
	images := &productImageGeneratorStub{response: &ai.ImageResponse{
		RequestID: "image-request-1",
		Data:      []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(generated)}},
	}}
	adapter, err := NewProductImageAdapter(validProductImageAdapterConfig(images, &productImageChatStub{}))
	require.NoError(t, err)
	subject, err := productimage.NewSubjectCapability(adapter)
	require.NoError(t, err)

	result, err := subject.Extract(context.Background(), productimage.ExtractRequest{Source: productImageSource("source-1", "https://source.example/item.png"), Product: productImageContext()})

	require.NoError(t, err)
	require.Equal(t, "https://source.example/item.png", images.lastEdit.ImageURL)
	require.Nil(t, images.lastEdit.Image)
	require.Equal(t, "image/png", images.lastEdit.ImageContentType)
	require.Equal(t, 1, images.lastEdit.N)
	require.Equal(t, ImageRouteSelection{CredentialReference: "credential-a", ConfigurationVersion: "config-v1"}, images.lastRoute)
	require.Contains(t, images.lastEdit.Prompt, "isolate the exact product")
	require.Contains(t, images.lastEdit.Prompt, `"product_key":"product-1"`)
	require.Empty(t, result.Asset.URL)
	require.Equal(t, generated, result.Asset.Bytes)
	require.NotSame(t, &generated[0], &result.Asset.Bytes[0])
	require.Equal(t, "image/png", result.Asset.MediaType)
	require.Equal(t, 2, result.Asset.Width)
	require.Equal(t, 3, result.Asset.Height)
	require.Equal(t, productimage.RoleSubject, result.Asset.Role)
	require.Equal(t, []string{"extract_subject"}, result.Asset.Operations)
	require.Equal(t, "image-request-1", result.Metadata.InvocationID)
}

func TestProductImageAdapterRendersWhiteBackgroundFromInlineSubjectWithOriginalProvenance(t *testing.T) {
	t.Parallel()

	generated := productImagePNG(t, 3, 4)
	images := &productImageGeneratorStub{response: &ai.ImageResponse{Data: []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(generated)}}}}
	adapter, err := NewProductImageAdapter(validProductImageAdapterConfig(images, &productImageChatStub{}))
	require.NoError(t, err)
	white, err := productimage.NewWhiteBackgroundCapability(adapter)
	require.NoError(t, err)
	source := productImageSource("source-1", "https://source.example/item.png")
	subjectBytes := productImagePNG(t, 2, 2)
	subject := productimage.Candidate{Asset: productimage.Asset{
		Bytes: subjectBytes, MediaType: "image/png", SourceURL: source.URL, SourceAssetID: source.SourceAssetID,
		Role: productimage.RoleSubject, Width: 2, Height: 2, Operations: []string{"extract_subject"},
	}}

	result, err := white.RenderWhiteBackground(context.Background(), productimage.RenderRequest{
		Source: source, Subject: subject, Product: productImageContext(),
	})

	require.NoError(t, err)
	require.Equal(t, subjectBytes, images.lastEdit.Image)
	require.Empty(t, images.lastEdit.ImageURL)
	require.Equal(t, "image/png", images.lastEdit.ImageContentType)
	require.Equal(t, source.URL, result.Asset.SourceURL)
	require.Equal(t, source.SourceAssetID, result.Asset.SourceAssetID)
}

func TestProductImageAdapterSceneUsesOnlyExplicitOptionsAndAuthorizedStyleReferences(t *testing.T) {
	t.Parallel()

	images := &productImageGeneratorStub{response: &ai.ImageResponse{Data: []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(productImagePNG(t, 4, 5))}}}}
	adapter, err := NewProductImageAdapter(validProductImageAdapterConfig(images, &productImageChatStub{}))
	require.NoError(t, err)
	scenes, err := productimage.NewSceneCapability(adapter)
	require.NoError(t, err)
	request := productimage.SceneRequest{
		Source: productImageSource("source-1", "https://source.example/item.png"), Product: productImageContext(),
		Options: productimage.SceneOptions{
			SceneCategory: "category-a", SceneStyle: "studio", BackgroundTone: "neutral", Composition: "centered",
			PropsLevel: "none", AudienceHint: "general", SlotRole: "scene", SlotBrief: "show product clearly",
		},
		StyleReferences: []productimage.Asset{productImageSource("style-1", "https://style.example/reference.png")},
		MaximumOutputs:  1,
	}

	result, err := scenes.RenderScene(context.Background(), request)

	require.NoError(t, err)
	require.Equal(t, []string{"https://style.example/reference.png"}, images.lastEdit.ImageURLs)
	for _, explicit := range []string{"category-a", "studio", "neutral", "centered", "none", "general", "show product clearly"} {
		require.Contains(t, images.lastEdit.Prompt, explicit)
	}
	require.Equal(t, 1, images.lastEdit.N)
	require.Len(t, result, 1)
	require.Equal(t, productimage.RoleScene, result[0].Asset.Role)
	require.Equal(t, []string{"render_scene"}, result[0].Asset.Operations)
}

func TestProductImageAdapterReviewFailsClosedAndReturnsStructuredDecision(t *testing.T) {
	t.Parallel()

	chat := &productImageChatStub{response: &ai.ChatCompletionResponse{Choices: []ai.ChatCompletionChoice{{Message: ai.ChatCompletionMessage{Content: `{"score":0.82,"needs_human_review":true,"reasons":["edge artifact"]}`}}}}}
	adapter, err := NewProductImageAdapter(validProductImageAdapterConfig(&productImageGeneratorStub{}, chat))
	require.NoError(t, err)
	reviewer, err := productimage.NewReviewCapability(adapter)
	require.NoError(t, err)
	request := productimage.ReviewRequest{
		Product: productImageContext(), Sources: []productimage.Asset{productImageSource("source-1", "https://source.example/item.png")},
		Candidates: []productimage.Candidate{{Asset: productimage.Asset{
			Bytes: productImagePNG(t, 2, 2), MediaType: "image/png", SourceURL: "https://source.example/item.png",
			SourceAssetID: "source-1", Role: productimage.RoleScene, Width: 2, Height: 2, Operations: []string{"render_scene"},
		}}},
	}

	result, err := reviewer.Review(context.Background(), request)

	require.NoError(t, err)
	require.Equal(t, productimage.Review{Score: 0.82, NeedsHumanReview: true, Reasons: []string{"edge artifact"}}, result)
	require.Equal(t, "json_object", chat.lastRequest.ResponseFormat)
	require.Len(t, chat.lastRequest.Messages, 1)
	require.Len(t, chat.lastRequest.Messages[0].MultiContent, 3)
	require.True(t, strings.HasPrefix(chat.lastRequest.Messages[0].MultiContent[2].ImageURL.URL, "data:image/png;base64,"))

	chat.response = &ai.ChatCompletionResponse{Choices: []ai.ChatCompletionChoice{{Message: ai.ChatCompletionMessage{Content: "not-json"}}}}
	_, err = reviewer.Review(context.Background(), request)
	require.ErrorIs(t, err, productimage.ErrOutputValidation)
}

func TestProductImageAdapterQuotesExactTypedOperation(t *testing.T) {
	t.Parallel()

	adapter, err := NewProductImageAdapter(validProductImageAdapterConfig(&productImageGeneratorStub{}, &productImageChatStub{}))
	require.NoError(t, err)

	first, err := adapter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{Operation: "render_scene", InputFingerprint: "input-a", MaximumOutputs: 2})
	require.NoError(t, err)
	second, err := adapter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{Operation: "render_scene", InputFingerprint: "input-b", MaximumOutputs: 2})
	require.NoError(t, err)
	require.Equal(t, int64(2), first.MaximumOutputs)
	require.Equal(t, "openai", first.Provider)
	require.Equal(t, int64(1), first.MaximumModelCalls)
	require.Equal(t, int64(200), first.MaximumCostMicros)
	require.True(t, first.CostUpperBoundKnown)
	require.NotEqual(t, first.Fingerprint, second.Fingerprint)

	_, err = adapter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{Operation: "unknown", InputFingerprint: "input-a", MaximumOutputs: 1})
	require.ErrorIs(t, err, productimage.ErrCapabilityUnsupported)
}

func TestNewProductImageAdapterRejectsIncompleteRuntime(t *testing.T) {
	t.Parallel()

	config := validProductImageAdapterConfig(&productImageGeneratorStub{}, &productImageChatStub{})
	config.ImageClient = nil
	_, err := NewProductImageAdapter(config)
	require.Error(t, err)

	config = validProductImageAdapterConfig(&productImageGeneratorStub{}, &productImageChatStub{})
	config.Prompts.Scene = ""
	_, err = NewProductImageAdapter(config)
	require.Error(t, err)

	config = validProductImageAdapterConfig(&Client{}, &productImageChatStub{})
	_, err = NewProductImageAdapter(config)
	require.ErrorContains(t, err, "route pinning")
}

func TestProductImageAdapterBoundsAggregateInlineOutputBeforeCapabilityReturn(t *testing.T) {
	t.Parallel()

	used, err := consumeProductImageArtifactBudget(0, productimage.MaxInlineArtifactBytes)
	require.NoError(t, err)
	used, err = consumeProductImageArtifactBudget(used, productimage.MaxInlineArtifactBytes)
	require.NoError(t, err)
	require.Equal(t, productimage.MaxInlineArtifactAggregateBytes, used)

	_, err = consumeProductImageArtifactBudget(used, 1)
	require.ErrorIs(t, err, productimage.ErrOutputValidation)
}

func validProductImageAdapterConfig(images ai.ImageGenerator, chat ai.ChatCompleter) ProductImageAdapterConfig {
	return ProductImageAdapterConfig{
		ImageClient: images, ReviewClient: chat, Provider: "openai", ImageModel: "gpt-image-1", ReviewModel: "gpt-5-mini",
		RouteReference: "route-a", CredentialReference: "credential-a", ConfigurationVersion: "config-v1", PricingVersion: "pricing-v1",
		Prompts: ProductImagePrompts{
			Subject: "isolate the exact product", WhiteBackground: "place the exact product on white",
			Scene: "create a controlled product scene", Review: "review the generated product images", Version: "prompts-v1",
		},
		MaximumSceneOutputs: 4, ImageCostMicrosPerOutput: 100, ReviewCostMicros: 25, CostUpperBoundKnown: true,
	}
}

func productImageSource(id, url string) productimage.Asset {
	return productimage.Asset{URL: url, MediaType: "image/png", SourceURL: url, SourceAssetID: id, Role: productimage.RoleSource, Width: 1200, Height: 1200, Operations: []string{"ingest"}}
}

func productImageContext() productimage.ProductContext {
	return productimage.ProductContext{ProductKey: "product-1", Title: "Example", ProductType: "shoe", Attributes: map[string]string{"color": "blue"}}
}

func productImagePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, canvas))
	return encoded.Bytes()
}

type productImageGeneratorStub struct {
	response  *ai.ImageResponse
	err       error
	lastEdit  *ai.ImageEditRequest
	lastRoute ImageRouteSelection
}

func (s *productImageGeneratorStub) GenerateImage(context.Context, *ai.ImageGenerateRequest) (*ai.ImageResponse, error) {
	return s.response, s.err
}
func (s *productImageGeneratorStub) EditImage(_ context.Context, request *ai.ImageEditRequest) (*ai.ImageResponse, error) {
	cloned := *request
	cloned.Image = append([]byte(nil), request.Image...)
	cloned.ImageURLs = append([]string(nil), request.ImageURLs...)
	s.lastEdit = &cloned
	return s.response, s.err
}
func (s *productImageGeneratorStub) EditImageWithRoute(ctx context.Context, request *ai.ImageEditRequest, route ImageRouteSelection) (*ai.ImageResponse, error) {
	s.lastRoute = route
	return s.EditImage(ctx, request)
}
func (*productImageGeneratorStub) GetDefaultModel() string            { return "gpt-image-1" }
func (*productImageGeneratorStub) SupportsAsyncImageGeneration() bool { return false }
func (*productImageGeneratorStub) SubmitImageGeneration(context.Context, *ai.ImageGenerateRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}
func (*productImageGeneratorStub) SubmitImageEdit(context.Context, *ai.ImageEditRequest) (*ai.ImageAsyncSubmitResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}
func (*productImageGeneratorStub) QueryImageGeneration(context.Context, string) (*ai.ImageAsyncQueryResponse, error) {
	return nil, ai.ErrAsyncImageGenerationNotSupported
}

type productImageChatStub struct {
	response    *ai.ChatCompletionResponse
	err         error
	lastRequest *ai.ChatCompletionRequest
}

func (s *productImageChatStub) CreateChatCompletion(_ context.Context, request *ai.ChatCompletionRequest) (*ai.ChatCompletionResponse, error) {
	s.lastRequest = request
	return s.response, s.err
}
func (*productImageChatStub) Generate(context.Context, string) (string, error) { return "", nil }
func (*productImageChatStub) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}
func (*productImageChatStub) GetDefaultModel() string { return "gpt-5-mini" }
