package listingkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/listingadmin"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/submitprep"
)

func TestSubmitTaskRebuildsNormalizedProductAttributesFromPackage(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	compositionValueID := 526
	materialValueID := 3473050
	task.Result.Shein.ResolvedAttributes = []SheinResolvedAttribute{
		{
			Name:             "Composition",
			Value:            "Polyester",
			AttributeID:      62,
			AttributeValueID: &compositionValueID,
			AttributeType:    3,
		},
		{
			Name:             "Material",
			Value:            "100%涤纶",
			AttributeID:      160,
			AttributeValueID: &materialValueID,
			AttributeType:    4,
		},
		{
			Name:             "Material",
			Value:            "Made with polyester",
			AttributeID:      160,
			AttributeValueID: &materialValueID,
			AttributeType:    4,
		},
	}
	task.Result.Shein.PreviewProduct.ProductAttributeList = []sheinproduct.ProductAttribute{
		{AttributeID: 160, AttributeValueID: &materialValueID},
		{AttributeID: 160, AttributeValueID: &materialValueID},
		{AttributeID: 62, AttributeValueID: &compositionValueID},
	}
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true},
				},
			},
		}),
		withTestSheinImageAPIBuilder(stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if len(submitted.ProductAttributeList) != 3 {
		t.Fatalf("submitted product attributes = %#v, want composition plus preserved material values", submitted.ProductAttributeList)
	}
	if submitted.ProductAttributeList[0].AttributeID != 62 || submitted.ProductAttributeList[0].AttributeExtraValue != "100" {
		t.Fatalf("submitted composition attribute = %#v, want extra value 100", submitted.ProductAttributeList[0])
	}
	if submitted.ProductAttributeList[1].AttributeID != 160 || submitted.ProductAttributeList[1].AttributeExtraValue != "100" {
		t.Fatalf("submitted first material attribute = %#v, want preserved numeric extra value", submitted.ProductAttributeList[1])
	}
	if submitted.ProductAttributeList[2].AttributeID != 160 || submitted.ProductAttributeList[2].AttributeExtraValue != "" {
		t.Fatalf("submitted second material attribute = %#v, want preserved text-only material value", submitted.ProductAttributeList[2])
	}
}

