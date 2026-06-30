package listingkit

import (
	"strings"
	"testing"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sdsdesign "task-processor/internal/sds/design"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildStudioFallbackCanonicalProductUsesSDSMetadata(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/mockup-1.png"},
			Text:      "botanical cushion print",
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					VariantID:              212097,
					ParentProductID:        212096,
					ProductName:            "Custom Pillow Cover",
					ProductSKU:             "NS212096",
					CategoryPath:           []string{"Home", "Decor", "Cushions"},
					Material:               "Polyester",
					MaterialDescription:    "Soft polyester fabric",
					ProductionProcess:      "Heat transfer",
					ProductPerformance:     "Comfortable printed pillow cover for home decor.",
					ProductSize:            "10x10inch",
					PackagingSpecification: "1 piece per poly bag, 22x16x3cm, 0.2kg",
					SpecialDescription:     "Invisible zipper closure.",
					ApplicableScenarios:    "Living room, bedroom",
					VariantSKU:             "NS212096001",
					VariantSize:            "10x10inch",
					VariantColor:           "White",
					VariantPrice:           12.8,
					VariantWeight:          180,
					ProductionCycle:        48,
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
	if canonical.Attributes["material_description"].Value != "Soft polyester fabric" {
		t.Fatalf("material_description attribute = %+v", canonical.Attributes["material_description"])
	}
	if canonical.Attributes["product_size"].Value != "10x10inch" {
		t.Fatalf("product_size attribute = %+v", canonical.Attributes["product_size"])
	}
	if canonical.Attributes["packaging_specification"].Value == "" {
		t.Fatalf("packaging_specification attribute = %+v", canonical.Attributes["packaging_specification"])
	}
	if canonical.Specifications == nil || canonical.Specifications.Weight == nil || canonical.Specifications.Weight.Value != 180 {
		t.Fatalf("specifications = %+v", canonical.Specifications)
	}
	if canonical.Specifications.Technical["product_size"] != "10x10inch" {
		t.Fatalf("product_size technical spec = %+v", canonical.Specifications.Technical)
	}
	if canonical.Specifications.Technical["packaging_specification"] == "" {
		t.Fatalf("packaging_specification technical spec = %+v", canonical.Specifications.Technical)
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
	if variant.Price == nil || variant.Price.Currency != "CNY" {
		t.Fatalf("variant price currency = %+v, want CNY", variant.Price)
	}
}

func TestBuildStudioFallbackCanonicalProductExpandsSDSVariantsWithStyleSuffix(t *testing.T) {
	task := &Task{
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/design.png"},
			Text:      "cute dog print",
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					ProductName: "Adult T-Shirt",
					ProductSKU:  "NS6001064",
					StyleID:     "a1b2-c3d4-extra",
					Variants: []SDSSyncVariantOption{
						{VariantID: 89764, VariantSKU: "NS6001064001", Size: "S", Color: "Black", Price: 19.8},
						{VariantID: 89765, VariantSKU: "NS6001064002", Size: "M", Color: "Black", Price: 19.8},
						{VariantID: 89772, VariantSKU: "NS6001064009", Size: "S", Color: "White", Price: 19.8},
					},
				},
			},
		},
	}

	canonical := buildStudioFallbackCanonicalProduct(task)
	if canonical == nil {
		t.Fatal("canonical = nil")
	}
	if len(canonical.Variants) != 3 {
		t.Fatalf("variants = %d, want 3", len(canonical.Variants))
	}
	if canonical.Variants[0].SKU != "NS6001064001-A1B2C3D4" {
		t.Fatalf("first sku = %q", canonical.Variants[0].SKU)
	}
	if canonical.Variants[0].Attributes["source_sds_sku"].Value != "NS6001064001" {
		t.Fatalf("source_sds_sku = %+v", canonical.Variants[0].Attributes["source_sds_sku"])
	}
	if canonical.Variants[2].Attributes["Color"].Value != "White" || canonical.Variants[2].Attributes["Size"].Value != "S" {
		t.Fatalf("third attributes = %+v", canonical.Variants[2].Attributes)
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

func TestBuildSDSVariantSyncSummariesCapturesMissingFinishedProductObservation(t *testing.T) {
	options := &SDSSyncOptions{VariantID: 252086}
	summaries := buildSDSVariantSyncSummaries(options, []SDSSyncVariantOption{
		{VariantID: 252087, VariantSKU: "SKU-GRAY", Color: "gray"},
	}, &sdsdesign.PrepareSyncDesignResult{})
	if len(summaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(summaries))
	}
	summary := summaries[0]
	if summary.Status != "render_unavailable" {
		t.Fatalf("status = %q", summary.Status)
	}
	if !strings.Contains(summary.Error, "did not create finished product records") {
		t.Fatalf("error = %q", summary.Error)
	}
}

