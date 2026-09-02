package httpimage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	productimage "task-processor/internal/product/image"

	"github.com/stretchr/testify/require"
)

func TestProductImageHTTPAdapterImplementsExplicitSubjectAndWhiteBackgroundPorts(t *testing.T) {
	t.Parallel()

	generated := productImageHTTPPNG(t, 3, 4)
	requests := make(chan productImageHTTPRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer secret", request.Header.Get("Authorization"))
		var payload productImageHTTPRequest
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		requests <- payload
		_ = json.NewEncoder(w).Encode(map[string]any{
			"image_base64": base64.StdEncoding.EncodeToString(generated), "format": "png",
		})
	}))
	defer server.Close()
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Subject:         productImageHTTPEndpoint(server.URL+"/subject", "segmenter"),
		WhiteBackground: productImageHTTPEndpoint(server.URL+"/white", "white-background"),
		SourceFetcher:   func(context.Context, string) ([]byte, error) { return []byte("owned-source"), nil },
	})
	require.NoError(t, err)
	subject, err := productimage.NewSubjectCapability(adapter)
	require.NoError(t, err)
	white, err := productimage.NewWhiteBackgroundCapability(adapter)
	require.NoError(t, err)
	source := productImageHTTPSource("source-1", "https://source.example/item.png")

	extracted, err := subject.Extract(context.Background(), productimage.ExtractRequest{Source: source, Product: productImageHTTPContext()})
	require.NoError(t, err)
	whiteResult, err := white.RenderWhiteBackground(context.Background(), productimage.RenderRequest{Source: source, Subject: extracted, Product: productImageHTTPContext()})
	require.NoError(t, err)

	require.Equal(t, productimage.RoleSubject, extracted.Asset.Role)
	require.Equal(t, []string{"extract_subject"}, extracted.Asset.Operations)
	require.Equal(t, productimage.RoleWhiteBackground, whiteResult.Asset.Role)
	require.Equal(t, []string{"render_white_background"}, whiteResult.Asset.Operations)
	require.Equal(t, generated, extracted.Asset.Bytes)
	require.Equal(t, 3, extracted.Asset.Width)
	require.Equal(t, 4, extracted.Asset.Height)
	first, second := <-requests, <-requests
	require.Equal(t, "subject_extract", first.Task)
	require.Equal(t, "white_background", second.Task)
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("owned-source")), first.ImageBase64)
	require.Equal(t, base64.StdEncoding.EncodeToString(generated), second.ImageBase64)
}

func TestProductImageHTTPAdapterSceneSendsStructuredOptionsWithoutCategoryInference(t *testing.T) {
	t.Parallel()

	generated := productImageHTTPPNG(t, 5, 6)
	var captured productImageHTTPRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.NoError(t, json.NewDecoder(request.Body).Decode(&captured))
		_ = json.NewEncoder(w).Encode(map[string]any{"images": []map[string]any{{"image_base64": base64.StdEncoding.EncodeToString(generated), "format": "png"}}})
	}))
	defer server.Close()
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Scene: productImageHTTPEndpoint(server.URL+"/scene", "scene-service"), MaximumSceneOutputs: 3,
		SourceFetcher: func(context.Context, string) ([]byte, error) { return []byte("owned-source"), nil },
	})
	require.NoError(t, err)
	scenes, err := productimage.NewSceneCapability(adapter)
	require.NoError(t, err)
	request := productimage.SceneRequest{
		Source: productImageHTTPSource("source-1", "https://source.example/item.png"), Product: productImageHTTPContext(),
		Options:         productimage.SceneOptions{SceneCategory: "category-a", SceneStyle: "studio", BackgroundTone: "neutral", Composition: "centered", PropsLevel: "none", AudienceHint: "general"},
		StyleReferences: []productimage.Asset{productImageHTTPSource("style-1", "https://style.example/reference.png")}, MaximumOutputs: 2,
	}

	result, err := scenes.RenderScene(context.Background(), request)

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "scene", captured.Task)
	require.Equal(t, productImageHTTPSceneOptions{
		SceneCategory: "category-a", SceneStyle: "studio", BackgroundTone: "neutral", Composition: "centered",
		PropsLevel: "none", AudienceHint: "general",
	}, captured.SceneOptions)
	require.Equal(t, []string{"https://style.example/reference.png"}, captured.StyleReferenceURLs)
	require.Equal(t, 2, captured.MaximumOutputs)
	require.Equal(t, "shoe", captured.Product.ProductType, "product facts may shape rendering but never select policy")
}

func TestProductImageHTTPAdapterRejectsUnknownOrOversizedProviderOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"image_base64":"aW52YWxpZA==","format":"png","unexpected":true}`))
	}))
	defer server.Close()
	adapter, err := NewProductImageAdapter(ProductImageAdapterConfig{
		Subject:       productImageHTTPEndpoint(server.URL, "segmenter"),
		SourceFetcher: func(context.Context, string) ([]byte, error) { return []byte("owned-source"), nil },
	})
	require.NoError(t, err)
	subject, err := productimage.NewSubjectCapability(adapter)
	require.NoError(t, err)

	_, err = subject.Extract(context.Background(), productimage.ExtractRequest{Source: productImageHTTPSource("source-1", "https://source.example/item.png"), Product: productImageHTTPContext()})

	require.ErrorIs(t, err, productimage.ErrOutputValidation)
}

func TestNewProductImageHTTPAdapterRequiresAtLeastOneTypedEndpoint(t *testing.T) {
	t.Parallel()

	_, err := NewProductImageAdapter(ProductImageAdapterConfig{})
	require.Error(t, err)

	endpoint := productImageHTTPEndpoint(" /relative", "segmenter")
	_, err = NewProductImageAdapter(ProductImageAdapterConfig{Subject: endpoint})
	require.Error(t, err)
}

func productImageHTTPEndpoint(endpoint, model string) *ProductImageEndpointConfig {
	return &ProductImageEndpointConfig{
		Endpoint: endpoint, BearerToken: "secret", Provider: "http-image", Model: model,
		RouteReference: "route-a", CredentialReference: "credential-a", ConfigurationVersion: "config-v1", PricingVersion: "pricing-v1",
		CostMicrosPerOutput: 10, CostUpperBoundKnown: true,
	}
}

func productImageHTTPSource(id, url string) productimage.Asset {
	return productimage.Asset{URL: url, MediaType: "image/png", SourceURL: url, SourceAssetID: id, Role: productimage.RoleSource, Width: 1200, Height: 1200, Operations: []string{"ingest"}}
}

func productImageHTTPContext() productimage.ProductContext {
	return productimage.ProductContext{ProductKey: "product-1", Title: "Example", ProductType: "shoe", Attributes: map[string]string{"color": "blue"}}
}

func productImageHTTPPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, canvas))
	return encoded.Bytes()
}