func TestSubmitTaskNormalizesLegacyStudioSupplierSKUs(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	oldSKU := "MG8014186001-D7E68190"
	sizeImage := "https://img.shein.com/uploaded/size-map.jpg"
	blackValueID := 2493
	whiteValueID := 2494
	sizeValueID := 267
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "D7E68190"},
		SDS: &SDSSyncOptions{
			ProductSKU: "MG8014186001",
			StyleID:    "D7E68190",
			Variants: []SDSSyncVariantOption{
				{VariantID: 101, Color: "black", Size: "均码"},
				{VariantID: 102, Color: "white", Size: "均码"},
			},
		},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: "https://img.shein.com/uploaded/default-main.jpg",
		Gallery: []string{
			"https://img.shein.com/uploaded/default-main.jpg",
			"https://img.shein.com/uploaded/default-gallery.jpg",
			sizeImage,
		},
	}
	task.Result.Shein.RequestDraft.SKCList = []SheinSKCRequestDraft{
		{
			SupplierCode: "BLACK",
			SaleAttribute: &SheinResolvedSaleAttribute{
				Scope: "skc", Name: "Color", Value: "black", AttributeID: 27, AttributeValueID: &blackValueID,
			},
			ImageInfo: &SheinImageDraft{MainImage: "https://img.shein.com/uploaded/black.jpg"},
			SKUList: []SheinSKUDraft{{
				SupplierSKU: oldSKU,
				Currency:    "USD",
				CostPrice:   "10.00",
				BasePrice:   "19.99",
				StockCount:  20,
				SitePriceList: []sheinpub.SitePrice{{
					SubSite: "US", BasePrice: "19.99", Currency: "USD",
				}},
				SaleAttributes: []SheinResolvedSaleAttribute{{
					Scope: "sku", Name: "Size", Value: "均码", AttributeID: 87, AttributeValueID: &sizeValueID,
				}},
				Attributes: map[string]string{
					"Color": "black",
					"Size":  "均码",
				},
			}},
		},
		{
			SupplierCode: "WHITE",
			SaleAttribute: &SheinResolvedSaleAttribute{
				Scope: "skc", Name: "Color", Value: "white", AttributeID: 27, AttributeValueID: &whiteValueID,
			},
			ImageInfo: &SheinImageDraft{MainImage: "https://img.shein.com/uploaded/white.jpg"},
			SKUList: []SheinSKUDraft{{
				SupplierSKU: oldSKU,
				Currency:    "USD",
				CostPrice:   "11.00",
				BasePrice:   "21.99",
				StockCount:  20,
				SitePriceList: []sheinpub.SitePrice{{
					SubSite: "US", BasePrice: "21.99", Currency: "USD",
				}},
				SaleAttributes: []SheinResolvedSaleAttribute{{
					Scope: "sku", Name: "Size", Value: "均码", AttributeID: 87, AttributeValueID: &sizeValueID,
				}},
				Attributes: map[string]string{
					"Color": "white",
					"Size":  "均码",
				},
			}},
		},
	}
	task.Result.Shein.SkcList = []SheinSKCPackage{
		{SupplierCode: "BLACK", SkcName: "black", SaleName: "black", MainImageURL: "https://img.shein.com/uploaded/black.jpg", SKUs: []common.Variant{{SKU: oldSKU, Attributes: map[string]string{"Color": "black", "Size": "均码"}}}},
		{SupplierCode: "WHITE", SkcName: "white", SaleName: "white", MainImageURL: "https://img.shein.com/uploaded/white.jpg", SKUs: []common.Variant{{SKU: oldSKU, Attributes: map[string]string{"Color": "white", "Size": "均码"}}}},
	}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{
		"https://img.shein.com/uploaded/default-main.jpg",
		"https://img.shein.com/uploaded/default-gallery.jpg",
		sizeImage,
	})
	task.Result.Shein.PreviewProduct.SKCList = []sheinproduct.SKC{
		{
			SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 27, AttributeValueID: blackValueID},
			ImageInfo:     sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{ImageType: 1, ImageSort: 1, ImageURL: "https://img.shein.com/uploaded/black.jpg"}}},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: oldSKU,
				CostInfo:    &sheinproduct.CostInfo{CostPrice: "10.00", Currency: "USD"},
				PriceInfoList: []sheinproduct.PriceInfo{{
					SubSite: "US", BasePrice: 19.99, Currency: "USD",
				}},
				StockInfoList:     []sheinproduct.StockInfo{{MerchantWarehouseCode: "US", InventoryNum: 20}},
				SaleAttributeList: []sheinproduct.SaleAttribute{{AttributeID: 87, AttributeValueID: sizeValueID}},
			}},
		},
		{
			SaleAttribute: sheinproduct.SaleAttribute{AttributeID: 27, AttributeValueID: whiteValueID},
			ImageInfo:     sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{ImageType: 1, ImageSort: 1, ImageURL: "https://img.shein.com/uploaded/white.jpg"}}},
			SKUS: []sheinproduct.SKU{{
				SupplierSKU: oldSKU,
				CostInfo:    &sheinproduct.CostInfo{CostPrice: "11.00", Currency: "USD"},
				PriceInfoList: []sheinproduct.PriceInfo{{
					SubSite: "US", BasePrice: 21.99, Currency: "USD",
				}},
				StockInfoList:     []sheinproduct.StockInfo{{MerchantWarehouseCode: "US", InventoryNum: 20}},
				SaleAttributeList: []sheinproduct.SaleAttribute{{AttributeID: 87, AttributeValueID: sizeValueID}},
			}},
		},
	}
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:       true,
		MainImageURL:    "https://img.shein.com/uploaded/default-main.jpg",
		FinalImageOrder: []string{"https://img.shein.com/uploaded/default-main.jpg", "https://img.shein.com/uploaded/default-gallery.jpg", sizeImage},
		ImageRoleOverrides: map[string]string{
			sizeImage: "size_map",
		},
		ManualPriceOverrides: map[string]float64{oldSKU: 25.55},
	}
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		Ready:           true,
		ManualOverrides: map[string]float64{oldSKU: 25.55},
		SKUPrices: []sheinpub.SKUPriceReview{
			{SupplierSKU: oldSKU, FinalPrice: 25.55, Currency: "USD"},
			{SupplierSKU: oldSKU, FinalPrice: 25.55, Currency: "USD"},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	got := []string{
		submitted.SKCList[0].SKUS[0].SupplierSKU,
		submitted.SKCList[1].SKUS[0].SupplierSKU,
	}
	want := []string{
		"MG8014186001-V101-TSUBMITTA-D7E68190",
		"MG8014186001-V102-TSUBMITTA-D7E68190",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("submitted supplier skus = %#v, want %#v", got, want)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if saved.Result == nil || saved.Result.Shein == nil || saved.Result.Shein.FinalDraft == nil {
		t.Fatalf("saved shein final draft = %+v", saved.Result)
	}
	overrides := saved.Result.Shein.FinalDraft.ManualPriceOverrides
	if len(overrides) != 2 || overrides[want[0]] != 25.55 || overrides[want[1]] != 25.55 {
		t.Fatalf("final draft overrides = %#v, want fan-out to both new skus", overrides)
	}
	if _, exists := overrides[oldSKU]; exists {
		t.Fatalf("final draft overrides still contains legacy sku %q", oldSKU)
	}
	if saved.Result.Shein.Pricing == nil || len(saved.Result.Shein.Pricing.SKUPrices) != 2 {
		t.Fatalf("saved pricing = %+v", saved.Result.Shein.Pricing)
	}
	if saved.Result.Shein.Pricing.SKUPrices[0].SupplierSKU != want[0] || saved.Result.Shein.Pricing.SKUPrices[1].SupplierSKU != want[1] {
		t.Fatalf("pricing sku prices = %#v, want normalized sku order", saved.Result.Shein.Pricing.SKUPrices)
	}
}

func TestSubmitTaskNormalizesSingleStudioSupplierSKUWithTaskDiscriminator(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.ID = "fe7413d2-ac75-4c97-be0f-800a40dffa00"
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "D7E68190"},
		SDS: &SDSSyncOptions{
			ProductSKU:   "MG8014186001",
			VariantSKU:   "MG8014186001",
			StyleID:      "D7E68190",
			VariantColor: "black",
			VariantSize:  "均码",
		},
	}
	task.Result.TaskID = task.ID
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Attributes = map[string]string{
		"Color":          "black",
		"Size":           "均码",
		"source_sds_sku": "MG8014186001",
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.SkcList[0].SKUs[0].SKU = "MG8014186001-D7E68190"
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:            true,
		MainImageURL:         "https://img.shein.com/uploaded/default-main.jpg",
		FinalImageOrder:      []string{"https://img.shein.com/uploaded/default-main.jpg"},
		ManualPriceOverrides: map[string]float64{"MG8014186001-D7E68190": 13.56},
	}
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		Ready:           true,
		ManualOverrides: map[string]float64{"MG8014186001-D7E68190": 13.56},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: "MG8014186001-D7E68190",
			FinalPrice:  13.56,
			Currency:    "USD",
		}},
	}
	pkg := &sheinpub.Package{
		RequestDraft:   task.Result.Shein.RequestDraft,
		PreviewProduct: task.Result.Shein.PreviewProduct,
		SkcList:        task.Result.Shein.SkcList,
		FinalDraft:     task.Result.Shein.FinalDraft,
		Pricing:        task.Result.Shein.Pricing,
	}
	changed := normalizeSheinStudioSubmitSupplierSKUs(task, pkg, "")
	if !changed {
		t.Fatal("expected single-variant supplier sku normalization to change payload")
	}
	wantSKU := "MG8014186001-BLACK-V1-TFE7413D2-D7E68190"
	if got := pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU; got != wantSKU {
		t.Fatalf("request draft supplier sku = %q, want %q", got, wantSKU)
	}
	if got := pkg.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU; got != wantSKU {
		t.Fatalf("preview supplier sku = %q, want %q", got, wantSKU)
	}
	if got := pkg.SkcList[0].SKUs[0].SKU; got != wantSKU {
		t.Fatalf("package supplier sku = %q, want %q", got, wantSKU)
	}
	if pkg.FinalDraft.ManualPriceOverrides[wantSKU] != 13.56 {
		t.Fatalf("final draft overrides = %#v, want remapped key %q", pkg.FinalDraft.ManualPriceOverrides, wantSKU)
	}
	if _, exists := pkg.FinalDraft.ManualPriceOverrides["MG8014186001-D7E68190"]; exists {
		t.Fatalf("final draft overrides still contains legacy sku")
	}
	if len(pkg.Pricing.SKUPrices) != 1 || pkg.Pricing.SKUPrices[0].SupplierSKU != wantSKU {
		t.Fatalf("pricing sku prices = %#v, want remapped single sku", pkg.Pricing.SKUPrices)
	}
}