func TestBuildSDSVariantSyncSummariesDoesNotReusePrimaryFinishedProductObservation(t *testing.T) {
	options := &SDSSyncOptions{VariantID: 101}
	summaries := buildSDSVariantSyncSummaries(options, []SDSSyncVariantOption{
		{VariantID: 101, VariantSKU: "SKU-RED", Color: "red"},
		{VariantID: 102, VariantSKU: "SKU-GREEN", Color: "green"},
	}, &sdsdesign.PrepareSyncDesignResult{
		Page: &sdsdesign.DesignProductPage{
			Product: sdsdesign.DesignProduct{ID: 101},
		},
		Request: &sdsdesign.SyncDesignRequest{
			Prototypes: []sdsdesign.SyncDesignPrototype{
				{Layers: []sdsdesign.SyncDesignLayer{{LayerID: "layer-red"}}},
			},
		},
		RenderedImageURLs: []string{"https://cdn.sdspod.com/out/red.jpg"},
		RenderedImageURLsByProduct: map[int64][]string{
			101: []string{"https://cdn.sdspod.com/out/red.jpg"},
		},
		RenderedImageObservations: map[int64]sdsdesign.RenderedImageObservation{
			101: {
				ProductID:   101,
				Found:       true,
				BuildFinish: true,
				ItemID:      "item-red",
				ImageCount:  1,
			},
		},
	})
	if len(summaries) != 2 {
		t.Fatalf("summaries = %d, want 2", len(summaries))
	}
	if summaries[0].Diagnostics == nil || summaries[0].Diagnostics.FinishedProduct == nil {
		t.Fatalf("primary diagnostics = %+v, want finished product observation", summaries[0].Diagnostics)
	}
	if summaries[1].Diagnostics != nil && summaries[1].Diagnostics.FinishedProduct != nil {
		t.Fatalf("related diagnostics reused primary finished product: %+v", summaries[1].Diagnostics.FinishedProduct)
	}
	if !strings.Contains(summaries[1].Error, "did not create finished product records") {
		t.Fatalf("related error = %q", summaries[1].Error)
	}
}

func TestBuildSDSVariantSyncSummariesPrefersSensitiveWordFailureReason(t *testing.T) {
	options := &SDSSyncOptions{VariantID: 252086}
	summaries := buildSDSVariantSyncSummaries(options, []SDSSyncVariantOption{
		{VariantID: 252086, VariantSKU: "SKU-BLACK", Color: "black"},
	}, &sdsdesign.PrepareSyncDesignResult{
		RenderedImageObservations: map[int64]sdsdesign.RenderedImageObservation{
			252086: {
				ProductID:   252086,
				Found:       true,
				BuildFinish: true,
				ItemID:      "904265725796831232",
				ImageCount:  8,
			},
		},
		RenderedSensitiveWords: map[string][]sdsdesign.SensitiveWordHit{
			"904265725796831232": {
				{
					SensitiveWord: "Mask",
					PositionStrs:  "导出名称",
				},
			},
		},
	})
	if len(summaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(summaries))
	}
	summary := summaries[0]
	if summary.Diagnostics == nil || len(summary.Diagnostics.SensitiveWords) != 1 {
		t.Fatalf("sensitive words = %+v", summary.Diagnostics)
	}
	if summary.Diagnostics.SensitiveWords[0].SensitiveWord != "Mask" {
		t.Fatalf("sensitive word = %+v", summary.Diagnostics.SensitiveWords[0])
	}
	if !strings.Contains(summary.Error, "Mask") {
		t.Fatalf("error = %q", summary.Error)
	}
}

func TestMergeSDSVariantSyncSummariesPrefersAuthFailureReason(t *testing.T) {
	merged := mergeSDSVariantSyncSummaries(&SDSSyncOptions{VariantID: 101}, []SDSSyncSummary{
		{
			VariantID:    101,
			VariantColor: "white",
			Status:       "failed",
			Error:        "sds POST /ps/design/add_and_design auth required with status 400: 用户未登录",
		},
	})

	if merged.Status != "failed" {
		t.Fatalf("status = %q", merged.Status)
	}
	if merged.Error != sdsAuthRequiredMessage {
		t.Fatalf("error = %q", merged.Error)
	}
}

func TestMergeSDSVariantSyncSummariesKeepsVariantFailureReason(t *testing.T) {
	merged := mergeSDSVariantSyncSummaries(&SDSSyncOptions{VariantID: 101}, []SDSSyncSummary{
		{
			VariantID:    101,
			VariantColor: "white",
			Status:       "failed",
			Error:        "SDS template render returned empty result",
		},
	})

	if merged.Status != "failed" {
		t.Fatalf("status = %q", merged.Status)
	}
	if merged.Error != "SDS render failed for selected color variants: white" {
		t.Fatalf("error = %q", merged.Error)
	}
}

