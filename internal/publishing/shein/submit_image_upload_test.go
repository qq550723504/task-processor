package shein

import (
	"fmt"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestUploadImageInfoUsesColorBlockBuilderForSwatch(t *testing.T) {
	t.Parallel()

	info := &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{{
			ImageURL:  "https://cdn.example.com/swatch.jpg",
			ImageType: 6,
		}},
	}
	api := &stubSubmitImageAPI{originalUpload: "https://img.shein.com/uploaded/swatch.jpg"}
	builderCalls := 0

	count, err := UploadImageInfo(info, api, map[string]string{}, func(imageURL string) ([]byte, error) {
		builderCalls++
		if imageURL != "https://cdn.example.com/swatch.jpg" {
			t.Fatalf("builder image URL = %q, want source URL", imageURL)
		}
		return []byte("image-bytes"), nil
	})
	if err != nil {
		t.Fatalf("UploadImageInfo: %v", err)
	}
	if count != 1 || builderCalls != 1 || api.originalCalls != 1 {
		t.Fatalf("count/builder/original = %d/%d/%d, want 1/1/1", count, builderCalls, api.originalCalls)
	}
	if got := info.ImageInfoList[0].ImageURL; got != api.originalUpload {
		t.Fatalf("image URL = %q, want %q", got, api.originalUpload)
	}
}

func TestUploadProductImagesDeduplicatesSharedURLs(t *testing.T) {
	t.Parallel()

	sourceURL := "https://cdn.example.com/shared.jpg"
	uploadedURL := "https://img.shein.com/uploaded/shared.jpg"
	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: sourceURL}},
		},
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: sourceURL}},
			},
			SKUS: []sheinproduct.SKU{{
				ImageInfo: &sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: sourceURL}},
				},
			}},
		}},
	}
	api := &stubSubmitImageAPI{
		uploaded: map[string]string{
			sourceURL: uploadedURL,
		},
	}

	count, cache, err := UploadProductImages(product, api, nil, nil)
	if err != nil {
		t.Fatalf("UploadProductImages: %v", err)
	}
	if count != 1 {
		t.Fatalf("upload count = %d, want 1 unique upload", count)
	}
	if got := api.calls[sourceURL]; got != 1 {
		t.Fatalf("upload calls = %d, want 1", got)
	}
	if cache[sourceURL] != uploadedURL {
		t.Fatalf("cache[%q] = %q, want %q", sourceURL, cache[sourceURL], uploadedURL)
	}
	if got := product.ImageInfo.ImageInfoList[0].ImageURL; got != uploadedURL {
		t.Fatalf("spu image URL = %q, want %q", got, uploadedURL)
	}
	if got := product.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploadedURL {
		t.Fatalf("skc image URL = %q, want %q", got, uploadedURL)
	}
	if got := product.SKCList[0].SKUS[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploadedURL {
		t.Fatalf("sku image URL = %q, want %q", got, uploadedURL)
	}
}

type stubSubmitImageAPI struct {
	uploaded       map[string]string
	calls          map[string]int
	originalUpload string
	originalCalls  int
}

func (s *stubSubmitImageAPI) UploadOriginalImage([]byte) (string, error) {
	s.originalCalls++
	if s.originalUpload == "" {
		return "", fmt.Errorf("missing original upload URL")
	}
	return s.originalUpload, nil
}

func (s *stubSubmitImageAPI) DownloadAndUploadImage(imageURL string) (string, error) {
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[imageURL]++
	if uploadedURL := s.uploaded[imageURL]; uploadedURL != "" {
		return uploadedURL, nil
	}
	return "https://img.shein.com/uploaded/default.jpg", nil
}
