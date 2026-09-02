package listingkit

import (
	"context"
	"fmt"
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/product/catalog/canonical"
	"task-processor/internal/productimage"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsdesign "task-processor/internal/sds/design"
	sdsworkflow "task-processor/internal/sds/workflow"
)

func TestStudioAttributesAndSpecificationsIncludeRichSDSFields(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU:             "MG17701062",
		Material:               "复合板",
		MaterialDescription:    "优选复合板材质",
		ProductionProcess:      "UV打印",
		ProductPerformance:     "静音无声，轻奢质地。",
		ApplicableScenarios:    "办公室、卧室、客厅",
		SpecialDescription:     "挂钟不含电池。",
		ProductSize:            "25*25cm",
		PackagingSpecification: "30*30*5cm，0.45kg",
		VariantSize:            "25cm/9.8inch",
		VariantColor:           "White",
	}

	attrs := studioAttributes(sds, canonical.FieldTrace{})
	if attrs["material_description"].Value != "优选复合板材质" {
		t.Fatalf("material_description = %+v", attrs["material_description"])
	}
	if attrs["product_performance"].Value == "" {
		t.Fatalf("product_performance = %+v", attrs["product_performance"])
	}
	if attrs["product_size"].Value != "25*25cm" {
		t.Fatalf("product_size = %+v", attrs["product_size"])
	}
	if attrs["packaging_specification"].Value == "" {
		t.Fatalf("packaging_specification = %+v", attrs["packaging_specification"])
	}

	specs := studioSpecifications(sds)
	if specs == nil {
		t.Fatal("specs = nil")
	}
	if specs.Technical["product_size"] != "25*25cm" {
		t.Fatalf("technical product_size = %+v", specs.Technical)
	}
	if specs.Technical["packaging_specification"] != "30*30*5cm，0.45kg" {
		t.Fatalf("technical packaging_specification = %+v", specs.Technical)
	}
	if specs.Technical["product_performance"] == "" {
		t.Fatalf("technical product_performance = %+v", specs.Technical)
	}
}

func TestStudioVariantsAddsVariantDiscriminatorWhenVariantSKUMissing(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantID: 101, Color: "黑色", Size: "均码"},
			{VariantID: 102, Color: "白色", Size: "均码"},
		},
	}

	variants := studioVariants(sds, nil, canonical.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-V101-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-V102-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
	if variants[0].SKU == variants[1].SKU {
		t.Fatalf("expected unique skus, got %q", variants[0].SKU)
	}
}

func TestStudioVariantsDeduplicatesRepeatedBaseSKU(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantSKU: "MG8014186001", Color: "Black", Size: "One Size"},
			{VariantSKU: "MG8014186001", Color: "Gray", Size: "One Size"},
		},
	}

	variants := studioVariants(sds, nil, canonical.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-BLACK-ONE-SIZE-V1-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-GRAY-ONE-SIZE-V2-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
	if variants[0].SKU == variants[1].SKU {
		t.Fatalf("expected unique skus, got %q", variants[0].SKU)
	}
}

func TestStudioVariantsPreservesDistinctVariantSKUs(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantSKU: "MG8014186001-BLK", Color: "Black", Size: "One Size"},
			{VariantSKU: "MG8014186001-WHT", Color: "White", Size: "One Size"},
		},
	}

	variants := studioVariants(sds, nil, canonical.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].SKU != "MG8014186001-BLK-D7E68190" {
		t.Fatalf("first sku = %q", variants[0].SKU)
	}
	if variants[1].SKU != "MG8014186001-WHT-D7E68190" {
		t.Fatalf("second sku = %q", variants[1].SKU)
	}
}

