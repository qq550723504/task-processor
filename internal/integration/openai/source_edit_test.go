package openai

import (
	"context"
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"task-processor/internal/ai"
	productimage "task-processor/internal/product/image"
	"testing"
)

func TestSourceEditAdapterSendsExactBundleAndApprovedPromptOnce(t *testing.T) {
	images := &productImageGeneratorStub{response: &ai.ImageResponse{Data: []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(productImagePNG(t, 1024, 1024))}}}}
	config := validProductImageAdapterConfig(images, nil)
	config.Provider = "grsai"
	config.ImageModel = "gpt-image-2.5"
	adapter, err := NewProductImageAdapter(config)
	require.NoError(t, err)
	capability, err := productimage.NewSourceEditCapability(adapter)
	require.NoError(t, err)
	quote, err := adapter.QuoteUsage(context.Background(), productimage.UsageQuoteRequest{Operation: productimage.SourceEditOperation, InputFingerprint: "approved-input", MaximumOutputs: 1})
	require.NoError(t, err)
	request := productimage.SourceEditRequest{Sources: []productimage.Asset{productImageSource("one", "https://example.com/one.png"), productImageSource("two", "https://example.com/two.png")}, ReferenceBytes: [][]byte{[]byte("first exact bytes"), []byte("second exact bytes")}, Product: productImageContext(), Prompt: "approved purpose and exact real evidence", PromptVersion: "image-set-prompt-v1", Authorization: &quote}
	candidate, err := capability.EditSources(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, 1, images.editCalls)
	require.Equal(t, request.Prompt, images.lastEdit.Prompt)
	require.Empty(t, images.lastEdit.ImageURL)
	require.Empty(t, images.lastEdit.ImageURLs)
	require.Equal(t, request.ReferenceBytes[0], images.lastEdit.Image)
	require.Equal(t, request.ReferenceBytes[1], images.lastEdit.ReferenceImages[0].Bytes)
	require.NotNil(t, images.lastEdit.MaxRetries)
	require.Zero(t, *images.lastEdit.MaxRetries)
	require.Equal(t, 1, images.lastEdit.N)
	require.Equal(t, request.PromptVersion, candidate.Metadata.PromptVersion)
	require.Equal(t, []string{productimage.SourceEditOperation}, candidate.Asset.Operations)
	request.ReferenceBytes[1][0] = 'X'
	require.Equal(t, []byte("second exact bytes"), images.lastEdit.ReferenceImages[0].Bytes)
	images.response = &ai.ImageResponse{Data: []ai.ImageData{{B64JSON: base64.StdEncoding.EncodeToString(productImagePNG(t, 2, 2))}}}
	request.ReferenceBytes[1] = []byte("second exact bytes")
	_, err = capability.EditSources(context.Background(), request)
	require.ErrorIs(t, err, productimage.ErrOutputValidation, "actual output dimensions must satisfy the configured native renderer")
}
