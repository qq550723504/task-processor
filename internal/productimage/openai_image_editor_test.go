package productimage

import (
	"context"
	"testing"
)

func TestDecodeFirstEditedImageUsesURLWhenB64JSONIsAbsent(t *testing.T) {
	calledURL := ""
	data, revisedPrompt, err := decodeFirstEditedImage(
		context.Background(),
		&imageEditResponse{Data: []imageEditData{{
			URL:           "https://cdn.example.com/generated.png",
			RevisedPrompt: "revised",
		}}},
		func(_ context.Context, imageURL string) ([]byte, error) {
			calledURL = imageURL
			return []byte("generated-image"), nil
		},
	)
	if err != nil {
		t.Fatalf("decodeFirstEditedImage() error = %v", err)
	}
	if string(data) != "generated-image" {
		t.Fatalf("data = %q, want downloaded image data", data)
	}
	if revisedPrompt != "revised" {
		t.Fatalf("revised prompt = %q, want revised", revisedPrompt)
	}
	if calledURL != "https://cdn.example.com/generated.png" {
		t.Fatalf("download URL = %q, want generated image URL", calledURL)
	}
}