func TestStudioVariantsPreserveVariantDimensionsAndWeight(t *testing.T) {
	sds := &SDSSyncOptions{
		ProductSKU: "MG8014186001",
		StyleID:    "d7e6-8190-abcdef",
		Variants: []SDSSyncVariantOption{
			{VariantID: 101, Color: "Black", Size: "40x60cm", Weight: 120, BoxLength: 40, BoxWidth: 30, BoxHeight: 2},
			{VariantID: 102, Color: "Black", Size: "50x80cm", Weight: 180, BoxLength: 50, BoxWidth: 40, BoxHeight: 3},
		},
	}

	variants := studioVariants(sds, nil, canonical.FieldTrace{})
	if len(variants) != 2 {
		t.Fatalf("variant count = %d, want 2", len(variants))
	}
	if variants[0].Dimensions == nil || variants[1].Dimensions == nil {
		t.Fatalf("expected variant dimensions to be preserved: %+v", variants)
	}
	if variants[0].Dimensions.Length != 40 || variants[0].Dimensions.Width != 30 || variants[0].Dimensions.Height != 2 {
		t.Fatalf("first dimensions = %+v", variants[0].Dimensions)
	}
	if variants[1].Dimensions.Length != 50 || variants[1].Dimensions.Width != 40 || variants[1].Dimensions.Height != 3 {
		t.Fatalf("second dimensions = %+v", variants[1].Dimensions)
	}
	if variants[0].Weight == nil || variants[1].Weight == nil {
		t.Fatalf("expected variant weight to be preserved: %+v", variants)
	}
	if variants[0].Weight.Value != 120 || variants[1].Weight.Value != 180 {
		t.Fatalf("weights = %+v / %+v", variants[0].Weight, variants[1].Weight)
	}
}

func TestApplySDSSyncMetadataToCanonicalPromotesRenderedMockupsToCanonicalImages(t *testing.T) {
	trace := canonical.FieldTrace{}
	product := &canonical.Product{
		Title: "Source clock",
		Images: []canonical.Image{
			{URL: "https://example.com/source-design.png", Role: "primary", Trace: trace},
		},
		Variants: []canonical.Variant{
			{
				SKU: "MG17701061001-STYLE",
				Attributes: map[string]canonical.Attribute{
					"source_sds_sku": {Value: "MG17701061001", Trace: trace},
				},
				Images: []canonical.Image{{URL: "https://example.com/source-design.png", Role: "primary", Trace: trace}},
			},
			{
				SKU: "MG17701061002-STYLE",
				Attributes: map[string]canonical.Attribute{
					"source_sds_sku": {Value: "MG17701061002", Trace: trace},
				},
				Images: []canonical.Image{{URL: "https://example.com/source-design.png", Role: "primary", Trace: trace}},
			},
		},
	}

	changed := applySDSSyncMetadataToCanonical(product, &SDSSyncSummary{
		ProductName: "Rendered clock",
		VariantResults: []SDSSyncSummary{
			{
				VariantSKU:      "MG17701061001",
				VariantColor:    "Black",
				Status:          "completed",
				MockupImageURLs: []string{"https://cdn.sdspod.com/out/black-main.jpg", "https://cdn.sdspod.com/out/black-side.jpg"},
			},
			{
				VariantSKU:      "MG17701061002",
				VariantColor:    "White",
				Status:          "completed",
				MockupImageURLs: []string{"https://cdn.sdspod.com/out/white-main.jpg", "https://cdn.sdspod.com/out/white-side.jpg"},
			},
		},
	}, nil)

	if !changed {
		t.Fatal("changed = false, want true")
	}
	if got := product.Images; len(got) != 4 ||
		got[0].URL != "https://cdn.sdspod.com/out/black-main.jpg" ||
		got[0].Role != "primary" ||
		got[3].URL != "https://cdn.sdspod.com/out/white-side.jpg" {
		t.Fatalf("canonical images = %+v", got)
	}
	if got := product.Variants[0].Images; len(got) != 2 || got[0].URL != "https://cdn.sdspod.com/out/black-main.jpg" {
		t.Fatalf("black variant images = %+v", got)
	}
	if got := product.Variants[1].Images; len(got) != 2 || got[0].URL != "https://cdn.sdspod.com/out/white-main.jpg" {
		t.Fatalf("white variant images = %+v", got)
	}
	if got := product.FieldTraces["images"]; got.Sources[0].Detail != "SDS rendered mockup images" {
		t.Fatalf("images trace = %+v", got)
	}
}

