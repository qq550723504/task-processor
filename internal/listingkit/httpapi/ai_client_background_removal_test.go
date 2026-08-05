package httpapi

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func TestListingKitBackgroundRemoverPassesImageBytesToConfiguredClient(t *testing.T) {
	generator := &backgroundRemovalImageGenerator{
		response: &openaiclient.ImageResponse{
			Data:      []openaiclient.ImageData{{B64JSON: "cG5n"}},
			RequestID: "request-1",
		},
	}
	remover := adaptListingKitBackgroundRemover(generator)

	result, err := remover.Remove(context.Background(), []byte("source"), "image/webp")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if string(generator.editReq.Image) != "source" || generator.editReq.ImageContentType != "image/webp" {
		t.Fatalf("edit request image = %#v/%q", generator.editReq.Image, generator.editReq.ImageContentType)
	}
	if generator.editReq.N != 1 || generator.editReq.ResponseFormat != "b64_json" {
		t.Fatalf("edit request format = %#v", generator.editReq)
	}
	if string(result.Data) != "png" || result.RequestID != "request-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestListingKitBackgroundRemoverRejectsEmptyProviderResponse(t *testing.T) {
	remover := adaptListingKitBackgroundRemover(&backgroundRemovalImageGenerator{})
	if _, err := remover.Remove(context.Background(), []byte("source"), "image/png"); err == nil {
		t.Fatal("Remove() error = nil, want missing image data error")
	}
}

func TestListingKitRoutedImageClientResolvesBackgroundRemovalSelector(t *testing.T) {
	removal := &backgroundRemovalImageGenerator{}
	client := &listingKitRoutedImageClient{
		defaultImage:      &backgroundRemovalImageGenerator{},
		backgroundRemoval: removal,
	}

	got, configured, err := client.resolveBySelector("background-removal")
	if err != nil {
		t.Fatalf("resolveBySelector() error = %v", err)
	}
	if got != removal || !configured {
		t.Fatalf("resolveBySelector() = %v/%v, want configured removal client", got, configured)
	}
}

type backgroundRemovalImageGenerator struct {
	editReq  *openaiclient.ImageEditRequest
	response *openaiclient.ImageResponse
}

func (*backgroundRemovalImageGenerator) GenerateImage(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageResponse, error) {
	return nil, nil
}

func (s *backgroundRemovalImageGenerator) EditImage(_ context.Context, req *openaiclient.ImageEditRequest) (*openaiclient.ImageResponse, error) {
	s.editReq = req
	return s.response, nil
}

func (*backgroundRemovalImageGenerator) GetDefaultModel() string { return "removal-model" }

func (*backgroundRemovalImageGenerator) SupportsAsyncImageGeneration() bool { return false }

func (*backgroundRemovalImageGenerator) SubmitImageGeneration(context.Context, *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (*backgroundRemovalImageGenerator) SubmitImageEdit(context.Context, *openaiclient.ImageEditRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}

func (*backgroundRemovalImageGenerator) QueryImageGeneration(context.Context, string) (*openaiclient.ImageAsyncQueryResponse, error) {
	return nil, openaiclient.ErrAsyncImageGenerationNotSupported
}
