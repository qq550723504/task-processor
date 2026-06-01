package listingkit

import (
	"errors"
	"testing"
)

func TestApplyListingKitRevisionUpdatesSheinDraftAndPreview(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		Platforms: []string{"shein"},
		Shein: &SheinPackage{
			SpuName:       "Old Name",
			ProductNameEn: "Old Name",
			BrandName:     "Old Brand",
			Description:   "Old Description",
			ProductAttributes: []PlatformAttribute{
				{Name: "material", Value: "cotton"},
			},
			RequestDraft: &SheinRequestDraft{
				SpuName: "Old Name",
				MultiLanguageNameList: []LocalizedText{
					{Language: "en", Name: "Old Name"},
				},
				MultiLanguageDescList: []LocalizedText{
					{Language: "en", Name: "Old Description"},
				},
				ProductAttributeList: []PlatformAttribute{
					{Name: "material", Value: "cotton"},
				},
			},
			ReviewNotes: []string{"old note"},
		},
	}

	newName := "Updated Travel Bottle"
	newBrand := "Updated Brand"
	newDescription := "Updated description"
	categoryID := 5566

	err := applyListingKitRevision(result, &ApplyRevisionRequest{
		Platform: "shein",
		Actor:    "tester",
		Reason:   "manual fix",
		Shein: &SheinRevisionInput{
			SpuName:       &newName,
			ProductNameEn: &newName,
			BrandName:     &newBrand,
			Description:   &newDescription,
			CategoryID:    &categoryID,
			Images: &PlatformImageSet{
				MainImage: "https://cdn.example.com/updated-main.jpg",
				Gallery:   []string{"https://cdn.example.com/updated-gallery.jpg"},
			},
			ProductAttributes: []PlatformAttribute{
				{Name: "material", Value: "stainless steel"},
			},
			ReviewNotes: []string{"confirm category again"},
		},
	})
	if err != nil {
		t.Fatalf("apply revision: %v", err)
	}

	if result.Shein.SpuName != newName {
		t.Fatalf("spu_name = %q, want %q", result.Shein.SpuName, newName)
	}
	if result.Shein.RequestDraft == nil || result.Shein.RequestDraft.SpuName != newName {
		t.Fatalf("request draft spu_name = %+v, want %q", result.Shein.RequestDraft, newName)
	}
	if result.Shein.RequestDraft.ImageInfo == nil || result.Shein.RequestDraft.ImageInfo.MainImage != "https://cdn.example.com/updated-main.jpg" {
		t.Fatalf("request draft image info = %+v", result.Shein.RequestDraft.ImageInfo)
	}
	if len(result.Shein.RequestDraft.ProductAttributeList) != 1 || result.Shein.RequestDraft.ProductAttributeList[0].Value != "stainless steel" {
		t.Fatalf("request draft product attributes = %+v", result.Shein.RequestDraft.ProductAttributeList)
	}
	if result.Shein.PreviewProduct == nil || result.Shein.PreviewProduct.SPUName != newName {
		t.Fatalf("preview product = %+v, want updated name", result.Shein.PreviewProduct)
	}
	if result.Shein.PreviewProduct.CategoryID != categoryID {
		t.Fatalf("preview category id = %d, want %d", result.Shein.PreviewProduct.CategoryID, categoryID)
	}
	if result.Revision == nil || result.Revision.Platform != "shein" || result.Revision.UpdatedBy != "tester" {
		t.Fatalf("revision summary = %+v", result.Revision)
	}
	if result.Shein.Inspection == nil {
		t.Fatal("expected shein inspection to be rebuilt")
	}
}