func TestApplySDSSyncMetadataToCanonicalMatchesDecoratedSupplierSKU(t *testing.T) {
	product := &canonical.Product{
		Variants: []canonical.Variant{{
			SKU: "MG17701061001-B6C753EB",
		}},
	}
	summary := &SDSSyncSummary{
		VariantResults: []SDSSyncSummary{{
			VariantSKU:      "MG17701061001",
			Status:          "completed",
			MockupImageURLs: []string{"https://cdn.sdspod.com/out/main.jpg"},
		}},
	}

	applySDSSyncMetadataToCanonical(product, summary, nil)

	if got := product.Variants[0].Images; len(got) != 1 || got[0].URL != "https://cdn.sdspod.com/out/main.jpg" {
		t.Fatalf("decorated supplier SKU variant images = %+v", got)
	}
}

func TestApplySDSSyncMetadataToCanonicalPromotesSDSIdentityAttributes(t *testing.T) {
	product := &canonical.Product{
		Variants: []canonical.Variant{
			{
				SKU: "MG8014070001-B6C753EB",
				Attributes: map[string]canonical.Attribute{
					"source_sds_sku": {Value: "MG8014070001"},
				},
			},
			{
				SKU: "MG8014070002-B6C753EB",
				Attributes: map[string]canonical.Attribute{
					"source_sds_sku": {Value: "MG8014070002"},
				},
			},
		},
	}

	changed := applySDSSyncMetadataToCanonical(product, &SDSSyncSummary{
		ProductSKU:   "MG8014070",
		VariantSKU:   "MG8014070001",
		VariantSize:  "40x60cm",
		VariantColor: "white",
	}, nil)

	if !changed {
		t.Fatal("changed = false, want true")
	}
	if product.Attributes == nil {
		t.Fatal("attributes = nil")
	}
	for key, want := range map[string]string{
		"sku":           "MG8014070",
		"product_sku":   "MG8014070",
		"variant_sku":   "MG8014070001",
		"variant_size":  "40x60cm",
		"variant_color": "white",
	} {
		if got := product.Attributes[key].Value; got != want {
			t.Fatalf("attribute %s = %q, want %q", key, got, want)
		}
	}
	if got := product.FieldTraces["attributes"].Sources[0].Detail; got != "SDS design product identity" {
		t.Fatalf("attributes trace detail = %q", got)
	}
}