func TestMergeSDSVariantSyncSummariesAggregatesSuccessfulVariantImages(t *testing.T) {
	merged := mergeSDSVariantSyncSummaries(&SDSSyncOptions{VariantID: 101}, []SDSSyncSummary{
		{
			VariantID:       101,
			VariantColor:    "black",
			Status:          "completed",
			MockupImageURLs: []string{"https://cdn.sdspod.com/out/black-main.jpg"},
		},
		{
			VariantID:       106,
			VariantColor:    "apricot",
			Status:          "completed",
			MockupImageURLs: []string{"https://cdn.sdspod.com/out/apricot-main.jpg", "https://cdn.sdspod.com/out/apricot-side.jpg"},
		},
	})

	want := []string{
		"https://cdn.sdspod.com/out/black-main.jpg",
		"https://cdn.sdspod.com/out/apricot-main.jpg",
		"https://cdn.sdspod.com/out/apricot-side.jpg",
	}
	if strings.Join(merged.MockupImageURLs, "|") != strings.Join(want, "|") {
		t.Fatalf("merged mockups = %+v, want %+v", merged.MockupImageURLs, want)
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

func TestApplySDSTemplateImagesToSheinUsesColorSpecificRenderedMockups(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	blackImages := []string{"https://cdn.sdspod.com/out/black-main.jpg", "https://cdn.sdspod.com/out/black-2.jpg"}
	whiteImages := []string{"https://cdn.sdspod.com/out/white-main.jpg", "https://cdn.sdspod.com/out/white-2.jpg"}
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		SkcList: []sheinpub.SKCPackage{
			{SkcName: "Black", SupplierCode: "BLACK-STYLE"},
			{SkcName: "White", SupplierCode: "WHITE-STYLE"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{
				{SkcName: "Black", SupplierCode: "BLACK-STYLE", SKUList: []sheinpub.SKUDraft{{SupplierSKU: "BLACK-S"}}},
				{SkcName: "White", SupplierCode: "WHITE-STYLE", SKUList: []sheinpub.SKUDraft{{SupplierSKU: "WHITE-S"}}},
			},
		},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		VariantColor:    "Black",
		MockupImageURLs: blackImages,
		VariantResults: []SDSSyncSummary{
			{VariantColor: "Black", Status: "completed", MockupImageURLs: blackImages},
			{VariantColor: "White", Status: "completed", MockupImageURLs: whiteImages},
		},
	}, []string{sourceImage})

	if pkg.Images.MainImage != blackImages[0] {
		t.Fatalf("spu main image = %q, want black", pkg.Images.MainImage)
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != blackImages[0] {
		t.Fatalf("black skc image = %+v", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[1].ImageInfo.MainImage != whiteImages[0] {
		t.Fatalf("white skc image = %+v", pkg.RequestDraft.SKCList[1].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[1].SKUList[0].MainImage != whiteImages[0] {
		t.Fatalf("white sku main image = %q", pkg.RequestDraft.SKCList[1].SKUList[0].MainImage)
	}
	if pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) != 2 {
		t.Fatalf("preview product = %+v", pkg.PreviewProduct)
	}
	if pkg.PreviewProduct.SKCList[1].ImageInfo.ImageInfoList[0].ImageURL != whiteImages[0] {
		t.Fatalf("preview white skc image = %+v", pkg.PreviewProduct.SKCList[1].ImageInfo)
	}
}

func TestApplySDSTemplateImagesToSheinDoesNotFallbackToOfficialReferencesWhenRenderedMockupsMissing(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	redRendered := []string{"https://cdn.sdspod.com/out/red-main.jpg", "https://cdn.sdspod.com/out/red-2.jpg"}
	greenOfficial := []string{"https://cdn.sds.example.com/green-main.jpg", "https://cdn.sds.example.com/green-2.jpg"}
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		SkcList: []sheinpub.SKCPackage{
			{
				SkcName:    "red",
				SaleName:   "red",
				Attributes: map[string]string{"Color": "red"},
				SKUs:       []common.Variant{{SKU: "MG8089002001"}},
			},
			{
				SkcName:    "green",
				SaleName:   "green",
				Attributes: map[string]string{"Color": "green"},
				SKUs:       []common.Variant{{SKU: "MG8089002002"}},
			},
		},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName:      "red",
					SaleName:     "red",
					SupplierCode: "MG8089002001-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Color",
						Value: "red",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "MG8089002001-One size",
						Attributes:  map[string]string{"Color": "red", "source_sds_sku": "MG8089002001"},
					}},
				},
				{
					SkcName:      "green",
					SaleName:     "green",
					SupplierCode: "MG8089002002-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Color",
						Value: "green",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "MG8089002002-One size",
						Attributes:  map[string]string{"Color": "green", "source_sds_sku": "MG8089002002"},
					}},
				},
			},
		},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		Status:       "failed",
		VariantColor: "red",
		Error:        "SDS render failed for selected color variants: green",
		VariantResults: []SDSSyncSummary{
			{VariantSKU: "MG8089002001", VariantColor: "red", Status: "completed", MockupImageURLs: redRendered},
			{VariantSKU: "MG8089002002", VariantColor: "green", Status: "render_unavailable", Error: "SDS did not return current fused mockup images"},
		},
	}, []string{sourceImage}, &SDSSyncOptions{
		Variants: []SDSSyncVariantOption{
			{VariantSKU: "MG8089002001", Color: "red", MockupImageURLs: []string{"https://cdn.sds.example.com/red-official.jpg"}},
			{VariantSKU: "MG8089002002", Color: "green", MockupImageURLs: greenOfficial},
		},
	})

	if pkg.Images.MainImage != redRendered[0] {
		t.Fatalf("spu main image = %q, want rendered red", pkg.Images.MainImage)
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != redRendered[0] {
		t.Fatalf("red skc image = %+v", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if greenImageInfo := pkg.RequestDraft.SKCList[1].ImageInfo; greenImageInfo != nil {
		if greenImageInfo.MainImage == greenOfficial[0] {
			t.Fatalf("green skc image used official reference fallback: %+v", greenImageInfo)
		}
		if greenImageInfo.MainImage == redRendered[0] {
			t.Fatalf("green skc image used another variant's rendered image: %+v", greenImageInfo)
		}
	}
	if pkg.RequestDraft.SKCList[1].SKUList[0].MainImage == greenOfficial[0] {
		t.Fatalf("green sku main image used official reference fallback: %q", pkg.RequestDraft.SKCList[1].SKUList[0].MainImage)
	}
	if pkg.SkcList[1].MainImageURL == greenOfficial[0] {
		t.Fatalf("green skc package main image used official reference fallback: %q", pkg.SkcList[1].MainImageURL)
	}
	if pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) != 2 {
		t.Fatalf("preview product = %+v", pkg.PreviewProduct)
	}
	if images := pkg.PreviewProduct.SKCList[1].ImageInfo.ImageInfoList; len(images) > 0 {
		if got := images[0].ImageURL; got == greenOfficial[0] || got == redRendered[0] {
			t.Fatalf("preview green skc image used fallback %q", got)
		}
	}
}

func TestApplySDSTemplateImagesToSheinMatchesSKCByVariantSKUWhenPrimarySaleAttributeIsStyle(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	blackImages := []string{"https://cdn.sdspod.com/out/black-style-main.jpg", "https://cdn.sdspod.com/out/black-style-2.jpg"}
	whiteImages := []string{"https://cdn.sdspod.com/out/white-style-main.jpg", "https://cdn.sdspod.com/out/white-style-2.jpg"}
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		SkcList: []sheinpub.SKCPackage{
			{
				SkcName:      "Style A",
				SaleName:     "Style A",
				SupplierCode: "SDS-BLACK-STYLE",
				Attributes:   map[string]string{"Style Type": "Style A"},
				SKUs: []common.Variant{{
					SKU:        "SDS-BLACK-One size",
					Attributes: map[string]string{"source_sds_sku": "SDS-BLACK"},
				}},
			},
			{
				SkcName:      "Style B",
				SaleName:     "Style B",
				SupplierCode: "SDS-WHITE-STYLE",
				Attributes:   map[string]string{"Style Type": "Style B"},
				SKUs: []common.Variant{{
					SKU:        "SDS-WHITE-One size",
					Attributes: map[string]string{"source_sds_sku": "SDS-WHITE"},
				}},
			},
		},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName:      "Style A",
					SaleName:     "Style A",
					SupplierCode: "SDS-BLACK-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Style Type",
						Value: "Style A",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "SDS-BLACK-One size",
						Attributes:  map[string]string{"source_sds_sku": "SDS-BLACK"},
					}},
				},
				{
					SkcName:      "Style B",
					SaleName:     "Style B",
					SupplierCode: "SDS-WHITE-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Style Type",
						Value: "Style B",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "SDS-WHITE-One size",
						Attributes:  map[string]string{"source_sds_sku": "SDS-WHITE"},
					}},
				},
			},
		},
	}

	applySDSTemplateImagesToShein(pkg, &SDSSyncSummary{
		VariantResults: []SDSSyncSummary{
			{VariantSKU: "SDS-BLACK", VariantColor: "Black", Status: "completed", MockupImageURLs: blackImages},
			{VariantSKU: "SDS-WHITE", VariantColor: "White", Status: "completed", MockupImageURLs: whiteImages},
		},
	}, []string{sourceImage})

	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != blackImages[0] {
		t.Fatalf("black style skc image = %+v", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[1].ImageInfo.MainImage != whiteImages[0] {
		t.Fatalf("white style skc image = %+v", pkg.RequestDraft.SKCList[1].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[1].SKUList[0].MainImage != whiteImages[0] {
		t.Fatalf("white style sku image = %q", pkg.RequestDraft.SKCList[1].SKUList[0].MainImage)
	}
	if pkg.SkcList[1].MainImageURL != whiteImages[0] {
		t.Fatalf("white style package skc image = %q", pkg.SkcList[1].MainImageURL)
	}
	if got := pkg.PreviewProduct.SKCList[1].ImageInfo.ImageInfoList[0].ImageURL; got != whiteImages[0] {
		t.Fatalf("preview white style skc image = %q, want %q", got, whiteImages[0])
	}
}

func TestApplySelectedSDSImagesToSheinUsesExplicitSelection(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		SkcList: []sheinpub.SKCPackage{
			{
				SkcName:    "Running Shoes - Black",
				SaleName:   "Running Shoes - Black",
				Attributes: map[string]string{"Color": "Black"},
				SKUs:       []common.Variant{{SKU: "SKU-BLK"}},
			},
		},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName: "Running Shoes - Black",
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "SKU-BLK-S",
						Attributes:  map[string]string{"Color": "Black", "source_sds_sku": "SKU-BLK"},
					}},
				},
			},
		},
	}

	applied := applySelectedSDSImagesToShein(pkg, &GenerateRequest{
		ImageURLs: []string{sourceImage},
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{
				SelectedSDSImages: []SheinStudioSelectedSDSImage{
					{
						ImageURL:   "https://cdn.sdspod.com/out/black-main.jpg",
						Color:      "Black",
						VariantSKU: "SKU-BLK",
					},
					{
						ImageURL:   "https://cdn.sdspod.com/out/black-detail.jpg",
						Color:      "Black",
						VariantSKU: "SKU-BLK",
					},
				},
			},
		},
	}, []string{sourceImage})

	if !applied {
		t.Fatal("applySelectedSDSImagesToShein = false, want true")
	}
	if pkg.Images.MainImage != "https://cdn.sdspod.com/out/black-main.jpg" {
		t.Fatalf("spu main image = %q", pkg.Images.MainImage)
	}
	if len(pkg.Images.Gallery) != 1 || pkg.Images.Gallery[0] != "https://cdn.sdspod.com/out/black-detail.jpg" {
		t.Fatalf("spu gallery = %+v", pkg.Images.Gallery)
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != "https://cdn.sdspod.com/out/black-main.jpg" {
		t.Fatalf("skc image info = %+v", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].MainImage != "https://cdn.sdspod.com/out/black-main.jpg" {
		t.Fatalf("sku main image = %q", pkg.RequestDraft.SKCList[0].SKUList[0].MainImage)
	}
}

func TestApplySDSOfficialImagesToSheinPrefersRenderedVariantMockupsOverSelectedReferences(t *testing.T) {
	sourceImage := "http://127.0.0.1:9100/listingkit-assets/source.png"
	selectedReference := "https://cdn.sdspod.com/reference/green-bandana.jpg"
	rendered := "https://cdn.sdspod.com/out/0/202605/rendered-green-bandana.jpg"
	pkg := &sheinpub.Package{
		Images: sheinImageSet(sourceImage),
		SkcList: []sheinpub.SKCPackage{{
			SkcName:      "dark green",
			SaleName:     "dark green",
			SupplierCode: "MG8089003001-PETBANDA",
			Attributes:   map[string]string{"Color": "dark green"},
			SKUs: []common.Variant{{
				SKU:        "MG8089003001-One size",
				Attributes: map[string]string{"source_sds_sku": "MG8089003001"},
			}},
		}},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet(sourceImage)),
			SKCList: []sheinpub.SKCRequestDraft{{
				SkcName:      "dark green",
				SaleName:     "dark green",
				SupplierCode: "MG8089003001-PETBANDA",
				SaleAttribute: &sheinpub.ResolvedSaleAttribute{
					Name:  "Color",
					Value: "dark green",
				},
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8089003001-One size",
					Attributes:  map[string]string{"Color": "dark green", "source_sds_sku": "MG8089003001"},
				}},
			}},
		},
	}
	req := &GenerateRequest{
		ImageURLs: []string{sourceImage},
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{
				SelectedSDSImages: []SheinStudioSelectedSDSImage{{
					ImageURL:   selectedReference,
					VariantSKU: "MG8089003001",
					Color:      "dark green",
				}},
			},
		},
	}

	applied := applySDSOfficialImagesToShein(pkg, req, &SDSSyncSummary{
		VariantResults: []SDSSyncSummary{{
			VariantSKU:      "MG8089003001",
			VariantColor:    "dark green",
			Status:          "completed",
			MockupImageURLs: []string{rendered},
		}},
	}, &SDSSyncOptions{
		Variants: []SDSSyncVariantOption{{
			VariantSKU:     "MG8089003001",
			Color:          "dark green",
			MockupImageURL: selectedReference,
		}},
	})

	if !applied {
		t.Fatal("applySDSOfficialImagesToShein = false, want true")
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != rendered {
		t.Fatalf("skc image = %q, want rendered SDS output", pkg.RequestDraft.SKCList[0].ImageInfo.MainImage)
	}
	if pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL != rendered {
		t.Fatalf("preview skc image = %q, want rendered SDS output", pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL)
	}
	if pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL == selectedReference {
		t.Fatalf("preview skc image still uses selected SDS reference: %q", selectedReference)
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

func TestShouldRunStudioInlineKeepsAIGeneratedStrategyInline(t *testing.T) {
	req := &GenerateRequest{
		ImageURLs: []string{"https://cdn.example.com/source.png"},
		Platforms: []string{"shein"},
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SDS:           &SDSSyncOptions{VariantID: 212097},
		},
	}

	if !shouldRunStudioInline(req) {
		t.Fatal("shouldRunStudioInline = false, want true for AI generated studio task")
	}
}

func TestShouldRunRemoteSDSDesignSyncAllowsSDSOfficialMultiPlatform(t *testing.T) {
	req := &GenerateRequest{
		ImageURLs: []string{"https://cdn.example.com/source.png"},
		Platforms: []string{"shein", "temu", "amazon"},
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategySDSOfficial,
			SDS:           &SDSSyncOptions{VariantID: 212097},
		},
	}

	if !shouldRunRemoteSDSDesignSync(req) {
		t.Fatal("shouldRunRemoteSDSDesignSync = false, want true for SDS official multi-platform task")
	}
	if shouldRunStudioInline(req) {
		t.Fatal("shouldRunStudioInline = true, want false so multi-platform tasks still use queued workflow")
	}
}

