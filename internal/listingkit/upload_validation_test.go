package listingkit

import (
	"bytes"
	"encoding/base64"
	"image"
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

func TestValidateUploadedImageAcceptsWebP(t *testing.T) {
	t.Parallel()
	data := validWebPData(t)
	if _, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		t.Fatalf("image.DecodeConfig() error = %v", err)
	} else if format != "webp" {
		t.Fatalf("image.DecodeConfig() format = %q, want webp", format)
	}

	validated, err := validateUploadedImage(ImageUploadInput{Data: data})
	if err != nil {
		t.Fatalf("validateUploadedImage() error = %v", err)
	}
	if validated.ContentType != "image/webp" || validated.Extension != ".webp" {
		t.Fatalf("validated = %+v, want image/webp and .webp", validated)
	}
}

func validWebPData(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