func TestApplyListingKitRevisionPatchesSheinSaleAttributesAndSKUs(t *testing.T) {
	t.Parallel()

	skcValueID := 101
	skuValueID := 202
	result := &ListingKitResult{
		Platforms: []string{"shein"},
		Shein: &SheinPackage{
			SkcList: []SheinSKCPackage{{
				SupplierCode: "SKC-1",
				SkcName:      "Black",
				SaleName:     "Black",
				MainImageURL: "https://cdn.example.com/old-skc.jpg",
				SKUs: []PlatformVariant{{
					SKU:        "SKU-1",
					Stock:      10,
					Image:      "https://cdn.example.com/old-sku.jpg",
					Barcode:    "OLD",
					Attributes: map[string]string{"size": "M"},
				}},
			}},
			RequestDraft: &SheinRequestDraft{
				SKCList: []SheinSKCRequestDraft{{
					SupplierCode: "SKC-1",
					SkcName:      "Black",
					SaleName:     "Black",
					ImageInfo:    &SheinImageDraft{MainImage: "https://cdn.example.com/old-skc.jpg"},
					SKUList: []SheinSKUDraft{{
						SupplierSKU: "SKU-1",
						BasePrice:   "19.99",
						CostPrice:   "8.88",
						StockCount:  10,
						MainImage:   "https://cdn.example.com/old-sku.jpg",
						Barcode:     "OLD",
						Attributes:  map[string]string{"size": "M"},
					}},
				}},
			},
			SaleAttributeResolution: &SheinSaleAttributeResolution{
				Status: "resolved",
			},
		},
	}

	newStatus := "patched"
	newSource := "manual_revision"
	newPrimary := 501
	newSecondary := 502
	newSkcName := "Matte Black"
	newSkcImage := "https://cdn.example.com/new-skc.jpg"
	newBasePrice := "21.99"
	newStock := 26
	newBarcode := "NEW-BARCODE"
	newSkuImage := "https://cdn.example.com/new-sku.jpg"

	err := applyListingKitRevision(result, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				Status:               &newStatus,
				Source:               &newSource,
				PrimaryAttributeID:   &newPrimary,
				SecondaryAttributeID: &newSecondary,
				SelectionSummary:     []string{"颜色作为 SKC，尺码作为 SKU"},
			},
			SKCPatches: []SheinSKCRevisionPatch{{
				SupplierCode: "SKC-1",
				SkcName:      &newSkcName,
				MainImageURL: &newSkcImage,
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:            "skc",
					Name:             "Color",
					Value:            "Matte Black",
					AttributeID:      501,
					AttributeValueID: &skcValueID,
					MatchedBy:        "manual_patch",
				},
				SKUPatches: []SheinSKURevisionPatch{{
					SupplierSKU: "SKU-1",
					BasePrice:   &newBasePrice,
					StockCount:  &newStock,
					Barcode:     &newBarcode,
					MainImage:   &newSkuImage,
					SaleAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            "L",
						AttributeID:      502,
						AttributeValueID: &skuValueID,
						MatchedBy:        "manual_patch",
					}},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("apply shein patch revision: %v", err)
	}

	if result.Shein.RequestDraft.SKCList[0].SkcName != newSkcName {
		t.Fatalf("skc_name = %q, want %q", result.Shein.RequestDraft.SKCList[0].SkcName, newSkcName)
	}
	if result.Shein.RequestDraft.SKCList[0].ImageInfo == nil || result.Shein.RequestDraft.SKCList[0].ImageInfo.MainImage != newSkcImage {
		t.Fatalf("skc image info = %+v", result.Shein.RequestDraft.SKCList[0].ImageInfo)
	}
	if result.Shein.RequestDraft.SKCList[0].SaleAttribute == nil || result.Shein.RequestDraft.SKCList[0].SaleAttribute.AttributeID != 501 {
		t.Fatalf("skc sale attribute = %+v", result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
	if result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice != newBasePrice {
		t.Fatalf("sku base price = %q, want %q", result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice, newBasePrice)
	}
	if result.Shein.RequestDraft.SKCList[0].SKUList[0].StockCount != newStock {
		t.Fatalf("sku stock = %d, want %d", result.Shein.RequestDraft.SKCList[0].SKUList[0].StockCount, newStock)
	}
	if result.Shein.RequestDraft.SKCList[0].SKUList[0].Barcode != newBarcode {
		t.Fatalf("sku barcode = %q, want %q", result.Shein.RequestDraft.SKCList[0].SKUList[0].Barcode, newBarcode)
	}
	if len(result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes) != 1 || result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes[0].AttributeID != 502 {
		t.Fatalf("sku sale attributes = %+v", result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes)
	}
	if result.Shein.SaleAttributeResolution == nil || result.Shein.SaleAttributeResolution.PrimaryAttributeID != 501 || result.Shein.SaleAttributeResolution.SecondaryAttributeID != 502 {
		t.Fatalf("sale attribute resolution = %+v", result.Shein.SaleAttributeResolution)
	}
	if result.Shein.PreviewProduct == nil || result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID != 501 {
		t.Fatalf("preview skc sale attribute = %+v", result.Shein.PreviewProduct)
	}
	if len(result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList) != 1 || result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID != 502 {
		t.Fatalf("preview sku sale attributes = %+v", result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList)
	}
}

func TestApplyListingKitRevisionAppliesSaleAttributeResolutionWithoutSKCPatches(t *testing.T) {
	t.Parallel()

	skcValueID := 1001
	skuValueID := 2002
	status := "resolved"
	source := "manual_review"
	primaryAttributeID := 1001466
	secondaryAttributeID := 1001467

	result := &ListingKitResult{
		Platforms: []string{"shein"},
		Shein: &SheinPackage{
			SkcList: []SheinSKCPackage{{
				SupplierCode: "SKC-1",
				Attributes:   map[string]string{"color": "white"},
				SKUs: []PlatformVariant{{
					SKU:        "SKU-1",
					Attributes: map[string]string{"size": "M"},
				}},
			}},
			RequestDraft: &SheinRequestDraft{
				SKCList: []SheinSKCRequestDraft{{
					SupplierCode: "SKC-1",
					SKUList: []SheinSKUDraft{{
						SupplierSKU: "SKU-1",
						Attributes:  map[string]string{"size": "M"},
					}},
				}},
			},
		},
	}

	err := applyListingKitRevision(result, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				Status:               &status,
				Source:               &source,
				PrimaryAttributeID:   &primaryAttributeID,
				SecondaryAttributeID: &secondaryAttributeID,
				SKCAttributes: []SheinResolvedSaleAttribute{{
					Scope:            "skc",
					Name:             "Plug(Voltage)",
					Value:            "white",
					AttributeID:      primaryAttributeID,
					AttributeValueID: &skcValueID,
					MatchedBy:        "llm_sale_attribute_mapping",
				}},
				SKUAttributes: []SheinResolvedSaleAttribute{{
					Scope:            "sku",
					Name:             "Size",
					Value:            "M",
					AttributeID:      secondaryAttributeID,
					AttributeValueID: &skuValueID,
					MatchedBy:        "llm_sale_attribute_mapping",
				}},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply sale attribute resolution revision: %v", err)
	}

	if result.Shein.RequestDraft == nil || len(result.Shein.RequestDraft.SKCList) != 1 {
		t.Fatalf("request draft skc list = %+v", result.Shein.RequestDraft)
	}
	if result.Shein.RequestDraft.SKCList[0].SaleAttribute == nil || result.Shein.RequestDraft.SKCList[0].SaleAttribute.AttributeID != primaryAttributeID {
		t.Fatalf("request draft skc sale attribute = %+v", result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
	if got := result.Shein.RequestDraft.SKCList[0].SaleAttribute.AttributeValueID; got == nil || *got != skcValueID {
		t.Fatalf("request draft skc sale attribute value id = %+v", result.Shein.RequestDraft.SKCList[0].SaleAttribute)
	}
	if len(result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes) != 1 || result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes[0].AttributeID != secondaryAttributeID {
		t.Fatalf("request draft sku sale attributes = %+v", result.Shein.RequestDraft.SKCList[0].SKUList[0].SaleAttributes)
	}
	if result.Shein.PreviewProduct == nil || len(result.Shein.PreviewProduct.SKCList) != 1 {
		t.Fatalf("preview product = %+v", result.Shein.PreviewProduct)
	}
	if result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID != primaryAttributeID {
		t.Fatalf("preview skc sale attribute = %+v", result.Shein.PreviewProduct.SKCList[0].SaleAttribute)
	}
	if len(result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList) != 1 || result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID != secondaryAttributeID {
		t.Fatalf("preview sku sale attribute list = %+v", result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList)
	}
}

func TestApplyListingKitRevisionUsesValueAssignmentsToFillAllSaleAttributeSKUs(t *testing.T) {
	t.Parallel()

	skcValueID := 739
	skuMValueID := 2002
	skuLValueID := 2003
	status := "resolved"
	source := "manual_review"
	primaryAttributeID := 27
	secondaryAttributeID := 87
	primarySourceDimension := "Color"
	secondarySourceDimension := "Size"

	result := &ListingKitResult{
		Platforms: []string{"shein"},
		Shein: &SheinPackage{
			SkcList: []SheinSKCPackage{{
				SupplierCode: "SKC-1",
				Attributes:   map[string]string{"Color": "white"},
				SKUs: []PlatformVariant{
					{SKU: "SKU-1", Attributes: map[string]string{"Size": "M"}},
					{SKU: "MG8014086001", Attributes: map[string]string{"Size": "L"}},
				},
			}},
			RequestDraft: &SheinRequestDraft{
				SKCList: []SheinSKCRequestDraft{{
					SupplierCode: "SKC-1",
					SKUList: []SheinSKUDraft{
						{SupplierSKU: "SKU-1", Attributes: map[string]string{"Size": "M"}},
						{SupplierSKU: "MG8014086001", Attributes: map[string]string{"Size": "L"}},
					},
				}},
			},
		},
	}

	err := applyListingKitRevision(result, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			SaleAttributeResolution: &SheinSaleAttributeResolutionPatch{
				Status:                   &status,
				Source:                   &source,
				PrimaryAttributeID:       &primaryAttributeID,
				SecondaryAttributeID:     &secondaryAttributeID,
				PrimarySourceDimension:   &primarySourceDimension,
				SecondarySourceDimension: &secondarySourceDimension,
				SKCAttributes: []SheinResolvedSaleAttribute{{
					Scope:            "skc",
					Name:             "Color",
					Value:            "white",
					AttributeID:      primaryAttributeID,
					AttributeValueID: &skcValueID,
					MatchedBy:        "manual_review",
				}},
				SKUAttributes: []SheinResolvedSaleAttribute{{
					Scope:            "sku",
					Name:             "Size",
					Value:            "M",
					AttributeID:      secondaryAttributeID,
					AttributeValueID: &skuMValueID,
					MatchedBy:        "manual_review",
				}},
				SKCValueAssignments: map[string]SheinResolvedSaleAttribute{
					"white": {
						Scope:            "skc",
						Name:             "Color",
						Value:            "white",
						AttributeID:      primaryAttributeID,
						AttributeValueID: &skcValueID,
						MatchedBy:        "manual_review",
					},
				},
				SKUValueAssignments: map[string]SheinResolvedSaleAttribute{
					"m": {
						Scope:            "sku",
						Name:             "Size",
						Value:            "M",
						AttributeID:      secondaryAttributeID,
						AttributeValueID: &skuMValueID,
						MatchedBy:        "manual_review",
					},
					"l": {
						Scope:            "sku",
						Name:             "Size",
						Value:            "L",
						AttributeID:      secondaryAttributeID,
						AttributeValueID: &skuLValueID,
						MatchedBy:        "manual_review",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply sale attribute resolution revision with assignments: %v", err)
	}

	if len(result.Shein.RequestDraft.SKCList) != 1 || len(result.Shein.RequestDraft.SKCList[0].SKUList) != 2 {
		t.Fatalf("request draft skus = %+v", result.Shein.RequestDraft)
	}
	for _, sku := range result.Shein.RequestDraft.SKCList[0].SKUList {
		if len(sku.SaleAttributes) != 1 || sku.SaleAttributes[0].AttributeID != secondaryAttributeID {
			t.Fatalf("sku %s sale attributes = %+v", sku.SupplierSKU, sku.SaleAttributes)
		}
		if sku.SupplierSKU == "MG8014086001" {
			if got := sku.SaleAttributes[0].AttributeValueID; got == nil || *got != skuLValueID {
				t.Fatalf("sku %s sale attribute value id = %+v", sku.SupplierSKU, sku.SaleAttributes)
			}
		}
	}
	if result.Shein.SaleAttributeResolution == nil || result.Shein.SaleAttributeResolution.Status != "resolved" {
		t.Fatalf("sale attribute resolution = %+v", result.Shein.SaleAttributeResolution)
	}
}

func TestApplyListingKitRevisionPatchesSheinCategoryAndAttributeResolution(t *testing.T) {
	t.Parallel()

	attrValueID := 301
	categoryStatus := "resolved"
	categorySource := "manual_revision"
	categoryID := 7788
	productTypeID := 8899
	topCategoryID := 1010
	templateCount := 12
	unresolvedCount := 0

	result := &ListingKitResult{
		Platforms: []string{"shein"},
		Shein: &SheinPackage{
			SpuName:       "Bottle",
			ProductNameEn: "Bottle",
			ReviewNotes:   []string{"old note should stay"},
			RequestDraft: &SheinRequestDraft{
				SpuName: "Bottle",
			},
			SaleAttributeResolution: &SheinSaleAttributeResolution{
				Status:             "resolved",
				PrimaryAttributeID: 501,
			},
		},
	}

	err := applyListingKitRevision(result, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryResolution: &SheinCategoryResolutionPatch{
				Status:         &categoryStatus,
				Source:         &categorySource,
				MatchedPath:    []string{"Home", "Kitchen", "Bottle"},
				CategoryID:     &categoryID,
				CategoryIDList: []int{100, 200, categoryID},
				ProductTypeID:  &productTypeID,
				TopCategoryID:  &topCategoryID,
				ReviewNotes:    []string{},
			},
			AttributeResolution: &SheinAttributeResolutionPatch{
				Status:          &categoryStatus,
				Source:          &categorySource,
				CategoryID:      &categoryID,
				TemplateCount:   &templateCount,
				UnresolvedCount: &unresolvedCount,
				ResolvedAttributes: []SheinResolvedAttribute{{
					Name:                "material",
					Value:               "stainless steel",
					AttributeID:         7001,
					AttributeValueID:    &attrValueID,
					AttributeExtraValue: "stainless steel",
					MatchedBy:           "manual_revision",
				}},
				ReviewNotes: []string{},
			},
			ReviewNotes: []string{},
		},
	})
	if err != nil {
		t.Fatalf("apply category/attribute revision: %v", err)
	}

	if result.Shein.CategoryID != categoryID {
		t.Fatalf("category_id = %d, want %d", result.Shein.CategoryID, categoryID)
	}
	if result.Shein.ProductTypeID == nil || *result.Shein.ProductTypeID != productTypeID {
		t.Fatalf("product_type_id = %#v, want %d", result.Shein.ProductTypeID, productTypeID)
	}
	if result.Shein.CategoryResolution == nil || result.Shein.CategoryResolution.Status != "resolved" {
		t.Fatalf("category resolution = %+v", result.Shein.CategoryResolution)
	}
	if got := result.Shein.CategoryPath; len(got) != 3 || got[2] != "Bottle" {
		t.Fatalf("category path = %#v", got)
	}
	if result.Shein.AttributeResolution == nil || result.Shein.AttributeResolution.ResolvedCount != 1 {
		t.Fatalf("attribute resolution = %+v", result.Shein.AttributeResolution)
	}
	if len(result.Shein.ResolvedAttributes) != 1 || result.Shein.ResolvedAttributes[0].AttributeID != 7001 {
		t.Fatalf("resolved attributes = %+v", result.Shein.ResolvedAttributes)
	}
	if result.Shein.RequestDraft == nil || len(result.Shein.RequestDraft.ResolvedAttributes) != 1 {
		t.Fatalf("request draft resolved attributes = %+v", result.Shein.RequestDraft)
	}
	if result.Shein.PreviewProduct == nil || len(result.Shein.PreviewProduct.ProductAttributeList) != 1 {
		t.Fatalf("preview product attributes = %+v", result.Shein.PreviewProduct)
	}
	if result.Shein.Inspection == nil {
		t.Fatal("expected inspection")
	}
	if result.Shein.Inspection.Sections[0].Status != "resolved" {
		t.Fatalf("category inspection section = %+v", result.Shein.Inspection.Sections[0])
	}
	if result.Shein.Inspection.Sections[1].Status != "resolved" {
		t.Fatalf("attribute inspection section = %+v", result.Shein.Inspection.Sections[1])
	}
	if len(result.Shein.ReviewNotes) != 0 {
		t.Fatalf("review notes = %#v, want auto notes cleared", result.Shein.ReviewNotes)
	}
}

func TestRefreshSheinReviewStateDropsAutoNotesAfterManualResolution(t *testing.T) {
	t.Parallel()

	productTypeID := 9001
	pkg := &SheinPackage{
		CategoryID:    4004,
		CategoryPath:  []string{"Electronics", "Audio"},
		ProductTypeID: &productTypeID,
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 4004,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 501,
		},
		ReviewNotes: []string{
			"SHEIN 类目解析尚未命中真实 category_id，当前仍需要人工确认类目",
			"SHEIN 属性模板尚未完成真实 attribute_id 映射，当前仍需要人工确认属性",
			"SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格",
		},
	}

	refreshSheinReviewState(pkg)

	if len(pkg.ReviewNotes) != 0 {
		t.Fatalf("review notes = %#v, want auto notes cleared", pkg.ReviewNotes)
	}
	if pkg.Inspection == nil {
		t.Fatal("expected inspection")
	}
	if pkg.Inspection.NeedsReview {
		t.Fatalf("inspection needs_review = true, want false; inspection=%+v", pkg.Inspection)
	}
	for _, section := range pkg.Inspection.Sections {
		if len(section.Actions) != 0 {
			t.Fatalf("inspection section actions = %+v, want no unresolved actions", section.Actions)
		}
	}
}

func TestBuildSheinInspectionIncludesStructuredActionsWhenUnresolved(t *testing.T) {
	t.Parallel()

	valueID := 101
	pkg := &SheinPackage{
		CategoryPath: []string{"Home", "Kitchen"},
		CategoryName: "Kitchen",
		CategoryID:   456,
		CategoryIDList: []int{
			100, 456,
		},
		TopCategoryID: 100,
		ProductAttributes: []PlatformAttribute{
			{Name: "material", Value: "stainless steel"},
			{Name: "capacity", Value: "500ml"},
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:             "material",
			Value:            "stainless steel",
			AttributeID:      7001,
			AttributeValueID: &valueID,
		}},
		RequestDraft: &SheinRequestDraft{
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SkcName:      "Black",
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
					BasePrice:   "21.99",
					StockCount:  12,
				}},
			}},
		},
		CategoryResolution: &SheinCategoryResolution{
			Status: "partial",
			Source: "offline_match",
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:          "partial",
			Source:          "attribute_templates",
			ResolvedCount:   1,
			UnresolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status: "partial",
			Candidates: []SheinSaleAttributeCandidateInfo{
				{Name: "Color", AttributeID: 501},
			},
		},
	}

	inspection := buildSheinInspection(pkg)
	if inspection == nil {
		t.Fatal("expected inspection")
	}
	if len(inspection.Sections) != 3 {
		t.Fatalf("section count = %d, want 3", len(inspection.Sections))
	}
	for _, section := range inspection.Sections {
		if len(section.Actions) == 0 {
			t.Fatalf("section %s actions = %#v, want structured actions", section.Key, section.Actions)
		}
		if section.Actions[0].ActionType != "patch" {
			t.Fatalf("section %s action type = %q, want patch", section.Key, section.Actions[0].ActionType)
		}
	}
	if inspection.Sections[0].Actions[0].Payload["category_id"] != 456 {
		t.Fatalf("category action payload = %+v", inspection.Sections[0].Actions[0].Payload)
	}
	if inspection.Sections[0].Actions[0].Category == nil || inspection.Sections[0].Actions[0].Category.CategoryID != 456 {
		t.Fatalf("category action typed payload = %+v", inspection.Sections[0].Actions[0].Category)
	}
	pendingAttributes, ok := inspection.Sections[1].Actions[0].Payload["pending_attributes"].([]PlatformAttribute)
	if !ok || len(pendingAttributes) != 1 || pendingAttributes[0].Name != "capacity" {
		t.Fatalf("attribute action payload pending_attributes = %#v", inspection.Sections[1].Actions[0].Payload["pending_attributes"])
	}
	if inspection.Sections[1].Actions[0].Attributes == nil || len(inspection.Sections[1].Actions[0].Attributes.PendingAttributes) != 1 {
		t.Fatalf("attribute action typed payload = %+v", inspection.Sections[1].Actions[0].Attributes)
	}
	if inspection.Sections[2].Actions[0].Payload["candidate_count"] != 1 {
		t.Fatalf("sale attribute action payload = %+v", inspection.Sections[2].Actions[0].Payload)
	}
	if _, ok := inspection.Sections[2].Actions[0].Payload["skc_patches"]; !ok {
		t.Fatalf("sale attribute action payload missing skc_patches: %+v", inspection.Sections[2].Actions[0].Payload)
	}
	if inspection.Sections[2].Actions[0].Sale == nil || inspection.Sections[2].Actions[0].Sale.CandidateCount != 1 || len(inspection.Sections[2].Actions[0].Sale.SKCPatches) != 1 {
		t.Fatalf("sale action typed payload = %+v", inspection.Sections[2].Actions[0].Sale)
	}
}