func TestApplySheinStudioAIImagesToSheinReplacesDraftImages(t *testing.T) {
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

	applySheinStudioAIImagesToShein(pkg, &GenerateRequest{
		ImageURLs: []string{"https://cdn.example.com/source-style.png"},
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				SourceDesignURLs: []string{"https://cdn.example.com/source-style.png"},
				ProductImageURLs: []string{
					"https://cdn.example.com/ai-main.png",
					"https://cdn.example.com/ai-gallery-1.png",
				},
				SizeReferenceImageURLs: []string{"https://cdn.sdspod.com/size-chart.jpg"},
			},
		},
	}, nil)

	if pkg.Images.MainImage != "https://cdn.example.com/ai-main.png" {
		t.Fatalf("main image = %q", pkg.Images.MainImage)
	}
	if len(pkg.Images.Gallery) != 2 || pkg.Images.Gallery[1] != "https://cdn.sdspod.com/size-chart.jpg" {
		t.Fatalf("gallery = %+v", pkg.Images.Gallery)
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].MainImage != "https://cdn.example.com/ai-main.png" {
		t.Fatalf("sku main image = %q", pkg.RequestDraft.SKCList[0].SKUList[0].MainImage)
	}
	if pkg.PreviewProduct == nil || pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL != "https://cdn.example.com/ai-main.png" {
		t.Fatalf("preview image = %+v", pkg.PreviewProduct)
	}
	if !hasSizeReferenceImage(pkg.PreviewProduct.ImageInfo.ImageInfoList, "https://cdn.sdspod.com/size-chart.jpg") {
		t.Fatalf("preview product size map not marked: %+v", pkg.PreviewProduct.ImageInfo.ImageInfoList)
	}
	if !hasSizeReferenceImage(pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList, "https://cdn.sdspod.com/size-chart.jpg") {
		t.Fatalf("preview skc size map not marked: %+v", pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList)
	}
	finalImages := sheinworkspace.BuildFinalReviewImages(pkg.RequestDraft, pkg.FinalDraft, pkg.PreviewProduct)
	if !hasFinalReviewSizeMap(finalImages, "https://cdn.sdspod.com/size-chart.jpg") {
		t.Fatalf("final review size map not marked: %+v", finalImages)
	}
}

