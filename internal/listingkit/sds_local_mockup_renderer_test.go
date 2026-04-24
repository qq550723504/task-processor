package listingkit

import (
	"image"
	"image/color"
	"testing"
)

func TestCompositeSDSMockupPreservesTransparentSourceAreas(t *testing.T) {
	t.Parallel()

	mockup := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			mockup.Set(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	source := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := 10; y < 30; y++ {
		for x := 10; x < 30; x++ {
			source.Set(x, y, color.NRGBA{R: 200, G: 30, B: 20, A: 255})
		}
	}

	result := compositeSDSMockup(mockup, source)
	if got := color.NRGBAModel.Convert(result.At(20, 20)).(color.NRGBA); got.R != 10 || got.G != 20 || got.B != 30 {
		t.Fatalf("transparent source area changed background: %+v", got)
	}
	if got := color.NRGBAModel.Convert(result.At(50, 50)).(color.NRGBA); got.R < 150 || got.G > 80 || got.B > 80 {
		t.Fatalf("opaque source area was not composited: %+v", got)
	}
}

func TestLocalSDSMockupBaseURLsPrefersBlankDesignForMainImage(t *testing.T) {
	t.Parallel()

	urls := localSDSMockupBaseURLs(localSDSMockupRenderInput{
		BlankDesignURL:  "https://cdn.sdspod.com/blank.jpg",
		MockupImageURLs: []string{"https://cdn.sdspod.com/old-main.jpg", "https://cdn.sdspod.com/scene.jpg"},
	})

	if len(urls) != 2 || urls[0] != "https://cdn.sdspod.com/blank.jpg" || urls[1] != "https://cdn.sdspod.com/scene.jpg" {
		t.Fatalf("unexpected mockup base urls: %+v", urls)
	}
}
