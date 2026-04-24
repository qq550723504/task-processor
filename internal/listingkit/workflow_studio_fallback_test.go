package listingkit

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sdsdesign "task-processor/internal/sds/design"
)

func TestBuildStudioFallbackCanonicalProductUsesSDSMetadata(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/mockup-1.png"},
			Text:      "botanical cushion print",
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					VariantID:           212097,
					ParentProductID:     212096,
					ProductName:         "Custom Pillow Cover",
					ProductSKU:          "NS212096",
					CategoryPath:        []string{"Home", "Decor", "Cushions"},
					Material:            "Polyester",
					MaterialDescription: "Soft polyester fabric",
					ProductionProcess:   "Heat transfer",
					ProductPerformance:  "Comfortable printed pillow cover for home decor.",
					VariantSKU:          "NS212096001",
					VariantSize:         "10x10inch",
					VariantColor:        "White",
					VariantPrice:        12.8,
					VariantWeight:       180,
					ProductionCycle:     48,
				},
			},
		},
	}

	canonical := buildStudioFallbackCanonicalProduct(task)
	if canonical == nil {
		t.Fatal("canonical = nil")
	}
	if canonical.Title != "Custom Pillow Cover" {
		t.Fatalf("title = %q", canonical.Title)
	}
	if got := canonical.CategoryPath; len(got) != 3 || got[2] != "Cushions" {
		t.Fatalf("category path = %+v", got)
	}
	if canonical.Attributes["material"].Value != "Polyester" {
		t.Fatalf("material attribute = %+v", canonical.Attributes["material"])
	}
	if canonical.Specifications == nil || canonical.Specifications.Weight == nil || canonical.Specifications.Weight.Value != 180 {
		t.Fatalf("specifications = %+v", canonical.Specifications)
	}
	if len(canonical.Variants) != 1 {
		t.Fatalf("variants = %d", len(canonical.Variants))
	}
	variant := canonical.Variants[0]
	if variant.SKU != "NS212096001" {
		t.Fatalf("variant SKU = %q", variant.SKU)
	}
	if variant.Attributes["Size"].Value != "10x10inch" || variant.Attributes["Color"].Value != "White" {
		t.Fatalf("variant attributes = %+v", variant.Attributes)
	}
	if variant.Price == nil || variant.Price.Amount != 12.8 {
		t.Fatalf("variant price = %+v", variant.Price)
	}
}

func TestApplySDSSyncMetadataToCanonicalOverridesStaleStudioTitle(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/design.png"},
			Text:      "cute dog transparent print",
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					VariantID:   212097,
					ProductName: "Custom Pillow Cover",
				},
			},
		},
	}
	canonical := buildStudioFallbackCanonicalProduct(task)
	if canonical == nil {
		t.Fatal("canonical = nil")
	}
	if canonical.Title != "Custom Pillow Cover" {
		t.Fatalf("precondition title = %q", canonical.Title)
	}

	summary := buildSDSSyncSummary(task.Request.Options.SDS, &sdsdesign.PrepareSyncDesignResult{
		Page: &sdsdesign.DesignProductPage{
			Product: sdsdesign.DesignProduct{
				ID:        212097,
				Name:      "带刻度方形挂钟25*25（美国直发不含物流）（平台线上物流专用）",
				SKU:       "MG17701062001",
				ParentSKU: "MG17701062",
			},
		},
	})
	changed := applySDSSyncMetadataToCanonical(canonical, summary, task.Request.Options.SDS)

	if !changed {
		t.Fatal("changed = false")
	}
	if canonical.Title != "带刻度方形挂钟25*25（美国直发不含物流）（平台线上物流专用）" {
		t.Fatalf("title = %q", canonical.Title)
	}
	if summary.ProductName != canonical.Title {
		t.Fatalf("summary product name = %q", summary.ProductName)
	}
	if summary.ProductSKU != "MG17701062" || summary.VariantSKU != "MG17701062001" {
		t.Fatalf("summary sku = %q variant = %q", summary.ProductSKU, summary.VariantSKU)
	}
}

func TestApplySDSTemplateImagesToSheinReplacesFlatDesignImages(t *testing.T) {
	pkg := &sheinpub.Package{
		Images: sheinImageSet("https://cdn.example.com/flat-design.png"),
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet("https://cdn.example.com/flat-design.png")),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					ImageInfo: sheinpub.BuildImageDraft(sheinImageSet("https://cdn.example.com/flat-design.png")),
					SKUList: []sheinpub.SKUDraft{
						{MainImage: "https://cdn.example.com/flat-design.png"},
					},
				},
			},
		},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		MockupImageURLs: []string{
			"https://e.sdspod.com/rendered-main.jpg",
			"https://e.sdspod.com/rendered-gallery-1.jpg",
			"https://e.sdspod.com/rendered-gallery-2.jpg",
		},
	}, []string{"https://cdn.example.com/flat-design.png"})

	if pkg.Images.MainImage != "https://e.sdspod.com/rendered-main.jpg" {
		t.Fatalf("main image = %q", pkg.Images.MainImage)
	}
	if len(pkg.Images.Gallery) != 2 || pkg.Images.Gallery[1] != "https://e.sdspod.com/rendered-gallery-2.jpg" {
		t.Fatalf("gallery = %+v", pkg.Images.Gallery)
	}
	if pkg.RequestDraft.ImageInfo.MainImage != "https://e.sdspod.com/rendered-main.jpg" {
		t.Fatalf("draft image = %+v", pkg.RequestDraft.ImageInfo)
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].MainImage != "https://e.sdspod.com/rendered-main.jpg" {
		t.Fatalf("sku main image = %q", pkg.RequestDraft.SKCList[0].SKUList[0].MainImage)
	}
}