func TestApplySheinStudioAIImagesToSheinAppendsForHybrid(t *testing.T) {
	pkg := &sheinpub.Package{
		Images: &common.ImageSet{
			MainImage: "https://cdn.sdspod.com/rendered-main.jpg",
			Gallery:   []string{"https://cdn.sdspod.com/rendered-gallery.jpg"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(&common.ImageSet{
				MainImage: "https://cdn.sdspod.com/rendered-main.jpg",
				Gallery:   []string{"https://cdn.sdspod.com/rendered-gallery.jpg"},
			}),
		},
	}

	applySheinStudioAIImagesToShein(pkg, &GenerateRequest{
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyHybrid,
			SheinStudio: &SheinStudioOptions{
				ProductImageURLs: []string{"https://cdn.example.com/ai-gallery.png"},
			},
		},
	}, nil)

	if pkg.Images.MainImage != "https://cdn.sdspod.com/rendered-main.jpg" {
		t.Fatalf("main image = %q", pkg.Images.MainImage)
	}
	if got := pkg.Images.Gallery; len(got) != 2 || got[1] != "https://cdn.example.com/ai-gallery.png" {
		t.Fatalf("gallery = %+v", got)
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product missing")
	}
	if got := len(pkg.PreviewProduct.ImageInfo.ImageInfoList); got != 3 {
		t.Fatalf("preview image count = %d", got)
	}
}

