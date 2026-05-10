package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if len(submitted.ProductAttributeList) != 2 {
		t.Fatalf("submitted product attributes = %#v, want deduped composition+material", submitted.ProductAttributeList)
	}
	if submitted.ProductAttributeList[0].AttributeID != 62 || submitted.ProductAttributeList[0].AttributeExtraValue != "100" {
		t.Fatalf("submitted composition attribute = %#v, want extra value 100", submitted.ProductAttributeList[0])
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
	})
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
	changed := normalizeSheinStudioSubmitSupplierSKUs(task, pkg)
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

func TestSubmitTaskMarksSaveDraftCodeZeroAsSuccess(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct.SPUName = "Draft Display Title Should Not Be Submitted"
	var submitted *sheinproduct.Product
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
	})
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
	})
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
	if submitted.SKCList[0].SKUS[0].CostInfo.CostPrice != "10.25" {
		t.Fatalf("submitted cost price = %q, want 10.25", submitted.SKCList[0].SKUS[0].CostInfo.CostPrice)
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				saveHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				saveResponse: &sheinproduct.SheinResponse{
					Code: "0",
					Msg:  "OK",
				},
			},
		},
		SheinContentOptimizer: contentAI,
	})
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
	if contentAI.calls == 0 {
		t.Fatal("expected content optimizer to be attempted")
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "publish"}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected publish payload to be captured")
	}
	if submitted.ImageInfo == nil || len(submitted.ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("product image_info = %+v, want empty for publish payload", submitted.ImageInfo)
	}
	if len(submitted.SKCList[0].ImageInfo.ImageInfoList) != 6 {
		t.Fatalf("skc image count = %d, want 6", len(submitted.SKCList[0].ImageInfo.ImageInfoList))
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType != 5 {
		t.Fatalf("skc square image type = %d, want 5", submitted.SKCList[0].ImageInfo.ImageInfoList[4].ImageType)
	}
	if submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType != 6 {
		t.Fatalf("skc color block image type = %d, want 6", submitted.SKCList[0].ImageInfo.ImageInfoList[5].ImageType)
	}
	if submitted.SKCList[0].SKUS[0].ImageInfo == nil || len(submitted.SKCList[0].SKUS[0].ImageInfo.ImageInfoList) != 0 {
		t.Fatalf("sku image_info = %+v, want empty for publish payload", submitted.SKCList[0].SKUS[0].ImageInfo)
	}
	if len(submitted.SiteList) != 1 || submitted.SiteList[0].MainSite != "shein" || len(submitted.SiteList[0].SubSiteList) != 1 || submitted.SiteList[0].SubSiteList[0] != "shein-us" {
		t.Fatalf("site_list = %+v, want shein/shein-us", submitted.SiteList)
	}
	submittedSKU := submitted.SKCList[0].SKUS[0]
	if len(submittedSKU.StockInfoList) != 1 || submittedSKU.StockInfoList[0].MerchantWarehouseCode != defaultSheinWarehouseCode || submittedSKU.StockInfoList[0].InventoryNum != 999 {
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
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
		},
		SheinImageAPIBuilder: stubSheinImageAPIBuilder{api: &stubSheinImageAPI{}},
	})
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
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
		SheinProductAPIBuilder: stubSheinProductAPIBuilder{
			api: stubSheinProductAPI{
				publishHook: func(product *sheinproduct.Product) {
					submitted = product
				},
				publishResponse: &sheinproduct.SheinResponse{Code: "0", Msg: "success", Info: sheinproduct.ResponseInfo{Success: true}},
			},
		},
		SheinTranslateAPIBuilder: stubSheinTranslateAPIBuilder{api: translateAPI},
		SheinContentOptimizer:    contentAI,
	})
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
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "en"); got != "Optimized Bottle Cap Metal Sign for Bar and Garage Decor" {
		t.Fatalf("english product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageNameList, "es"); got != "Spanish Optimized Bottle Cap Metal Sign for Bar and Garage Decor" {
		t.Fatalf("spanish product name = %q", got)
	}
	if got := findSheinLanguageContent(submitted.MultiLanguageDescList, "en"); got != "A durable decorative metal sign designed for wall display in bars, garages, game rooms, and home spaces." {
		t.Fatalf("english product description = %q", got)
	}
	if got := submitted.SKCList[0].MultiLanguageName.Name; got != "English 白色" {
		t.Fatalf("skc primary name = %q", got)
	}
	if len(translateAPI.calls) == 0 {
		t.Fatal("expected translate API to be called")
	}
	if contentAI.calls == 0 {
		t.Fatal("expected content optimizer to be called")
	}
}
