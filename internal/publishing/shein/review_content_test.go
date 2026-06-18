package shein

import (
	"context"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
)

func TestOptimizePackageReviewContent_UsesAIAndUpdatesPreviewSurface(t *testing.T) {
	pkg := &Package{
		SpuName:          "SPU-123",
		ProductNameEn:    "Envelope style pillow cover",
		ProductNameMulti: "Envelope style pillow cover",
		Description:      "Simple pillow cover for home decor.",
		SellingPoints:    []string{"Soft polyester", "Botanical print"},
		CategoryPath:     []string{"Home", "Textiles", "Pillow Covers"},
		Images:           &common.ImageSet{MainImage: "https://example.com/main.jpg"},
		Metadata:         map[string]string{"language": "en_US"},
		RequestDraft: &RequestDraft{
			SpuName:               "SPU-123",
			MultiLanguageNameList: localizedEnglishText("en_US", "Envelope style pillow cover"),
			MultiLanguageDescList: localizedEnglishText("en_US", "Simple pillow cover for home decor."),
			SKCList: []SKCRequestDraft{{
				SkcName:               "Envelope style pillow cover beige",
				SaleName:              "beige",
				MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Envelope style pillow cover beige"}},
			}},
		},
		SkcList: []SKCPackage{{
			SkcName:  "Envelope style pillow cover beige",
			SaleName: "beige",
		}},
	}
	ai := &recordingChatCompleter{
		response: &openaiclient.ChatCompletionResponse{
			Choices: []openaiclient.ChatCompletionChoice{{
				Message: openaiclient.ChatCompletionMessage{
					Content: `{"title":"Botanical Envelope Pillow Cover for Sofa Couch Bedroom Decor, Soft Polyester Accent Cushion Case","description":"A soft polyester envelope pillow cover designed to add botanical pattern detail to sofas, beds, and reading corners. The clean hidden-overlap closure makes it easy to refresh everyday decor while keeping a neat finished look."}`,
				},
			}},
		},
	}

	if err := OptimizePackageReviewContent(context.Background(), pkg, NewReviewContentOptimizer(ai)); err != nil {
		t.Fatalf("OptimizePackageReviewContent returned error: %v", err)
	}
	if ai.lastUserPrompt == "" {
		t.Fatal("ai user prompt is empty, want multimodal optimization request")
	}
	if len(ai.lastImageURLs) != 1 || ai.lastImageURLs[0] != "https://example.com/main.jpg" {
		t.Fatalf("ai image URLs = %+v, want main image", ai.lastImageURLs)
	}
	if got := pkg.ProductNameEn; !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("pkg title = %q", got)
	}
	if got := pkg.Description; !strings.Contains(got, "reading corners") {
		t.Fatalf("pkg description = %q", got)
	}
	if got := firstLocalizedText(pkg.RequestDraft.MultiLanguageNameList); !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("draft title = %q", got)
	}
	if got := firstLocalizedText(pkg.RequestDraft.MultiLanguageDescList); !strings.Contains(got, "reading corners") {
		t.Fatalf("draft description = %q", got)
	}
	if got := pkg.RequestDraft.SKCList[0].SkcName; !strings.Contains(strings.ToLower(got), "beige") || !strings.Contains(strings.ToLower(got), "botanical envelope pillow cover") {
		t.Fatalf("draft skc name = %q", got)
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product = nil")
	}
	if got := findLocalizedText(pkg.PreviewProduct.MultiLanguageNameList, "en"); !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("preview title = %q", got)
	}
	if got := findLocalizedText(pkg.PreviewProduct.MultiLanguageDescList, "en"); !strings.Contains(got, "reading corners") {
		t.Fatalf("preview description = %q", got)
	}
}

func TestOptimizePackageReviewContent_FallsBackWithoutAIRewrite(t *testing.T) {
	pkg := &Package{
		ProductNameEn: "Door curtain for home decor",
		Description:   "A soft curtain for bedrooms and living rooms.",
		RequestDraft: &RequestDraft{
			MultiLanguageNameList: localizedEnglishText("en_US", "Door curtain for home decor"),
			MultiLanguageDescList: localizedEnglishText("en_US", "A soft curtain for bedrooms and living rooms."),
			SKCList: []SKCRequestDraft{{
				SkcName:               "Door curtain for home decor white",
				SaleName:              "white",
				MultiLanguageNameList: []LocalizedText{{Language: "en", Name: "Door curtain for home decor white"}},
			}},
		},
		SkcList: []SKCPackage{{
			SkcName:  "Door curtain for home decor white",
			SaleName: "white",
		}},
	}

	if err := OptimizePackageReviewContent(context.Background(), pkg, nil); err != nil {
		t.Fatalf("OptimizePackageReviewContent returned error: %v", err)
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product = nil")
	}
	if got := findLocalizedText(pkg.PreviewProduct.MultiLanguageNameList, "en"); got != "Door curtain for home decor - A soft curtain for bedrooms and living rooms" {
		t.Fatalf("preview title = %q", got)
	}
	if got := findLocalizedText(pkg.PreviewProduct.MultiLanguageDescList, "en"); got != "A soft curtain for bedrooms and living rooms." {
		t.Fatalf("preview description = %q", got)
	}
}