func TestApplySheinStudioAIImagesToSheinFallsBackToSDSMockupsWhenNoProductImages(t *testing.T) {
	sourceImage := "https://cdn.sdspod.com/images/preview-source.jpg"
	sdsImages := []string{
		"https://cdn.sdspod.com/out/36811/202605/rendered-main.jpg",
		"https://cdn.sdspod.com/out/36811/202605/rendered-gallery.jpg",
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

	applySheinStudioAIImagesToShein(pkg, &GenerateRequest{
		ImageURLs: []string{sourceImage},
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				SourceDesignURLs: []string{sourceImage},
			},
		},
	}, &SDSSyncSummary{
		Status:          "completed",
		MockupImageURLs: sdsImages,
	})

	if pkg.Images.MainImage != sdsImages[0] {
		t.Fatalf("main image = %q, want %q", pkg.Images.MainImage, sdsImages[0])
	}
	if len(pkg.Images.Gallery) != 1 || pkg.Images.Gallery[0] != sdsImages[1] {
		t.Fatalf("gallery = %+v", pkg.Images.Gallery)
	}
	if pkg.RequestDraft.ImageInfo.MainImage != sdsImages[0] {
		t.Fatalf("draft image info = %+v", pkg.RequestDraft.ImageInfo)
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].MainImage != sdsImages[0] {
		t.Fatalf("sku main image = %q", pkg.RequestDraft.SKCList[0].SKUList[0].MainImage)
	}
	if pkg.PreviewProduct == nil || pkg.PreviewProduct.ImageInfo == nil {
		t.Fatalf("preview product missing image info: %+v", pkg.PreviewProduct)
	}
	if pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL != sdsImages[0] {
		t.Fatalf("preview main image = %q, want %q", pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL, sdsImages[0])
	}
	if pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL == sourceImage {
		t.Fatalf("preview product still uses source image: %q", sourceImage)
	}
}