func TestApplyListingKitRevisionRejectsMissingPayload(t *testing.T) {
	t.Parallel()

	err := applyListingKitRevision(&ListingKitResult{Shein: &SheinPackage{}}, &ApplyRevisionRequest{
		Platform: "shein",
	})
	if err == nil {
		t.Fatal("expected invalid revision request error")
	}
	if !errors.Is(err, ErrInvalidRevisionRequest) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRevisionRequest)
	}
}

func TestApplyListingKitRevisionReturnsFieldValidationErrors(t *testing.T) {
	t.Parallel()

	err := applyListingKitRevision(&ListingKitResult{Shein: &SheinPackage{}}, &ApplyRevisionRequest{
		Platform: "shein",
		Shein: &SheinRevisionInput{
			CategoryID: ptrInt(0),
			SKCPatches: []SheinSKCRevisionPatch{{
				SupplierCode: "",
				SKUPatches: []SheinSKURevisionPatch{{
					SupplierSKU: "",
					StockCount:  ptrInt(-1),
				}},
			}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *RevisionValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T, want RevisionValidationError", err)
	}
	if len(validationErr.Fields) < 3 {
		t.Fatalf("field errors = %+v, want multiple field errors", validationErr.Fields)
	}
}

func ptrInt(v int) *int {
	return &v
}