func TestSubmitTaskNormalizesStudioSupplierSKUWithRequestDiscriminator(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.ID = "fe7413d2-ac75-4c97-be0f-800a40dffa00"
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "D7E68190"},
		SDS: &SDSSyncOptions{
			ProductSKU: "MG8014186001",
			StyleID:    "D7E68190",
			Variants: []SDSSyncVariantOption{
				{VariantID: 101, Color: "black", Size: "均码"},
			},
		},
	}
	task.Result.TaskID = task.ID
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Attributes = map[string]string{
		"Color":          "black",
		"Size":           "均码",
		"source_sds_sku": "MG8014186001",
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU = "MG8014186001-D7E68190"
	task.Result.Shein.SkcList[0].SKUs[0].SKU = "MG8014186001-D7E68190"
	pkg := &sheinpub.Package{
		RequestDraft:   task.Result.Shein.RequestDraft,
		PreviewProduct: task.Result.Shein.PreviewProduct,
		SkcList:        task.Result.Shein.SkcList,
	}

	changed := normalizeSheinStudioSubmitSupplierSKUs(task, pkg, "debug-18095-run-1")
	if !changed {
		t.Fatal("expected supplier sku normalization to change payload")
	}
	got := pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU
	want := "MG8014186001-V101-TFE7413D2-RDEBUG180-D7E68190"
	if got != want {
		t.Fatalf("request-scoped supplier sku = %q, want %q", got, want)
	}
}

