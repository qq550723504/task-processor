package listingkit

import (
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestProcessStudioDesignImageRemovalPersistsSourceThenFinal(t *testing.T) {
	alphaPNG := studioTestAlphaPNG(t)
	var uploaded []ImageUploadInput
	remover := &studioTestBackgroundRemover{result: &StudioBackgroundRemovalResult{
		Data:        alphaPNG,
		ContentType: "image/png",
		Model:       "rmbg-model",
		RequestID:   "remove-1",
	}}
	service := &taskStudioMediaService{
		backgroundRemover: remover,
		uploadImages: func(_ context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
			uploaded = append(uploaded, req.Files...)
			return &UploadImagesResponse{ImageURLs: []string{
				"https://cdn.example.test/" + req.Files[0].Filename,
			}}, nil
		},
	}

	processed, err := service.processStudioDesignImage(context.Background(), &StudioDesignRequest{
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
	}, &AIImageResponse{Data: []AIImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("source"))}}}, "studio-design-1.png")
	if err != nil {
		t.Fatalf("processStudioDesignImage() error = %v", err)
	}
	if len(uploaded) != 2 || string(uploaded[0].Data) != "source" || uploaded[0].Filename != "studio-design-1-original.png" {
		t.Fatalf("uploads = %#v, want source followed by original upload", uploaded)
	}
	if remover.calls != 1 || string(remover.input) != "source" {
		t.Fatalf("remover calls/input = %d/%q, want 1/source", remover.calls, remover.input)
	}
	if processed.OriginalImageURL == "" || processed.ImageURL == processed.OriginalImageURL {
		t.Fatalf("processed URLs = %#v, want distinct source and final URLs", processed)
	}
	if processed.BackgroundRemovalStatus != StudioBackgroundRemovalStatusSucceeded || processed.BackgroundRemovalModel != "rmbg-model" {
		t.Fatalf("processed removal metadata = %#v", processed)
	}
}

func TestProcessStudioDesignImageRemovalFailureKeepsSource(t *testing.T) {
	service := &taskStudioMediaService{
		backgroundRemover: &studioTestBackgroundRemover{err: errStudioTestRemoval},
		uploadImages: func(_ context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
			return &UploadImagesResponse{ImageURLs: []string{"https://cdn.example.test/" + req.Files[0].Filename}}, nil
		},
	}

	processed, err := service.processStudioDesignImage(context.Background(), &StudioDesignRequest{
		TransparentBackgroundMode: StudioTransparencyModeRemoval,
	}, &AIImageResponse{Data: []AIImageData{{B64JSON: base64.StdEncoding.EncodeToString([]byte("source"))}}}, "studio-design-1.png")
	if err != nil {
		t.Fatalf("processStudioDesignImage() error = %v", err)
	}
	if processed.ImageURL != processed.OriginalImageURL || processed.BackgroundRemovalStatus != StudioBackgroundRemovalStatusFailed {
		t.Fatalf("processed failure = %#v, want source fallback and failed status", processed)
	}
}

func TestValidateStudioTransparentPNG(t *testing.T) {
	if err := validateStudioTransparentPNG(studioTestAlphaPNG(t)); err != nil {
		t.Fatalf("validateStudioTransparentPNG(alpha) error = %v", err)
	}
	if err := validateStudioTransparentPNG([]byte("not-png")); err == nil {
		t.Fatal("validateStudioTransparentPNG(non-png) error = nil")
	}
}

func TestRetryStudioBackgroundRemovalLoadsOriginalAndPersistsPNG(t *testing.T) {
	alphaPNG := studioTestAlphaPNG(t)
	remover := &studioTestBackgroundRemover{result: &StudioBackgroundRemovalResult{Data: alphaPNG, ContentType: "image/png", Model: "rmbg-model"}}
	var uploaded []ImageUploadInput
	service := &taskStudioMediaService{
		backgroundRemover: remover,
		loadUploadedImage: func(_ context.Context, key string) (*UploadedImageFile, error) {
			if key != "original/source.png" {
				t.Fatalf("uploaded source key = %q, want original/source.png", key)
			}
			return &UploadedImageFile{Data: []byte("source"), ContentType: "image/png"}, nil
		},
		uploadImages: func(_ context.Context, req *UploadImagesRequest) (*UploadImagesResponse, error) {
			uploaded = append(uploaded, req.Files...)
			return &UploadImagesResponse{ImageURLs: []string{"https://cdn.example.test/retry.png"}}, nil
		},
	}
	result, err := service.retryStudioBackgroundRemoval(context.Background(), "/api/v1/listing-kits/uploads/files/original/source.png", "studio-design-1.png")
	if err != nil {
		t.Fatalf("retryStudioBackgroundRemoval() error = %v", err)
	}
	if remover.calls != 1 || string(remover.input) != "source" {
		t.Fatalf("remover calls/input = %d/%q, want 1/source", remover.calls, remover.input)
	}
	if result.ImageURL != "https://cdn.example.test/retry.png" || result.Model != "rmbg-model" {
		t.Fatalf("retry result = %+v, want uploaded URL and model", result)
	}
	if len(uploaded) != 1 || uploaded[0].ContentType != "image/png" {
		t.Fatalf("retry upload = %#v, want one PNG upload", uploaded)
	}
}

type studioTestBackgroundRemover struct {
	result *StudioBackgroundRemovalResult
	err    error
	calls  int
	input  []byte
}

func (s *studioTestBackgroundRemover) Remove(_ context.Context, input []byte, _ string) (*StudioBackgroundRemovalResult, error) {
	s.calls++
	s.input = append([]byte(nil), input...)
	return s.result, s.err
}

var errStudioTestRemoval = &studioTestError{}

type studioTestError struct{}

func (*studioTestError) Error() string { return "removal failed" }

func studioTestAlphaPNG(t *testing.T) []byte {
	t.Helper()
	var output bytesBuffer
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	if err := png.Encode(&output, img); err != nil {
		t.Fatalf("encode alpha PNG: %v", err)
	}
	return output.bytes
}

type bytesBuffer struct{ bytes []byte }

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.bytes = append(b.bytes, p...)
	return len(p), nil
}
