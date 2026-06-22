package listingkit

import (
	"testing"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSheinSubmitReadinessBlockedWhenCoreFieldsMissing(t *testing.T) {
	t.Parallel()

	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryPath: []string{"Home", "Kitchen", "Bottle"},
		ReviewNotes:  []string{"人工确认尺寸映射"},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	if readiness.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	if len(readiness.BlockingItems) < 4 {
		t.Fatalf("blocking items = %+v, want multiple blockers", readiness.BlockingItems)
	}
	categoryBlocker := readiness.BlockingItems[0]
	if categoryBlocker.Key != "category" {
		t.Fatalf("first blocker key = %q, want category", categoryBlocker.Key)
	}
	if categoryBlocker.Reason == nil || categoryBlocker.Reason.Code != "category_unresolved" {
		t.Fatalf("category blocker reason = %+v", categoryBlocker.Reason)
	}
	if len(categoryBlocker.RepairHints) != 1 || categoryBlocker.RepairHints[0].Target != "editor.category" {
		t.Fatalf("category blocker repair hints = %+v", categoryBlocker.RepairHints)
	}
	if categoryBlocker.RepairHints[0].EditorSection != "category" || categoryBlocker.RepairHints[0].RevisionPath != "shein.category_resolution" {
		t.Fatalf("category blocker editor metadata = %+v", categoryBlocker.RepairHints[0])
	}
	if categoryBlocker.RepairHints[0].Patch == nil || categoryBlocker.RepairHints[0].Patch.CategoryResolution == nil {
		t.Fatalf("category blocker patch = %+v", categoryBlocker.RepairHints[0].Patch)
	}
	if categoryBlocker.RepairHints[0].Skeleton == nil || categoryBlocker.RepairHints[0].Skeleton.Shein == nil || categoryBlocker.RepairHints[0].Skeleton.Shein.CategoryResolution == nil {
		t.Fatalf("category blocker skeleton = %+v", categoryBlocker.RepairHints[0].Skeleton)
	}
	if categoryBlocker.RepairHints[0].Revision == nil || categoryBlocker.RepairHints[0].Revision.Shein == nil || categoryBlocker.RepairHints[0].Revision.Shein.CategoryResolution == nil {
		t.Fatalf("category blocker revision = %+v", categoryBlocker.RepairHints[0].Revision)
	}
	if categoryBlocker.RepairHints[0].Validation == nil || !categoryBlocker.RepairHints[0].Validation.Valid || categoryBlocker.RepairHints[0].Validation.Status != "ready" {
		t.Fatalf("category blocker validation = %+v", categoryBlocker.RepairHints[0].Validation)
	}
	if len(categoryBlocker.RepairHints[0].Validation.CategoryPreviewEffects) == 0 {
		t.Fatalf("category blocker validation effects = %+v", categoryBlocker.RepairHints[0].Validation)
	}
	if len(readiness.WarningItems) != 1 || readiness.WarningItems[0].Key != "manual_notes" {
		t.Fatalf("warning items = %+v", readiness.WarningItems)
	}
	if readiness.WarningItems[0].Reason == nil || readiness.WarningItems[0].Reason.Code != "manual_review_pending" {
		t.Fatalf("warning reason = %+v", readiness.WarningItems[0].Reason)
	}
}

func TestBuildSheinSubmitReadinessBlocksWhenSheinCookieUnavailable(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:      "resolved",
			CategoryID:  3001,
			ReviewNotes: []string{"SHEIN 店铺 cookie 不可用，已降级为离线解析"},
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 27,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "Black",
					AttributeID:      27,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
					BasePrice:   "10.00",
					SitePriceList: []SheinSitePrice{{
						SubSite:   "us",
						BasePrice: "10.00",
					}},
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		FinalDraft: &sheinpub.FinalDraft{
			Confirmed:       true,
			MainImageURL:    "https://cdn.example.com/main.jpg",
			SubmitMode:      "publish",
			FinalImageOrder: []string{"https://cdn.example.com/main.jpg"},
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/main.jpg": "swatch",
			},
		},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			MainImageURL: "https://cdn.example.com/main.jpg",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	var blocker *SheinReadinessItem
	for i := range readiness.BlockingItems {
		if readiness.BlockingItems[i].Key == sheinCookieUnavailableIssueCode {
			blocker = &readiness.BlockingItems[i]
			break
		}
	}
	if blocker == nil {
		t.Fatalf("blocking items = %+v, want cookie blocker", readiness.BlockingItems)
	}
	if blocker.Label != "SHEIN 店铺登录" {
		t.Fatalf("cookie blocker = %+v, want SHEIN store login label", blocker)
	}
}