func TestNormalizeSheinStudioSubmitSupplierSKUsReconcilesStalePricingKeys(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.ID = "f79b3e36-d6b9-440d-b098-ae4e83a4fe04"
	task.Request.Options = &GenerateOptions{
		SheinStudio: &SheinStudioOptions{StyleID: "167D3B4C"},
		SDS: &SDSSyncOptions{
			ProductSKU: "MG8014062001",
			StyleID:    "167D3B4C",
			Variants: []SDSSyncVariantOption{
				{VariantID: 124111, Color: "white", Size: "1PCS", VariantSKU: "MG8014062001"},
			},
		},
	}
	currentSKU := "MG8014062001-V124111-TF79B3E36-RF898D-167D3B4C"
	staleSKU := "MG8014062001-V124111-TF79B3E36-R622A0-167D3B4C"
	task.Result.TaskID = task.ID
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU = currentSKU
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Currency = "CNY"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "91.8"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite: "US", BasePrice: "91.8", Currency: "CNY",
	}}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Attributes = map[string]string{
		"Color":          "white",
		"Size":           "1PCS",
		"source_sds_sku": "MG8014062001",
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU = currentSKU
	task.Result.Shein.SkcList[0].SKUs[0].SKU = currentSKU
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		ManualPriceOverrides: map[string]float64{staleSKU: 39.99},
	}
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		Ready:           true,
		ManualOverrides: map[string]float64{staleSKU: 39.99},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: staleSKU,
			FinalPrice:  39.99,
			Currency:    "USD",
			Manual:      true,
		}},
	}

	pkg := &sheinpub.Package{
		RequestDraft:   task.Result.Shein.RequestDraft,
		PreviewProduct: task.Result.Shein.PreviewProduct,
		SkcList:        task.Result.Shein.SkcList,
		FinalDraft:     task.Result.Shein.FinalDraft,
		Pricing:        task.Result.Shein.Pricing,
	}
	changed := normalizeSheinStudioSubmitSupplierSKUs(task, pkg, "f898d3ad-b007-43b8-bc90-968fe494dbea")
	if !changed {
		t.Fatal("expected stale pricing keys to be reconciled")
	}
	if got := pkg.Pricing.SKUPrices[0].SupplierSKU; got != currentSKU {
		t.Fatalf("pricing supplier sku = %q, want %q", got, currentSKU)
	}
	if _, exists := pkg.Pricing.ManualOverrides[currentSKU]; !exists {
		t.Fatalf("pricing manual overrides = %#v, want remapped current sku", pkg.Pricing.ManualOverrides)
	}
	if _, exists := pkg.FinalDraft.ManualPriceOverrides[currentSKU]; !exists {
		t.Fatalf("final draft manual overrides = %#v, want remapped current sku", pkg.FinalDraft.ManualPriceOverrides)
	}
}

func TestSubmitTaskMarksSaveDraftCodeZeroAsSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SPUName = "Draft Display Title Should Not Be Submitted"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
					Info: sheinproduct.ResponseInfo{
						SPUName: "SPU-123",
					},
				},
			},
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if submitted.SPUName != "" {
		t.Fatalf("submitted draft spu_name = %q, want empty for new SHEIN product", submitted.SPUName)
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein)
	}
}