func TestApplySDSTemplateImagesToSheinUsesRenderedMockupsAcrossSheinPayload(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	rendered := []string{
		"https://cdn.sdspod.com/out/0/202604/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-1.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-2.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-3.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-4.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-5.jpg",
		"https://cdn.sdspod.com/out/36811/202604/rendered-gallery-6.jpg",
	}
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
					SKUList: []sheinpub.SKUDraft{
						{MainImage: sourceImage},
					},
				},
			},
		},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		Status:          "completed",
		MockupImageURLs: rendered,
	}, []string{sourceImage})

	if pkg.Images.MainImage != rendered[0] {
		t.Fatalf("main image = %q, want rendered main", pkg.Images.MainImage)
	}
	if len(pkg.Images.Gallery) != 6 {
		t.Fatalf("gallery count = %d, want 6", len(pkg.Images.Gallery))
	}
	if pkg.Images.Gallery[5] != rendered[6] {
		t.Fatalf("last gallery = %q, want %q", pkg.Images.Gallery[5], rendered[6])
	}
	if len(pkg.RequestDraft.ImageInfo.Gallery) != 6 || pkg.RequestDraft.ImageInfo.MainImage != rendered[0] {
		t.Fatalf("request draft image info = %+v", pkg.RequestDraft.ImageInfo)
	}
	if len(pkg.RequestDraft.ImageInfo.Source) != 1 || pkg.RequestDraft.ImageInfo.Source[0] != sourceImage {
		t.Fatalf("request draft source images = %+v", pkg.RequestDraft.ImageInfo.Source)
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != rendered[0] {
		t.Fatalf("request draft skc image info = %+v", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].MainImage != rendered[0] {
		t.Fatalf("request draft sku main image = %q", pkg.RequestDraft.SKCList[0].SKUList[0].MainImage)
	}
	if pkg.PreviewProduct == nil || pkg.PreviewProduct.ImageInfo == nil {
		t.Fatalf("preview product missing image info: %+v", pkg.PreviewProduct)
	}
	if got := len(pkg.PreviewProduct.ImageInfo.ImageInfoList); got != 7 {
		t.Fatalf("preview product image count = %d, want 7", got)
	}
	if pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL != rendered[0] {
		t.Fatalf("preview product main image = %q, want %q", pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL, rendered[0])
	}
	if pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageType != 1 || !pkg.PreviewProduct.ImageInfo.ImageInfoList[0].MarketingMainImage {
		t.Fatalf("preview product main image metadata = %+v", pkg.PreviewProduct.ImageInfo.ImageInfoList[0])
	}
	if len(pkg.PreviewProduct.SKCList) != 1 {
		t.Fatalf("preview skc count = %d, want 1", len(pkg.PreviewProduct.SKCList))
	}
	if got := len(pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList); got != 7 {
		t.Fatalf("preview skc image count = %d, want 7", got)
	}
	if got := pkg.PreviewProduct.SKCList[0].SKUS[0].ImageInfo.ImageInfoList[0].ImageURL; got != rendered[0] {
		t.Fatalf("preview sku main image = %q, want %q", got, rendered[0])
	}
	if pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL == sourceImage {
		t.Fatalf("preview product still uses source image: %q", sourceImage)
	}
}

func TestApplySDSTemplateImagesToSheinSkipsWithoutRenderedMockups(t *testing.T) {
	pkg := &sheinpub.Package{
		Images:       sheinImageSet("https://cdn.example.com/flat-design.png"),
		RequestDraft: &sheinpub.RequestDraft{ImageInfo: sheinpub.BuildImageDraft(sheinImageSet("https://cdn.example.com/flat-design.png"))},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		Status: "render_unavailable",
	}, []string{"https://cdn.example.com/flat-design.png"})

	if pkg.Images.MainImage != "https://cdn.example.com/flat-design.png" {
		t.Fatalf("main image = %q", pkg.Images.MainImage)
	}
	if pkg.RequestDraft.ImageInfo.MainImage != "https://cdn.example.com/flat-design.png" {
		t.Fatalf("draft image = %+v", pkg.RequestDraft.ImageInfo)
	}
}

func sheinImageSet(main string) *common.ImageSet {
	return &common.ImageSet{MainImage: main}
}