func TestRunStandardProductWorkflowUsesSnapshotInsteadOfCachedSDSBaseline(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	baselineRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}

	task := &Task{TenantID: "tenant-test", ID: "task-baseline-hit",
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "baseline task",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
					VariantID:        101,
					ProductName:      "Runtime Overlay Title",
				},
			},
		},
	}

	payload, err := newSDSBaselineCanonicalProductPayload(&canonical.Product{
		Title:  "Baseline Title",
		Images: []canonical.Image{{URL: "https://example.com/baseline.jpg", Role: "primary"}},
	})
	if err != nil {
		t.Fatalf("newSDSBaselineCanonicalProductPayload: %v", err)
	}
	if err := baselineRepo.SaveSDSBaselineCache(context.Background(), &SDSBaselineCacheEntry{
		TenantID:             "",
		BaselineKey:          sdsBaselineKey("", task.Request.Options.SDS),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusReady,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{ID: "unexpected-product-task"},
		product: &stubProductSnapshotFixture{
			Title: "Unexpected Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if productSvc.lastReq == nil {
		t.Fatal("product snapshot query = nil, want authoritative snapshot read")
	}
	if state.result.CanonicalProduct == nil {
		t.Fatal("expected canonical product from baseline")
	}
	if state.result.CanonicalProduct.Title != "Runtime Overlay Title" {
		t.Fatalf("canonical title = %q, want runtime overlay title", state.result.CanonicalProduct.Title)
	}
	if len(state.result.CanonicalProduct.Images) != 0 {
		t.Fatalf("canonical images = %+v, want cached baseline images ignored", state.result.CanonicalProduct.Images)
	}
}

func TestRunStandardProductWorkflow_DoesNotReuseBlockedSDSBaseline(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	baselineRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}

	task := &Task{TenantID: "tenant-test", ID: "task-blocked-baseline-fallback",
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "fallback product title",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
					VariantID:        101,
				},
			},
		},
	}

	payload, err := newSDSBaselineCanonicalProductPayload(&canonical.Product{
		Title:  "Blocked Baseline Title",
		Images: []canonical.Image{{URL: "https://example.com/blocked-baseline.jpg", Role: "primary"}},
	})
	if err != nil {
		t.Fatalf("newSDSBaselineCanonicalProductPayload: %v", err)
	}
	if err := baselineRepo.SaveSDSBaselineCache(context.Background(), &SDSBaselineCacheEntry{
		BaselineKey:          sdsBaselineKey("", task.Request.Options.SDS),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusBlocked,
		ValidationReasonCode: SDSBaselineReasonCodeLayerMissing,
		ValidationReason:     "layer is missing",
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{ID: "unexpected-product-task"},
		product: &stubProductSnapshotFixture{
			Title: "Unexpected Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if productSvc.lastReq == nil {
		t.Fatal("product snapshot query = nil, want authoritative snapshot read")
	}
	if state.result.CanonicalProduct == nil {
		t.Fatal("expected fallback canonical product")
	}
	if state.result.CanonicalProduct.Title == "Blocked Baseline Title" {
		t.Fatalf("canonical product = %+v, want blocked baseline ignored", state.result.CanonicalProduct)
	}
	if len(state.result.CanonicalProduct.Images) == 1 && state.result.CanonicalProduct.Images[0].URL == "https://example.com/blocked-baseline.jpg" {
		t.Fatalf("canonical images = %+v, want blocked baseline images ignored", state.result.CanonicalProduct.Images)
	}
}

func TestRunStandardProductWorkflowQueriesSnapshotWithTaskTenantID(t *testing.T) {
	t.Parallel()

	repo := NewInMemoryRepositoryForTest()
	baselineRepo, ok := repo.(SDSBaselineCacheRepository)
	if !ok {
		t.Fatal("mem task repository does not expose SDS baseline cache repository")
	}

	task := &Task{
		ID:       "task-baseline-tenant-fallback",
		TenantID: "tenant-a",
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "baseline tenant fallback",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
					VariantID:        101,
					ProductName:      "Runtime Overlay Title",
				},
			},
		},
	}

	payload, err := newSDSBaselineCanonicalProductPayload(&canonical.Product{
		Title: "Baseline Title",
	})
	if err != nil {
		t.Fatalf("newSDSBaselineCanonicalProductPayload: %v", err)
	}
	if err := baselineRepo.SaveSDSBaselineCache(context.Background(), &SDSBaselineCacheEntry{
		TenantID:             "tenant-a",
		BaselineKey:          sdsBaselineKey("tenant-a", task.Request.Options.SDS),
		Status:               SDSBaselineStatusBaselineCached,
		Version:              1,
		CanonicalProductBase: payload,
		ValidationStatus:     SDSBaselineValidationStatusReady,
	}); err != nil {
		t.Fatalf("SaveSDSBaselineCache: %v", err)
	}

	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{ID: "unexpected-product-task"},
		product: &stubProductSnapshotFixture{
			Title: "Unexpected Product",
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: repo,
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if productSvc.lastReq == nil || productSvc.lastReq.TenantID != "tenant-a" {
		t.Fatalf("product snapshot query = %+v, want tenant-a", productSvc.lastReq)
	}
	if state.result.CanonicalProduct == nil || state.result.CanonicalProduct.Title != "Runtime Overlay Title" {
		t.Fatalf("canonical product = %+v, want baseline with runtime overlay", state.result.CanonicalProduct)
	}
}

func TestRunStandardProductWorkflowUsesSnapshotWhenSDSBaselineMissing(t *testing.T) {
	t.Parallel()

	productSvc := &stubWorkflowProductSnapshotReader{
		task: &stubProductSnapshotTask{ID: "unexpected-product-task"},
		product: &stubProductSnapshotFixture{
			Title:      "Enriched Title",
			Category:   []string{"Home"},
			Images:     []string{"https://example.com/enriched.jpg"},
			Attributes: map[string]string{"brand": "DemoBrand"},
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: NewInMemoryRepositoryForTest(),
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), productSvc, nil)
	task := &Task{TenantID: "tenant-test", ID: "task-baseline-miss",
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "fallback task",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
					VariantID:        101,
				},
			},
		},
	}

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if productSvc.lastReq == nil {
		t.Fatal("product snapshot query = nil, want authoritative snapshot read")
	}
	if state.result.CanonicalProduct == nil {
		t.Fatal("expected canonical projection from product snapshot")
	}
	if state.result.CanonicalProduct.Title != "Enriched Title" {
		t.Fatalf("canonical title = %q, want snapshot title", state.result.CanonicalProduct.Title)
	}
}