func TestSubmitTaskReappliesReadyPricingBeforeSubmit(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency: "CNY",
			TargetCurrency: "USD",
		},
		Ready: true,
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU,
			FinalPrice:  25.55,
			Currency:    "USD",
		}},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "19.99"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "19.99",
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 19.99,
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].CostInfo = &sheinproduct.CostInfo{
		CostPrice: "73.8",
		Currency:  "USD",
	}

	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if len(submitted.SKCList) == 0 || len(submitted.SKCList[0].SKUS) == 0 || len(submitted.SKCList[0].SKUS[0].PriceInfoList) == 0 {
		t.Fatalf("submitted price info = %+v", submitted.SKCList)
	}
	if submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice != 25.55 {
		t.Fatalf("submitted base price = %v, want 25.55", submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo == nil || submitted.SKCList[0].SKUS[0].CostInfo.Currency != "USD" {
		t.Fatalf("submitted cost info = %+v, want currency USD", submitted.SKCList[0].SKUS[0].CostInfo)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo.CostPrice != "25.55" {
		t.Fatalf("submitted cost price = %q, want 25.55", submitted.SKCList[0].SKUS[0].CostInfo.CostPrice)
	}
}

func TestSubmitTaskSaveDraftDoesNotFailWhenContentOptimizerFails(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	var submitted *sheinproduct.Product
	contentAI := &stubSheinContentAI{err: errors.New("upstream EOF")}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinContentOptimizer = contentAI
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	preview, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if contentAI.calls != 0 {
		t.Fatalf("content optimizer calls = %d, want 0 because submit prep should preserve reviewed content", contentAI.calls)
	}
	if preview.Shein == nil || preview.Shein.Submission == nil || preview.Shein.Submission.LastStatus != "success" {
		t.Fatalf("submission = %+v", preview.Shein)
	}
}

func TestSubmitTaskNormalizesSheinPublishOnlyFields(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SiteList = nil
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{
		"https://cdn.example.com/main.jpg",
		"https://cdn.example.com/gallery-1.jpg",
	})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{
		"https://cdn.example.com/main.jpg",
		"https://cdn.example.com/gallery-1.jpg",
		"https://cdn.example.com/gallery-2.jpg",
		"https://cdn.example.com/gallery-3.jpg",
	})
	sku := &task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0]
	sku.ImageInfo = sheinImageInfo([]string{"https://cdn.example.com/main.jpg"})
	sku.Length = ""
	sku.Width = ""
	sku.Height = ""
	sku.LengthUnit = ""
	sku.StockInfoList = nil
	stockCount := 999
	sku.StockCount = &stockCount
	sku.QuantityInfo = nil
	sku.PackageType = 0
	sku.PriceInfoList = []sheinproduct.PriceInfo{{SubSite: "US", BasePrice: 19.99, Currency: "USD"}}
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != 3 {
		t.Fatalf("product image_info = %+v, want normalized SPU images without top-level color block", submitted.ImageInfo)
	}
	if submitted.ImageInfo.ImageInfoList[2].ImageType != 5 {
		t.Fatalf("spu square image type = %d, want 5", submitted.ImageInfo.ImageInfoList[2].ImageType)
	}
	for _, image := range submitted.ImageInfo.ImageInfoList {
		if image.ImageType == 6 {
			t.Fatalf("spu image_info = %+v, want no top-level image_type=6", submitted.ImageInfo)
		}
	}
	if submitted.Extra.SwitchToSPUPic {
		t.Fatalf("extra = %+v, want switch_to_spu_pic disabled to match shein-listing submit payload", submitted.Extra)
	}
	if submitted.Extra.FromPageID == nil || *submitted.Extra.FromPageID != "product_publish" {
		t.Fatalf("from_page_id = %+v, want product_publish", submitted.Extra.FromPageID)
	}
	if submitted.Extra.UseCVTransformImage || submitted.Extra.TransformCVSizeImage {
		t.Fatalf("extra cv flags = %+v, want both false for direct publish payload", submitted.Extra)
	}
	if submitted.SourceSystem != "listingkit" {
		t.Fatalf("source_system = %q, want listingkit", submitted.SourceSystem)
	}
	if submitted.SupplierCode == "" {
		t.Fatal("supplier_code is empty, want non-empty top-level publish lookup code")
	}
	if len(submitted.SKCList[0].ImageInfo.ImageInfoList) != 6 {
		t.Fatalf("skc image count = %d, want 6", len(submitted.SKCList[0].ImageInfo.ImageInfoList))
	}
	if submitted.SKCList[0].SupplierCode != nil {
		t.Fatalf("skc supplier_code = %#v, want nil to match shein-listing direct publish payload", submitted.SKCList[0].SupplierCode)
	}
	if submitted.SKCList[0].SaleAttribute.PreFillSpec {
		t.Fatalf("skc sale_attribute.pre_fill_spec = true, want false to match shein-listing direct publish payload")
	}
	if submitted.SKCList[0].SiteDetailImageInfoList == nil {
		t.Fatal("site_detail_image_info_list = nil, want empty array to match shein-listing direct publish payload")
	}
	if submitted.SKCList[0].SiteSpecImageInfoList == nil {
		t.Fatal("site_spec_image_info_list = nil, want empty array to match shein-listing direct publish payload")
	}
	if submitted.SKCList[0].SKCScopeAttributeList == nil {
		t.Fatal("skc_scope_attribute_list = nil, want empty array to match shein-listing direct publish payload")
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType != 5 {
		t.Fatalf("skc square image type = %d, want 5", submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType)
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType != 6 {
		t.Fatalf("skc color block image type = %d, want 6", submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType)
	}
	if submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 1 {
		t.Fatalf("sku image_info = %+v, want preserved SKU image for publish payload", submitted.SKCList[0].SKUS[0].ImageInfo)
	}
	if len(submitted.SiteList) != 1 || submitted.SiteList[0].MainSite != "shein" || len(submitted.SiteList[0].SubSiteList) != 1 || submitted.SiteList[0].SubSiteList[0] != "shein-us" {
		t.Fatalf("site_list = %+v, want shein/shein-us", submitted.SiteList)
	}
	submittedSKU := submitted.SKCList[0].SKUS[0]
	if len(submittedSKU.StockInfoList) != 1 || submittedSKU.StockInfoList[0].MerchantWarehouseCode != "DEFAULT" || submittedSKU.StockInfoList[0].InventoryNum != 999 {
		t.Fatalf("stock_info_list = %+v", submittedSKU.StockInfoList)
	}
	if submittedSKU.StockCount != nil {
		t.Fatalf("stock_count = %v, want nil when stock_info_list is populated", *submittedSKU.StockCount)
	}
	if submittedSKU.QuantityInfo == nil || submittedSKU.QuantityInfo.Quantity == nil || *submittedSKU.QuantityInfo.Quantity != 1 ||
		submittedSKU.QuantityInfo.QuantityType == nil || *submittedSKU.QuantityInfo.QuantityType != 1 ||
		submittedSKU.QuantityInfo.QuantityUnit == nil || *submittedSKU.QuantityInfo.QuantityUnit != 1 {
		t.Fatalf("quantity_info = %+v, want 1/1/1", submittedSKU.QuantityInfo)
	}
	if submittedSKU.PackageType != 3 {
		t.Fatalf("package_type = %d, want 3", submittedSKU.PackageType)
	}
	if submittedSKU.StopPurchase != 1 {
		t.Fatalf("stop_purchase = %d, want 1 to match shein-listing direct publish payload", submittedSKU.StopPurchase)
	}
	if submittedSKU.CompetingCostPriceImages == nil {
		t.Fatal("competing_cost_price_images = nil, want empty array to match shein-listing direct publish payload")
	}
	if len(submittedSKU.PriceInfoList) != 1 || submittedSKU.PriceInfoList[0].SubSite != "shein-us" {
		t.Fatalf("price_info_list = %+v, want sub_site shein-us", submittedSKU.PriceInfoList)
	}
	if submittedSKU.Length == "" || submittedSKU.Width == "" || submittedSKU.Height == "" || submittedSKU.LengthUnit == "" {
		t.Fatalf("dimensions not normalized: length=%q width=%q height=%q unit=%q", submittedSKU.Length, submittedSKU.Width, submittedSKU.Height, submittedSKU.LengthUnit)
	}
	if submittedSKU.WeightUnit != "g" {
		t.Fatalf("weight_unit = %q, want g", submittedSKU.WeightUnit)
	}
}

func TestSubmitTaskNormalizesNilSlicesToEmptyArraysForSheinPublish(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.BrandSeriesList = nil
	task.Result.Shein.PreviewProduct.MultiLanguageMakeupIngredientList = nil
	task.Result.Shein.PreviewProduct.ProductVideoList = nil
	task.Result.Shein.PreviewProduct.PartInfoList = nil
	task.Result.Shein.PreviewProduct.PLMPatternIDList = nil
	task.Result.Shein.PreviewProduct.SizeAttributeList = nil
	task.Result.Shein.PreviewProduct.BackSizeAttributeList = nil
	task.Result.Shein.PreviewProduct.ImageInfo = &sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{
			{ImageType: 1, ImageSort: 1, ImageURL: "https://cdn.example.com/main.jpg"},
			{ImageType: 2, ImageSort: 2, ImageURL: "https://cdn.example.com/gallery-1.jpg"},
		},
	}
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = sheinproduct.ImageInfo{
		ImageInfoList: []sheinproduct.ImageDetail{
			{ImageType: 1, ImageSort: 1, ImageURL: "https://cdn.example.com/main.jpg"},
			{ImageType: 2, ImageSort: 2, ImageURL: "https://cdn.example.com/gallery-1.jpg"},
		},
	}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList = nil
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].CompetingCostPriceImages = nil
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{Success: true, SPUName: "SPU-123"},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.BrandSeriesList == nil {
		t.Fatal("brand_series_list = nil, want []")
	}
	if submitted.MultiLanguageMakeupIngredientList == nil {
		t.Fatal("multi_language_makeup_ingredient_list = nil, want []")
	}
	if submitted.ProductVideoList == nil {
		t.Fatal("product_video_list = nil, want []")
	}
	if submitted.PartInfoList == nil {
		t.Fatal("part_info_list = nil, want []")
	}
	if submitted.PLMPatternIDList == nil {
		t.Fatal("plm_pattern_id_list = nil, want []")
	}
	if submitted.SizeAttributeList == nil {
		t.Fatal("size_attribute_list = nil, want []")
	}
	if submitted.BackSizeAttributeList == nil {
		t.Fatal("back_size_attribute_list = nil, want []")
	}
	if submitted.SKCList[0].SKUS[0].SaleAttributeList == nil {
		t.Fatal("sku.sale_attribute_list = nil, want []")
	}
	if submitted.SKCList[0].SKUS[0].CompetingCostPriceImages == nil {
		t.Fatal("sku.competing_cost_price_images = nil, want []")
	}
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) == 0 || submitted.ImageInfo.ImageInfoList[0].PSTypes == nil {
		t.Fatal("spu image ps_types = nil, want []")
	}
	if len(submitted.SKCList[0].ImageInfo.ImageInfoList) == 0 || submitted.SKCList[0].ImageInfo.ImageInfoList[0].PSTypes == nil {
		t.Fatal("skc image ps_types = nil, want []")
	}

	payload, err := json.Marshal(submitted)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	jsonText := string(payload)
	for _, needle := range []string{
		`"brand_series_list":null`,
		`"multi_language_makeup_ingredient_list":null`,
		`"product_video_list":null`,
		`"part_info_list":null`,
		`"plm_pattern_id_list":null`,
		`"size_attribute_list":null`,
		`"back_size_attribute_list":null`,
		`"sale_attribute_list":null`,
		`"competing_cost_price_images":null`,
		`"ps_types":null`,
	} {
		if strings.Contains(jsonText, needle) {
			t.Fatalf("publish payload still contains %s: %s", needle, jsonText)
		}
	}
}