func TestBuildSheinSubmitReadinessAllowsSingleVariantMainImageWithoutSizeMap(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Decor", "Wall Clock"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.sdspod.com/out/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001184,
		},
		RequestDraft: &SheinRequestDraft{
			ImageInfo: &SheinImageDraft{
				MainImage: "https://cdn.sdspod.com/out/main.jpg",
				Gallery:   []string{"https://cdn.sdspod.com/out/gallery.jpg"},
			},
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Style Type",
					Value:            "White",
					AttributeID:      1001184,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
					BasePrice:   "22.50",
					SitePriceList: []SheinSitePrice{{
						SubSite:   "us",
						BasePrice: "22.50",
					}},
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		FinalDraft: &sheinpub.FinalDraft{
			Confirmed:       true,
			MainImageURL:    "https://cdn.sdspod.com/out/main.jpg",
			SubmitMode:      "publish",
			FinalImageOrder: []string{"https://cdn.sdspod.com/out/main.jpg", "https://cdn.sdspod.com/out/gallery.jpg"},
		},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})

	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if !readiness.Ready {
		t.Fatalf("ready = false, want true; blocking items=%+v", readiness.BlockingItems)
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "final_images" {
			t.Fatalf("final_images blocked = %+v, want single variant main image accepted", item)
		}
	}
}

func TestBuildSheinSubmitReadinessSaveDraftDoesNotRequireFinalConfirmation(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       false,
		MainImageURL:    "https://cdn.example.com/main.jpg",
		FinalImageOrder: []string{"https://cdn.example.com/main.jpg"},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: "https://cdn.example.com/main.jpg",
		Gallery:   []string{"https://cdn.example.com/gallery.jpg"},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{
		"https://cdn.example.com/main.jpg",
		"https://cdn.example.com/gallery.jpg",
	})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{"https://cdn.example.com/main.jpg"})

	readiness := buildSheinSubmitReadinessForAction(task.Result.Shein, "save_draft")
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if !readiness.Ready {
		t.Fatalf("ready = false, want true for save_draft without final confirmation; blockers=%+v", readiness.BlockingItems)
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "final_review" {
			t.Fatalf("unexpected final_review blocker for save_draft: %+v", readiness.BlockingItems)
		}
	}
}

func TestBuildSheinSubmitReadinessReadyWithWarningsAfterManualNotes(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 501,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "Black",
					AttributeID:      501,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
		ReviewNotes: []string{"人工确认站点价格"},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready != true {
		t.Fatalf("ready = false, want true; readiness=%+v", readiness)
	}
	if readiness.Status != "ready_with_warnings" {
		t.Fatalf("status = %q, want ready_with_warnings", readiness.Status)
	}
	if len(readiness.BlockingItems) != 0 {
		t.Fatalf("blocking items = %+v, want none", readiness.BlockingItems)
	}
	if len(readiness.WarningItems) != 1 || readiness.WarningItems[0].Key != "manual_notes" {
		t.Fatalf("warning items = %+v", readiness.WarningItems)
	}
	if readiness.WarningItems[0].Reason == nil || readiness.WarningItems[0].Reason.Category != "manual_review" {
		t.Fatalf("warning reason = %+v", readiness.WarningItems[0].Reason)
	}
	if len(readiness.WarningItems[0].RepairHints) != 1 || readiness.WarningItems[0].RepairHints[0].Target != "editor.basics.review_notes" {
		t.Fatalf("warning repair hints = %+v", readiness.WarningItems[0].RepairHints)
	}
	if readiness.WarningItems[0].RepairHints[0].Patch == nil || len(readiness.WarningItems[0].RepairHints[0].Patch.ReviewNotes) != 1 {
		t.Fatalf("warning patch = %+v", readiness.WarningItems[0].RepairHints[0].Patch)
	}
	if readiness.WarningItems[0].RepairHints[0].Revision == nil || readiness.WarningItems[0].RepairHints[0].Revision.Shein == nil || len(readiness.WarningItems[0].RepairHints[0].Revision.Shein.ReviewNotes) != 1 {
		t.Fatalf("warning revision = %+v", readiness.WarningItems[0].RepairHints[0].Revision)
	}
	if readiness.WarningItems[0].RepairHints[0].Validation == nil || !readiness.WarningItems[0].RepairHints[0].Validation.Valid {
		t.Fatalf("warning validation = %+v", readiness.WarningItems[0].RepairHints[0].Validation)
	}
	if len(readiness.WarningItems[0].RepairHints[0].Validation.AffectedSections) == 0 {
		t.Fatalf("warning validation sections = %+v", readiness.WarningItems[0].RepairHints[0].Validation)
	}
}