func TestApplySheinStudioAIImagesToSheinUsesVariantImagesForSKCs(t *testing.T) {
	pkg := &sheinpub.Package{
		Images: sheinImageSet("https://cdn.example.com/flat-design.png"),
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: sheinpub.BuildImageDraft(sheinImageSet("https://cdn.example.com/flat-design.png")),
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName:      "black",
					SaleName:     "black",
					SupplierCode: "MG8012004001-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Color",
						Value: "black",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "MG8012004001-STYLE",
						Attributes: map[string]string{
							"Color":          "black",
							"source_sds_sku": "MG8012004001",
						},
					}},
				},
				{
					SkcName:      "white",
					SaleName:     "white",
					SupplierCode: "MG8012004002-STYLE",
					SaleAttribute: &sheinpub.ResolvedSaleAttribute{
						Name:  "Color",
						Value: "white",
					},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "MG8012004002-STYLE",
						Attributes: map[string]string{
							"Color":          "white",
							"source_sds_sku": "MG8012004002",
						},
					}},
				},
			},
		},
		SkcList: []sheinpub.SKCPackage{
			{SupplierCode: "MG8012004001-STYLE", SkcName: "black", SaleName: "black", Attributes: map[string]string{"Color": "black"}},
			{SupplierCode: "MG8012004002-STYLE", SkcName: "white", SaleName: "white", Attributes: map[string]string{"Color": "white"}},
		},
	}

	applySheinStudioAIImagesToShein(pkg, &GenerateRequest{
		ImageURLs: []string{"https://cdn.example.com/source-style.png"},
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				ProductImageURLs: []string{"https://cdn.example.com/black-main.png"},
				VariantProductImages: []SheinStudioVariantImageSet{
					{
						VariantSKU: "MG8012004001",
						Color:      "black",
						ImageURLs:  []string{"https://cdn.example.com/black-main.png"},
					},
					{
						VariantSKU: "MG8012004002",
						Color:      "white",
						ImageURLs:  []string{"https://cdn.example.com/white-main.png"},
					},
				},
			},
		},
	}, nil)

	if got := pkg.RequestDraft.SKCList[0].SKUList[0].MainImage; got != "https://cdn.example.com/black-main.png" {
		t.Fatalf("black sku main image = %q", got)
	}
	if got := pkg.RequestDraft.SKCList[1].SKUList[0].MainImage; got != "https://cdn.example.com/white-main.png" {
		t.Fatalf("white sku main image = %q", got)
	}
	if got := pkg.SkcList[1].MainImageURL; got != "https://cdn.example.com/white-main.png" {
		t.Fatalf("white skc main image = %q", got)
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product missing")
	}
}

func TestEnforceSheinVariantImageCoverageBlocksSharedSingleImageAcrossMultipleSKCs(t *testing.T) {
	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName:      "black",
					SupplierCode: "SKU-BLK-STYLE",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/shared.png"},
					SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "SKU-BLK", MainImage: "https://cdn.example.com/shared.png"}},
				},
				{
					SkcName:      "white",
					SupplierCode: "SKU-WHT-STYLE",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/shared.png"},
					SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "SKU-WHT", MainImage: "https://cdn.example.com/shared.png"}},
				},
			},
		},
		SkcList: []sheinpub.SKCPackage{
			{SupplierCode: "SKU-BLK-STYLE", MainImageURL: "https://cdn.example.com/shared.png"},
			{SupplierCode: "SKU-WHT-STYLE", MainImageURL: "https://cdn.example.com/shared.png"},
		},
		PreviewProduct: &sheinproduct.Product{
			SKCList: []sheinproduct.SKC{
				{ImageInfo: *sheinImageInfo([]string{"https://cdn.example.com/shared.png"}), SKUS: []sheinproduct.SKU{{ImageInfo: sheinImageInfo([]string{"https://cdn.example.com/shared.png"})}}},
				{ImageInfo: *sheinImageInfo([]string{"https://cdn.example.com/shared.png"}), SKUS: []sheinproduct.SKU{{ImageInfo: sheinImageInfo([]string{"https://cdn.example.com/shared.png"})}}},
			},
		},
	}

	warning, blocked := enforceSheinVariantImageCoverage(pkg, &GenerateRequest{
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				ProductImageURLs: []string{"https://cdn.example.com/shared.png"},
			},
		},
	}, &SDSSyncSummary{
		Status: "failed",
		Error:  "SDS render failed for selected color variants: white",
	})

	if !blocked {
		t.Fatal("blocked = false, want true")
	}
	if !strings.Contains(warning, "SDS render failed for selected color variants: white") {
		t.Fatalf("warning = %q", warning)
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo == nil || pkg.RequestDraft.SKCList[1].ImageInfo == nil {
		t.Fatalf("request draft skc image info should be preserved: %+v", pkg.RequestDraft.SKCList)
	}
	if len(pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList) == 0 || len(pkg.PreviewProduct.SKCList[1].ImageInfo.ImageInfoList) == 0 {
		t.Fatalf("preview skc images should be preserved: %+v", pkg.PreviewProduct.SKCList)
	}
}

