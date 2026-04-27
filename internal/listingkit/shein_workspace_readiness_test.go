package listingkit

import (
	"testing"

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

	checklist := buildSheinSubmitChecklist(&SheinSubmitReadiness{
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
	})
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