func TestBuildSheinSubmitReadinessBlocksLLMOnly1688Facts(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 501,
		},
		RequestDraft: &SheinRequestDraft{
			ImageInfo: &SheinImageDraft{
				MainImage: "https://cdn.example.com/main.jpg",
				Gallery:   []string{"https://cdn.example.com/gallery.jpg"},
			},
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				ImageInfo: &SheinImageDraft{
					MainImage: "https://cdn.example.com/skc.jpg",
				},
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "Black",
					AttributeID:      501,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
		FinalDraft: &sheinpub.FinalDraft{
			Confirmed:    true,
			MainImageURL: "https://cdn.example.com/main.jpg",
			ImageRoleOverrides: map[string]string{
				"https://cdn.example.com/skc.jpg":     "swatch",
				"https://cdn.example.com/gallery.jpg": "size_map",
			},
		},
		Metadata: map[string]string{
			"source_platform":             "1688",
			"source_fact_review_required": "true",
			"source_fact_review_fields":   "selling_points,specifications",
		},
	})

	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	var found bool
	for _, item := range readiness.BlockingItems {
		if item.Key == "source_facts" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("blocking items = %+v, want source_facts blocker", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessAcceptsDraftMainImage(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Decor", "Cushions"},
		ProductTypeID: &productTypeID,
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 27,
		},
		RequestDraft: &SheinRequestDraft{
			ImageInfo: &SheinImageDraft{
				MainImage: "http://localhost:8080/api/v1/listing-kits/uploads/files/20260423/main.png",
			},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "images" {
			t.Fatalf("images should not block when request draft has a main image: %+v", item)
		}
	}
}

func TestBuildSheinSubmitReadinessBlocksPartialCustomSaleValuesFromDraft(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	customValueID := 277561282
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Outdoor", "Cushions"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "partial",
			PrimaryAttributeID: 27,
			ReviewNotes: []string{
				`SHEIN 销售属性值使用自定义值承接: 模板属性 "Color" 的值 "牛津布/防水防晒/深蓝" 已创建为自定义候选`,
			},
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "牛津布/防水防晒/深蓝",
					AttributeID:      27,
					AttributeValueID: &customValueID,
					MatchedBy:        "custom_attribute_value",
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	if readiness.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "sale_attributes" {
			found = true
			if item.Reason == nil || item.Reason.Code != "sale_attributes_unresolved" {
				t.Fatalf("sale attribute reason = %+v", item.Reason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected sale_attributes blocker, got %+v", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessBlocksWhenCategoryReviewStillPending(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:                  "partial",
			RecommendCategoryReview: true,
			CategoryReviewReason:    "当前类目路径与商品语义明显不一致",
			PrimaryAttributeID:      501,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "Black",
					AttributeID:      501,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	if readiness.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "category_review" {
			found = true
			if item.Reason == nil || item.Reason.Code != "category_review_pending" {
				t.Fatalf("category review reason = %+v", item.Reason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected category_review blocker, got %+v", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessBlocksWhenResolvedSaleAttributesLackValueIDs(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:               "resolved",
			PrimaryAttributeID:   1001466,
			SKCAttributes:        []SheinResolvedSaleAttribute{{Scope: "skc", Name: "Plug(Voltage)", Value: "white", AttributeID: 1001466}},
			SecondaryAttributeID: 0,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode:  "SKC-1",
				SaleAttribute: nil,
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{
			SKCList: []sheinproduct.SKC{{
				SaleAttribute: sheinproduct.SaleAttribute{
					AttributeID:      0,
					AttributeValueID: 0,
				},
				SKUS: []sheinproduct.SKU{{SupplierSKU: "SKU-1"}},
			}},
		},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "sale_attributes" {
			found = true
			if item.Reason == nil || item.Reason.Code != "sale_attributes_unresolved" {
				t.Fatalf("sale attribute reason = %+v", item.Reason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected sale_attributes blocker, got %+v", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessBlocksWhenMultipleSKUsLackSaleAttributes(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID = 0
	task.Result.Shein.SaleAttributeResolution.SKUAttributes = nil
	task.Result.Shein.SaleAttributeResolution.SecondarySourceDimension = "尺码"
	task.Result.Shein.SaleAttributeResolution.TemplateOptions = []sheinpub.SaleAttributeTemplateOption{
		{
			AttributeID: 27,
			Name:        "Color",
			NameEn:      "Color",
			SKCScope:    true,
			Important:   true,
		},
		{
			AttributeID: 87,
			Name:        "Size",
			NameEn:      "Size",
		},
	}
	task.Result.Shein.SaleAttributeResolution.Candidates = []sheinpub.SaleAttributeCandidateInfo{
		{
			SourceDimension: "颜色",
			Name:            "Color",
			AttributeID:     27,
			SKCScope:        true,
			SelectedScope:   "primary",
		},
		{
			SourceDimension: "尺码",
			Name:            "Size",
			AttributeID:     87,
			SelectedScope:   "secondary",
		},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList = append(
		task.Result.Shein.RequestDraft.SKCList[0].SKUList,
		SheinSKUDraft{
			SupplierSKU: "SKU-2",
		},
	)
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS = append(
		task.Result.Shein.PreviewProduct.SKCList[0].SKUS,
		sheinproduct.SKU{
			SupplierSKU: "SKU-2",
		},
	)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	task.Result.Shein.SkcList[0].SKUs = append(
		task.Result.Shein.SkcList[0].SKUs,
		PlatformVariant{
			SKU: "SKU-2",
			Attributes: map[string]string{
				"颜色": "Black",
				"尺码": "40",
			},
		},
	)

	readiness := buildSheinSubmitReadiness(task.Result.Shein)
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "sale_attributes" {
			found = true
			if item.Reason == nil || item.Reason.Code != "sale_attributes_unresolved" {
				t.Fatalf("sale attribute reason = %+v", item.Reason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected sale_attributes blocker, got %+v", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessAllowsPrimaryOnlyMultiSKUWhenSecondaryTemplateIsUnavailable(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	primaryValueID := 739
	task.Result.Shein.SaleAttributeResolution.SecondaryAttributeID = 0
	task.Result.Shein.SaleAttributeResolution.SKUAttributes = nil
	task.Result.Shein.SaleAttributeResolution.PrimaryAttributeID = 1001184
	task.Result.Shein.SaleAttributeResolution.PrimarySourceDimension = "Color"
	task.Result.Shein.SaleAttributeResolution.SecondarySourceDimension = "Size"
	task.Result.Shein.SaleAttributeResolution.TemplateOptions = []sheinpub.SaleAttributeTemplateOption{
		{
			AttributeID: 1001184,
			Name:        "Style Type",
			NameEn:      "Style Type",
			Important:   true,
		},
		{
			AttributeID: 27,
			Name:        "Color",
			NameEn:      "Color",
		},
	}
	task.Result.Shein.SaleAttributeResolution.Candidates = []sheinpub.SaleAttributeCandidateInfo{
		{
			SourceDimension: "Color",
			Name:            "Style Type",
			AttributeID:     1001184,
			SelectedScope:   "primary",
		},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SaleAttribute = &SheinResolvedSaleAttribute{
		Scope:            "skc",
		Name:             "Style Type",
		Value:            "white",
		AttributeID:      1001184,
		AttributeValueID: &primaryValueID,
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList = append(
		task.Result.Shein.RequestDraft.SKCList[0].SKUList,
		SheinSKUDraft{
			SupplierSKU: "SKU-2",
		},
	)
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SaleAttribute = sheinproduct.SaleAttribute{
		AttributeID:      1001184,
		AttributeValueID: 739,
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS = append(
		task.Result.Shein.PreviewProduct.SKCList[0].SKUS,
		sheinproduct.SKU{
			SupplierSKU: "SKU-2",
		},
	)
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	task.Result.Shein.SkcList[0].SKUs = append(
		task.Result.Shein.SkcList[0].SKUs,
		PlatformVariant{
			SKU: "SKU-2",
			Attributes: map[string]string{
				"Color": "white",
				"Size":  "35×50cm",
			},
		},
	)

	readiness := buildSheinSubmitReadiness(task.Result.Shein)
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if !readiness.Ready {
		t.Fatalf("ready = false, want true when secondary is optional; readiness=%+v", readiness)
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "sale_attributes" {
			t.Fatalf("unexpected sale_attributes blocker: %+v", readiness.BlockingItems)
		}
	}
}

func TestBuildSheinSubmitReadinessBlocksWhenRequiredDisplayAttributesArePending(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Outdoor", "Cushions"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "Material",
			AttributeID: 160,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:            "partial",
			ResolvedCount:     1,
			UnresolvedCount:   1,
			PendingAttributes: []PlatformAttribute{{Name: "Width (cm)"}},
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 27,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "Material",
				AttributeID: 160,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Name:             "Color",
					AttributeID:      27,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs:         []PlatformVariant{{SKU: "SKU-1"}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	if readiness.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "attribute_review" {
			found = true
			if item.Reason == nil || item.Reason.Code != "required_attributes_pending" {
				t.Fatalf("attribute review reason = %+v", item.Reason)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected attribute_review blocker, got %+v", readiness.BlockingItems)
	}
}

func TestBuildSheinSubmitReadinessDoesNotBlockWhenOnlyImportantDisplayAttributesArePending(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	colorValueID := 9001
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Decor", "Clocks"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "Type",
			AttributeID: 109,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
			PendingAttributeCandidates: []SheinPendingAttributeCandidate{{
				Name:        "Product Model",
				AttributeID: 9001,
				Important:   true,
			}},
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 27,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "Type",
				AttributeID: 109,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Name:             "Color",
					AttributeID:      27,
					AttributeValueID: &colorValueID,
				},
				SKUList: []SheinSKUDraft{{SupplierSKU: "SKU-1"}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs:         []PlatformVariant{{SKU: "SKU-1"}},
		}},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if !readiness.Ready {
		t.Fatalf("ready = false, want true when only important attributes are pending; readiness=%+v", readiness)
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "attribute_review" {
			t.Fatalf("unexpected attribute_review blocker for important-only candidate: %+v", readiness.BlockingItems)
		}
	}
}

func TestBuildSheinSubmitChecklistGroupsChecks(t *testing.T) {
	t.Parallel()

	checklist := sheinworkspace.BuildSubmitChecklist(&SheinSubmitReadiness{
		Checks: []SheinReadinessCheck{
			{Key: "category", Label: "类目骨架", Status: "blocking"},
			{Key: "request_draft", Label: "请求草稿", Status: "ready"},
			{Key: "manual_notes", Label: "人工备注", Status: "warning"},
		},
		BlockingItems: []SheinReadinessItem{
			{
				Key:             "category",
				SuggestedAction: "确认类目",
				Reason: &SheinReadinessReason{
					Code:     "category_unresolved",
					Category: "classification",
				},
				RepairHints: []SheinRepairHint{{
					Action:        "确认类目",
					Target:        "editor.category",
					Priority:      "high",
					EditorSection: "category",
					RevisionPath:  "shein.category_resolution",
					Patch: &SheinRepairPatchPayload{
						CategoryResolution: &SheinCategoryResolutionPatch{},
					},
					Skeleton: &SheinEditorRevisionSkeleton{
						Platform: "shein",
						Shein: &SheinRevisionInput{
							CategoryResolution: &SheinCategoryResolutionPatch{},
						},
					},
					Revision: &ApplyRevisionRequest{
						Platform: "shein",
						Shein: &SheinRevisionInput{
							CategoryResolution: &SheinCategoryResolutionPatch{},
						},
					},
					Validation: &SheinRepairValidationPreview{
						Valid:                  true,
						Status:                 "ready",
						AffectedSections:       []string{"category", "inspection"},
						CategoryPreviewEffects: []SheinEditorEffect{{Reason: "refresh category preview"}},
					},
				}},
			},
		},
		WarningItems: []SheinReadinessItem{
			{
				Key:             "manual_notes",
				SuggestedAction: "处理备注",
				Reason: &SheinReadinessReason{
					Code:     "manual_review_pending",
					Category: "manual_review",
				},
			},
		},
	}, sheinworkspace.SubmitChecklistGroupForKey)
	if checklist == nil {
		t.Fatal("expected checklist")
	}
	if len(checklist.Required) != 1 || checklist.Required[0].Key != "category" {
		t.Fatalf("required = %+v", checklist.Required)
	}
	if checklist.Required[0].SuggestedAction != "确认类目" {
		t.Fatalf("required action = %+v", checklist.Required[0])
	}
	if checklist.Required[0].Reason == nil || checklist.Required[0].Reason.Code != "category_unresolved" {
		t.Fatalf("required reason = %+v", checklist.Required[0].Reason)
	}
	if len(checklist.Required[0].RepairHints) != 1 || checklist.Required[0].RepairHints[0].Target != "editor.category" {
		t.Fatalf("required repair hints = %+v", checklist.Required[0].RepairHints)
	}
	if checklist.Required[0].RepairHints[0].EditorSection != "category" || checklist.Required[0].RepairHints[0].Patch == nil {
		t.Fatalf("required repair hint metadata = %+v", checklist.Required[0].RepairHints[0])
	}
	if checklist.Required[0].RepairHints[0].Skeleton == nil || checklist.Required[0].RepairHints[0].Revision == nil {
		t.Fatalf("required repair hint revision payload = %+v", checklist.Required[0].RepairHints[0])
	}
	if checklist.Required[0].RepairHints[0].Validation == nil || !checklist.Required[0].RepairHints[0].Validation.Valid {
		t.Fatalf("required repair hint validation = %+v", checklist.Required[0].RepairHints[0])
	}
	if len(checklist.Recommended) != 1 || checklist.Recommended[0].Key != "request_draft" {
		t.Fatalf("recommended = %+v", checklist.Recommended)
	}
	if len(checklist.Optional) != 1 || checklist.Optional[0].Key != "manual_notes" {
		t.Fatalf("optional = %+v", checklist.Optional)
	}
	if checklist.Optional[0].Reason == nil || checklist.Optional[0].Reason.Code != "manual_review_pending" {
		t.Fatalf("optional reason = %+v", checklist.Optional[0].Reason)
	}
}

func TestBuildSheinReadinessReason(t *testing.T) {
	t.Parallel()

	reason := buildSheinReadinessReason(&sheinworkspace.ReadinessReasonSpec{
		Code:     "category_unresolved",
		Category: "classification",
		Summary:  "missing category",
	})
	if reason == nil {
		t.Fatal("expected reason")
	}
	if reason.Code != "category_unresolved" || reason.Category != "classification" || reason.Summary != "missing category" {
		t.Fatalf("reason = %+v", reason)
	}
}

func TestWorkspaceBuildReadinessPatchPayloadSupportsListingKitAlias(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		Images:      &PlatformImageSet{MainImage: "https://cdn.example.com/main.jpg"},
		ReviewNotes: []string{"manual review"},
	}

	categoryPatch := sheinworkspace.BuildReadinessPatchPayload(pkg, "category")
	if categoryPatch == nil || categoryPatch.CategoryResolution == nil {
		t.Fatalf("category patch = %+v", categoryPatch)
	}

	imagesPatch := sheinworkspace.BuildReadinessPatchPayload(pkg, "images")
	if imagesPatch == nil || imagesPatch.Images == nil || imagesPatch.Images.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("images patch = %+v", imagesPatch)
	}

	notesPatch := sheinworkspace.BuildReadinessPatchPayload(pkg, "manual_notes")
	if notesPatch == nil || len(notesPatch.ReviewNotes) != 1 || notesPatch.ReviewNotes[0] != "manual review" {
		t.Fatalf("manual notes patch = %+v", notesPatch)
	}

	if patch := sheinworkspace.BuildReadinessPatchPayload(pkg, "request_draft"); patch != nil {
		t.Fatalf("request_draft patch = %+v, want nil", patch)
	}
}

func TestBuildSheinReadinessRepairHint(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	pkg := &SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
	}

	hint := buildSheinReadinessRepairHint(
		pkg,
		"确认类目",
		[]string{"shein.category_id"},
		sheinworkspace.ReadinessHintSpec{
			Priority:      "high",
			Target:        "editor.category",
			EditorSection: "category",
			EditorFocus:   []string{"category_id"},
			RevisionPath:  "shein.category_resolution",
			Description:   "确认类目",
		},
		sheinworkspace.BuildReadinessPatchPayload(pkg, "category"),
	)

	if hint.Target != "editor.category" || hint.Priority != "high" {
		t.Fatalf("hint = %+v", hint)
	}
	if len(hint.FieldPaths) != 1 || hint.FieldPaths[0] != "shein.category_id" {
		t.Fatalf("hint field paths = %+v", hint.FieldPaths)
	}
	if hint.Patch == nil || hint.Patch.CategoryResolution == nil {
		t.Fatalf("hint patch = %+v", hint.Patch)
	}
	if hint.Skeleton == nil || hint.Skeleton.Shein == nil || hint.Skeleton.Shein.CategoryResolution == nil {
		t.Fatalf("hint skeleton = %+v", hint.Skeleton)
	}
	if hint.Revision == nil || hint.Revision.Shein == nil || hint.Revision.Shein.CategoryResolution == nil {
		t.Fatalf("hint revision = %+v", hint.Revision)
	}
	if hint.Validation == nil || !hint.Validation.Valid {
		t.Fatalf("hint validation = %+v", hint.Validation)
	}
}

func TestBuildSheinRepairRevisionBundle(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	categoryID := 3001
	payload := &SheinRepairPatchPayload{
		CategoryResolution: &SheinCategoryResolutionPatch{
			CategoryID:    &categoryID,
			MatchedPath:   []string{"Home", "Kitchen", "Bottle"},
			ProductTypeID: &productTypeID,
		},
	}

	bundle := buildSheinRepairRevisionBundle("确认类目", payload)
	if bundle.input == nil || bundle.input.CategoryResolution == nil {
		t.Fatalf("bundle input = %+v", bundle.input)
	}
	if bundle.skeleton == nil || bundle.skeleton.Shein == nil || bundle.skeleton.Shein.CategoryResolution == nil {
		t.Fatalf("bundle skeleton = %+v", bundle.skeleton)
	}
	if bundle.request == nil || bundle.request.Shein == nil || bundle.request.Shein.CategoryResolution == nil {
		t.Fatalf("bundle request = %+v", bundle.request)
	}
	if bundle.skeleton.Reason != "repair: 确认类目" || bundle.request.Reason != bundle.skeleton.Reason {
		t.Fatalf("bundle reasons = skeleton:%+v request:%+v", bundle.skeleton, bundle.request)
	}
}

func TestCloneSheinRepairArtifacts(t *testing.T) {
	t.Parallel()

	categoryID := 3001
	patch := &SheinRepairPatchPayload{
		CategoryResolution: &SheinCategoryResolutionPatch{
			CategoryID: &categoryID,
		},
	}
	skeleton := &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Reason:   "repair: 确认类目",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				CategoryID: &categoryID,
			},
		},
	}
	request := &ApplyRevisionRequest{
		Platform: "shein",
		Reason:   "repair: 确认类目",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				CategoryID: &categoryID,
			},
		},
	}
	validation := &SheinRepairValidationPreview{
		Valid: true,
	}

	artifacts := cloneSheinRepairArtifacts(patch, skeleton, request, validation)
	if artifacts.patch == nil || artifacts.patch.CategoryResolution == nil {
		t.Fatalf("artifacts patch = %+v", artifacts.patch)
	}
	if artifacts.skeleton == nil || artifacts.skeleton.Shein == nil {
		t.Fatalf("artifacts skeleton = %+v", artifacts.skeleton)
	}
	if artifacts.request == nil || artifacts.request.Shein == nil {
		t.Fatalf("artifacts request = %+v", artifacts.request)
	}
	if artifacts.validation == nil || !artifacts.validation.Valid {
		t.Fatalf("artifacts validation = %+v", artifacts.validation)
	}
}

func TestWorkspaceCloneRepairPatchPayloadDeepCopiesListingKitAlias(t *testing.T) {
	t.Parallel()

	categoryID := 3001
	payload := &SheinRepairPatchPayload{
		CategoryResolution: &SheinCategoryResolutionPatch{
			CategoryID: &categoryID,
		},
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ReviewNotes: []string{"manual review"},
	}

	cloned := sheinworkspace.CloneRepairPatchPayload(payload)
	if cloned == nil {
		t.Fatal("CloneRepairPatchPayload() = nil, want clone")
	}
	if cloned.CategoryResolution == nil || cloned.CategoryResolution.CategoryID == nil || *cloned.CategoryResolution.CategoryID != 3001 {
		t.Fatalf("category resolution = %+v", cloned.CategoryResolution)
	}
	if cloned.Images == nil || cloned.Images.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("images = %+v", cloned.Images)
	}
	if len(cloned.ReviewNotes) != 1 || cloned.ReviewNotes[0] != "manual review" {
		t.Fatalf("review notes = %+v", cloned.ReviewNotes)
	}
}
