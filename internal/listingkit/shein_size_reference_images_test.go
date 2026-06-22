package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestResolveSheinSizeReferenceImagesUsesRenderedSDSImage(t *testing.T) {
	req := &GenerateRequest{
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{
				SizeReferenceImageURLs: []string{"https://cdn.sdspod.com/raw-size.jpg"},
			},
			SDS: &SDSSyncOptions{
				MockupImageURLs: []string{
					"https://cdn.sdspod.com/raw-main.jpg",
					"https://cdn.sdspod.com/raw-size.jpg",
				},
			},
		},
	}
	summary := &SDSSyncSummary{
		MockupImageURLs: []string{
			"https://cdn.sdspod.com/rendered-main.jpg",
			"https://cdn.sdspod.com/rendered-size.jpg",
		},
	}

	got := resolveSheinSizeReferenceImages(req, summary)
	if len(got) != 1 || got[0] != "https://cdn.sdspod.com/rendered-size.jpg" {
		t.Fatalf("size refs = %+v", got)
	}
}

func TestResolveSheinSizeReferenceImagesFallsBackToRawImage(t *testing.T) {
	req := &GenerateRequest{
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{
				SizeReferenceImageURLs: []string{"https://cdn.sdspod.com/raw-size.jpg"},
			},
			SDS: &SDSSyncOptions{
				MockupImageURLs: []string{"https://cdn.sdspod.com/raw-main.jpg"},
			},
		},
	}

	got := resolveSheinSizeReferenceImages(req, &SDSSyncSummary{})
	if len(got) != 1 || got[0] != "https://cdn.sdspod.com/raw-size.jpg" {
		t.Fatalf("size refs = %+v", got)
	}
}

func TestResolveSheinSizeReferenceImagesUsesVariantRenderedImage(t *testing.T) {
	req := &GenerateRequest{
		Options: &GenerateOptions{
			SDS: &SDSSyncOptions{
				Variants: []SDSSyncVariantOption{
					{
						VariantID: 101,
						MockupImageURLs: []string{
							"https://cdn.sdspod.com/black-main.jpg",
							"https://cdn.sdspod.com/black-size.jpg",
						},
						SizeReferenceImageURLs: []string{"https://cdn.sdspod.com/black-size.jpg"},
					},
				},
			},
		},
	}
	summary := &SDSSyncSummary{
		VariantResults: []SDSSyncSummary{
			{
				VariantID: 101,
				MockupImageURLs: []string{
					"https://cdn.sdspod.com/rendered-black-main.jpg",
					"https://cdn.sdspod.com/rendered-black-size.jpg",
				},
			},
		},
	}

	got := resolveSheinSizeReferenceImages(req, summary)
	if len(got) != 1 || got[0] != "https://cdn.sdspod.com/rendered-black-size.jpg" {
		t.Fatalf("size refs = %+v", got)
	}
}

func TestSheinSizeReferenceImagesCallPublishingRendererDirectly(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("shein_size_reference_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_size_reference_images.go) error = %v", err)
	}
	content := string(src)
	if !strings.Contains(content, "sheinpub.ResolveRenderedSizeReferenceImages(") {
		t.Fatal("shein_size_reference_images.go should call publishing renderer directly")
	}
	if strings.Contains(content, "func renderedSizeReferenceImagesFromMockups(") {
		t.Fatal("shein_size_reference_images.go should not keep rendered size reference wrapper")
	}
}
