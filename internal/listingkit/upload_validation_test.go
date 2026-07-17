package listingkit

import (
	"strings"
	"testing"
)

func TestValidateUploadedImageRejectsMismatchedMultipartContentType(t *testing.T) {
	t.Parallel()

	_, err := validateUploadedImage(ImageUploadInput{
		Filename:    "photo.jpg",
		ContentType: "image/jpeg",
		Data:        []byte("not an image"),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid image") {
		t.Fatalf("validateUploadedImage() error = %v, want invalid image", err)
	}
}