func TestEnforceSheinVariantImageCoverageAllowsCompleteVariantImages(t *testing.T) {
	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{
				{SkcName: "black", SupplierCode: "SKU-BLK-STYLE", ImageInfo: &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/black.png"}},
				{SkcName: "white", SupplierCode: "SKU-WHT-STYLE", ImageInfo: &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/white.png"}},
			},
		},
	}

	warning, blocked := enforceSheinVariantImageCoverage(pkg, &GenerateRequest{
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				VariantProductImages: []SheinStudioVariantImageSet{
					{VariantSKU: "SKU-BLK", Color: "black", ImageURLs: []string{"https://cdn.example.com/black.png"}},
					{VariantSKU: "SKU-WHT", Color: "white", ImageURLs: []string{"https://cdn.example.com/white.png"}},
				},
			},
		},
	}, nil)

	if blocked {
		t.Fatalf("blocked = true, warning = %q", warning)
	}
}

func TestEnforceSheinVariantImageCoverageAllowsFallbackSplitSKCsForSameColor(t *testing.T) {
	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SkcName:      "white",
					SaleName:     "white",
					SupplierCode: "SKU-WHITE-30-STYLE",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/white-shared.png"},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "SKU-WHITE-30",
						MainImage:   "https://cdn.example.com/white-shared.png",
						Attributes:  map[string]string{"Color": "white", "Size": "30x40cm"},
					}},
				},
				{
					SkcName:      "white",
					SaleName:     "white",
					SupplierCode: "SKU-WHITE-35-STYLE",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: "https://cdn.example.com/white-shared.png"},
					SKUList: []sheinpub.SKUDraft{{
						SupplierSKU: "SKU-WHITE-35",
						MainImage:   "https://cdn.example.com/white-shared.png",
						Attributes:  map[string]string{"Color": "white", "Size": "35x50cm"},
					}},
				},
			},
		},
	}

	warning, blocked := enforceSheinVariantImageCoverage(pkg, &GenerateRequest{
		Options: &GenerateOptions{
			ImageStrategy: sheinImageStrategyAIGenerated,
			SheinStudio: &SheinStudioOptions{
				VariantProductImages: []SheinStudioVariantImageSet{
					{VariantSKU: "SKU-WHITE-30", Color: "white", ImageURLs: []string{"https://cdn.example.com/white-shared.png"}},
				},
			},
		},
	}, &SDSSyncSummary{
		Status: "completed",
		VariantResults: []SDSSyncSummary{
			{VariantSKU: "SKU-WHITE-30", VariantColor: "white", Status: "completed", MockupImageURLs: []string{"https://cdn.example.com/white-shared.png"}},
		},
	})

	if blocked {
		t.Fatalf("blocked = true, warning = %q", warning)
	}
}

func sheinImageSet(main string) *common.ImageSet {
	return &common.ImageSet{MainImage: main}
}

func hasSizeReferenceImage(images []sheinproduct.ImageDetail, imageURL string) bool {
	for _, image := range images {
		if image.ImageURL == imageURL && image.SizeImgFlag && image.ImageType == 6 {
			return true
		}
	}
	return false
}

func hasFinalReviewSizeMap(images []SheinFinalReviewImage, imageURL string) bool {
	for _, image := range images {
		if image.URL == imageURL && image.Role == "size_map" && image.SizeMap {
			return true
		}
	}
	return false
}

func TestBuildSheinFinalReviewImagesDeduplicatesRepeatedMainImage(t *testing.T) {
	mainImage := "https://cdn.sdspod.com/out/main.jpg"
	draft := &sheinpub.RequestDraft{
		ImageInfo: &sheinpub.ImageDraft{
			MainImage: mainImage,
			Gallery: []string{
				mainImage,
				"https://cdn.sdspod.com/out/detail-1.jpg",
			},
		},
		SKCList: []sheinpub.SKCRequestDraft{{
			ImageInfo: &sheinpub.ImageDraft{
				MainImage: mainImage,
			},
		}},
	}

	images := sheinworkspace.BuildFinalReviewImages(draft, &sheinpub.FinalDraft{MainImageURL: mainImage}, nil)
	if len(images) != 2 {
		t.Fatalf("final review image count = %d, want 2: %+v", len(images), images)
	}
	if images[0].URL != mainImage || images[0].Role != "main" || !images[0].Main {
		t.Fatalf("first image = %+v", images[0])
	}
	if images[1].URL != "https://cdn.sdspod.com/out/detail-1.jpg" || images[1].Role != "gallery" {
		t.Fatalf("second image = %+v", images[1])
	}
}