func TestSubmitTaskNormalizesSheinWeightToGrams(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].Weight = 0.35
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].WeightUnit = "kg"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "success",
					Info: sheinproduct.ResponseInfo{
						Success: true,
					},
				},
			},
		}),
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	submittedSKU := submitted.SKCList[0].SKUS[0]
	if submittedSKU.Weight != 350 {
		t.Fatalf("weight = %v, want 350g", submittedSKU.Weight)
	}
	if submittedSKU.WeightUnit != "g" {
		t.Fatalf("weight_unit = %q, want g", submittedSKU.WeightUnit)
	}
}

func TestSubmitTaskTranslatesChineseSheinContentBeforePublish(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Country = "US"
	task.Result.Shein.PreviewProduct.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "啤酒盖铁板画"}}
	task.Result.Shein.PreviewProduct.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: "适用于酒吧和车库装饰。"}}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: "白色"}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "白色"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var submitted *sheinproduct.Product
	translateAPI := &stubSheinTranslateAPI{}
	contentAI := &stubSheinContentAI{
		response: `{"title":"Optimized Bottle Cap Metal Sign for Bar and Garage Decor","description":"A durable decorative metal sign designed for wall display in bars, garages, game rooms, and home spaces."}`,
	}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "success", Info: sheinproduct.ResponseInfo{Success: true}},
			},
		}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinTranslateAPIBuilder = stubSheinTranslateAPIBuilder{api: translateAPI}
			cfg.Shein.SheinContentOptimizer = contentAI
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "en"); got != "English 啤酒盖铁板画" {
		t.Fatalf("english product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "es"); got != "Spanish 啤酒盖铁板画" {
		t.Fatalf("spanish product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageDescList, "en"); got != "English 适用于酒吧和车库装饰" {
		t.Fatalf("english product description = %q", got)
	}
	if got := submitted.SKCList[0].MultiLanguageName.Name; !strings.EqualFold(got, "english 啤酒盖铁板画 白色") {
		t.Fatalf("skc primary name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "spanish 啤酒盖铁板画 白色") {
		t.Fatalf("spanish skc name = %q", got)
	}
	if len(translateAPI.calls) == 0 {
		t.Fatal("expected translate API to be called")
	}
	if contentAI.calls != 0 {
		t.Fatalf("content optimizer calls = %d, want 0 because submit prep should preserve reviewed content", contentAI.calls)
	}
}

func TestSubmitTaskAddsRegionalTranslationsForEnglishSheinContent(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Country = "US"
	task.Result.Shein.PreviewProduct.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "Door curtain for home decor"}}
	task.Result.Shein.PreviewProduct.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: "A soft door curtain for bedrooms and living rooms."}}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: "white"}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "white"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var submitted *sheinproduct.Product
	translateAPI := &stubSheinTranslateAPI{}
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "success", Info: sheinproduct.ResponseInfo{Success: true}},
			},
		}),
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinTranslateAPIBuilder = stubSheinTranslateAPIBuilder{api: translateAPI}
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "es"); got != "Spanish Door curtain for home decor" {
		t.Fatalf("spanish product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageDescList, "es"); got != "Spanish A soft door curtain for bedrooms and living rooms." {
		t.Fatalf("spanish product description = %q", got)
	}
	if got := findSheinLanguageContent(submitted.SKCList[0].MultiLanguageNameList, "es"); !strings.EqualFold(got, "spanish door curtain for home decor white") {
		t.Fatalf("spanish skc name = %q", got)
	}
	if len(translateAPI.calls) == 0 {
		t.Fatal("expected translate API to be called")
	}
}

