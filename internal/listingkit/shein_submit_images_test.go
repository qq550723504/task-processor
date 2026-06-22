package listingkit

import (
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestUploadSheinImageInfoGeneratesColorBlockForSwatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 20, 20))
		for y := 0; y < 20; y++ {
			for x := 0; x < 20; x++ {
				img.Set(x, y, color.RGBA{R: 12, G: 34, B: 56, A: 255})
			}
		}
		w.Header().Set("Content-Type", "image/jpeg")
		if err := jpeg.Encode(w, img, nil); err != nil {
			t.Fatalf("encode test image: %v", err)
		}
	}))
	defer server.Close()

	info := &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{{
			ImageURL:  server.URL + "/source.jpg",
			ImageType: 6,
		}},
	}
	api := &stubSheinImageAPI{originalUpload: "https://img.shein.com/uploaded/swatch.jpg"}
	count, err := sheinpub.UploadImageInfo(info, api, map[string]string{}, buildSheinColorBlockImageFromURL)
	if err != nil {
		t.Fatalf("UploadImageInfo: %v", err)
	}
	if count != 1 {
		t.Fatalf("upload count = %d, want 1", count)
	}
	if api.originalCalls != 1 {
		t.Fatalf("original upload calls = %d, want 1", api.originalCalls)
	}
	if got := info.ImageInfoList[0].ImageURL; got != api.originalUpload {
		t.Fatalf("image url = %q, want generated color block URL", got)
	}
}

func TestUploadSheinProductImagesDeduplicatesSharedURLs(t *testing.T) {
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
	api := &stubSheinImageAPI{
		uploaded: map[string]string{
			sourceURL: uploadedURL,
		},
	}

	count, cache, err := uploadSheinProductImages(product, api, nil)
	if err != nil {
		t.Fatalf("uploadSheinProductImages: %v", err)
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
		t.Fatalf("spu image url = %q, want %q", got, uploadedURL)
	}
	if got := product.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploadedURL {
		t.Fatalf("skc image url = %q, want %q", got, uploadedURL)
	}
	if got := product.SKCList[0].SKUS[0].ImageInfo.ImageInfoList[0].ImageURL; got != uploadedURL {
		t.Fatalf("sku image url = %q, want %q", got, uploadedURL)
	}
}