func TestRunStandardProductWorkflowReappliesSDSMetadataWithoutDroppingProcessedAssets(t *testing.T) {
	t.Parallel()

	imageSvc := &stubWorkflowImageService{
		task: &productimage.Task{ID: "image-task-sds-assets"},
		result: &productimage.ImageProcessResult{
			MainImage:     &productimage.ImageAsset{URL: "https://cdn.example.com/main.jpg", SourceURL: "https://example.com/source.jpg"},
			WhiteBgImage:  &productimage.ImageAsset{URL: "https://cdn.example.com/white.jpg"},
			GalleryImages: []productimage.ImageAsset{{URL: "https://cdn.example.com/gallery.jpg"}},
		},
	}
	sdsSvc := &stubWorkflowSDSSyncService{
		result: &sdsadapter.SyncResult{
			DesignSync: &sdsworkflow.SyncResult{
				DesignResult: &sdsdesign.PrepareSyncDesignResult{
					Page: &sdsdesign.DesignProductPage{
						Product: sdsdesign.DesignProduct{
							ID:        101,
							Name:      "Rendered Product",
							SKU:       "SKU-101",
							ParentSKU: "SPU-101",
							ColorName: "Black",
						},
					},
					Request: &sdsdesign.SyncDesignRequest{
						PrototypeGroupID: 7001,
						Prototypes: []sdsdesign.SyncDesignPrototype{{
							Layers: []sdsdesign.SyncDesignLayer{{LayerID: "layer-1"}},
						}},
					},
					RenderedImageURLs: []string{
						"https://cdn.sdspod.com/out/main-render.jpg",
						"https://cdn.sdspod.com/out/side-render.jpg",
					},
				},
			},
		},
	}
	svc := seedWorkflowServices(seedWorkflowAssets(seedSupportDeps(&service{
		repo: NewInMemoryRepositoryForTest(),
	}, supportDependencySeed{
		sdsSyncService: sdsSvc,
		assembler:      NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService()), nil, imageSvc)
	task := &Task{TenantID: "tenant-test", ID: "task-sds-assets",
		Request: &GenerateRequest{ProductKey: "test-product",
			ImageURLs: []string{"https://example.com/source.jpg"},
			Text:      "fallback task",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options: &GenerateOptions{
				ProcessImages: true,
				SDS: &SDSSyncOptions{
					VariantID:        101,
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
				},
			},
		},
	}

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v", err)
	}
	if state.result.AssetBundle == nil {
		t.Fatal("expected asset bundle")
	}
	if !bundleHasAssetKind(state.result.AssetBundle, asset.KindMainImage) {
		t.Fatalf("asset bundle = %+v, want processed main image retained", state.result.AssetBundle)
	}
	if !bundleHasAssetKind(state.result.AssetBundle, asset.KindWhiteBgImage) {
		t.Fatalf("asset bundle = %+v, want processed white background image retained", state.result.AssetBundle)
	}
	if !bundleHasAssetKind(state.result.AssetBundle, asset.KindGalleryImage) {
		t.Fatalf("asset bundle = %+v, want processed gallery image retained", state.result.AssetBundle)
	}
	if state.result.AssetBundle.Selection == nil || state.result.AssetBundle.Selection.MainAssetID == "" {
		t.Fatalf("asset bundle selection = %+v, want processed selection to remain", state.result.AssetBundle.Selection)
	}
	if state.result.CanonicalProduct == nil || len(state.result.CanonicalProduct.Images) != 2 {
		t.Fatalf("canonical images = %+v, want SDS rendered images applied", state.result.CanonicalProduct)
	}
	if state.result.CanonicalProduct.Images[0].URL != "https://cdn.sdspod.com/out/main-render.jpg" {
		t.Fatalf("canonical images = %+v, want rendered SDS mockup first", state.result.CanonicalProduct.Images)
	}
}

func TestRunStandardProductWorkflowSnapshotReadIsIndependentOfSDSBaselineLookupErrors(t *testing.T) {
	t.Parallel()

	svc := seedSupportDeps(&service{
		repo: &stubInlineTaskRepo{
			tasks:             map[string]*Task{},
			sdsBaselineGetErr: fmt.Errorf("baseline repo unavailable"),
		},
	}, supportDependencySeed{
		assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
	})
	task := &Task{TenantID: "tenant-test", ID: "task-baseline-error",
		Request: &GenerateRequest{ProductKey: "test-product",
			Text:      "fallback on error",
			Platforms: []string{"amazon"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID:        101,
					ParentProductID:  9001,
					PrototypeGroupID: 7001,
				},
			},
		},
	}

	state, err := svc.runStandardProductWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runStandardProductWorkflow() error = %v, want baseline lookup errors to degrade gracefully", err)
	}
	if state.result.CanonicalProduct == nil || state.result.CanonicalProduct.Title != "Test Product" {
		t.Fatalf("canonical product = %+v, want product snapshot despite baseline error", state.result.CanonicalProduct)
	}
}