func TestSubmitTaskCleansSheinSensitiveWordsBeforePublish(t *testing.T) {
	t.Parallel()

	restoreRepo := submitprep.SetSensitiveWordRepository(&stubListingkitSensitiveWordRepository{
		pages: map[int64][]listingadmin.SensitiveWord{
			373211199677923496: {{
				TenantID: 373211199677923496,
				Language: "en",
				Word:     "whimsy",
				Status:   1,
			}},
		},
	})
	defer restoreRepo()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Request.Country = ""
	task.Result.Shein.PreviewProduct.MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}}
	task.Result.Shein.PreviewProduct.MultiLanguageDescList = []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy decor for a relaxed room."}}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageName = sheinproduct.LanguageContent{Language: "en", Name: "whimsy white"}
	task.Result.Shein.PreviewProduct.SKCList[0].MultiLanguageNameList = []sheinproduct.LanguageContent{{Language: "en", Name: "whimsy white"}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	var submitted *sheinproduct.Product
	svc, err := NewService(newTestServiceConfig(
		repo,
		withTestSheinProductAPIBuilder(stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "success", Info: sheinproduct.ResponseInfo{Success: true}},
			},
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if strings.Contains(strings.ToLower(findSheinLanguageContent(submitted.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english product name still contains whimsy: %q", findSheinLanguageContent(submitted.MultiLanguageNameList, "en"))
	}
	if strings.Contains(strings.ToLower(findSheinLanguageContent(submitted.MultiLanguageDescList, "en")), "whimsy") {
		t.Fatalf("english product description still contains whimsy: %q", findSheinLanguageContent(submitted.MultiLanguageDescList, "en"))
	}
	if strings.Contains(strings.ToLower(findSheinLanguageContent(submitted.SKCList[0].MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("english skc name still contains whimsy: %q", findSheinLanguageContent(submitted.SKCList[0].MultiLanguageNameList, "en"))
	}
}