func TestRunStandardProductWorkflowIgnoresUnavailableOrMalformedSDSBaselineEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry *SDSBaselineCacheEntry
	}{
		{
			name: "non-cached baseline",
			entry: &SDSBaselineCacheEntry{
				BaselineKey: "placeholder",
				Status:      "pending",
				Version:     1,
				CanonicalProductBase: func() *SDSBaselineCanonicalProductPayload {
					payload, _ := newSDSBaselineCanonicalProductPayload(&canonical.Product{Title: "Pending Baseline"})
					return payload
				}(),
			},
		},
		{
			name: "cached baseline missing payload",
			entry: &SDSBaselineCacheEntry{
				BaselineKey: "placeholder",
				Status:      SDSBaselineStatusBaselineCached,
				Version:     1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewInMemoryRepositoryForTest()
			baselineRepo, ok := repo.(SDSBaselineCacheRepository)
			if !ok {
				t.Fatal("mem task repository does not expose SDS baseline cache repository")
			}
			task := &Task{TenantID: "tenant-test", ID: "task-baseline-fallback-" + tt.name,
				Request: &GenerateRequest{ProductKey: "test-product",
					Text:      "baseline fallback",
					Platforms: []string{"amazon"},
					Options: &GenerateOptions{
						ProcessImages: false,
						SDS: &SDSSyncOptions{
							VariantID:        101,
							ParentProductID:  9001,
							PrototypeGroupID: 7001,
						},
					},
				},
			}
			entry := *tt.entry
			entry.BaselineKey = sdsBaselineKey("", task.Request.Options.SDS)
			if err := baselineRepo.SaveSDSBaselineCache(context.Background(), &entry); err != nil {
				t.Fatalf("SaveSDSBaselineCache: %v", err)
			}

			svc := seedWorkflowAssets(seedSupportDeps(&service{
				repo: repo,
			}, supportDependencySeed{
				assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
			}), nil, newDefaultAssetRecipeResolver(), newDefaultAssetBundleBuilder(), newDefaultAssetGenerationService())

			state, err := svc.runStandardProductWorkflow(context.Background(), task)
			if err != nil {
				t.Fatalf("runStandardProductWorkflow() error = %v", err)
			}
			if state.result.CanonicalProduct == nil || state.result.CanonicalProduct.Title != "Test Product" {
				t.Fatalf("canonical product = %+v, want product snapshot", state.result.CanonicalProduct)
			}
		})
	}
}

func bundleHasAssetKind(bundle *asset.Bundle, kind asset.Kind) bool {
	if bundle == nil {
		return false
	}
	for _, item := range bundle.Assets {
		if item.Kind == kind {
			return true
		}
	}
	return false
}
