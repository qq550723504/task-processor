package listingkit

import (
	"context"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSheinDraftBackedPricingReviewPreservesExistingDraftPrice(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "SKU-1",
					Currency:    "USD",
					CostPrice:   "48.80",
					BasePrice:   "21.99",
					SitePriceList: []sheinpub.SitePrice{{
						SubSite:   "US",
						BasePrice: "21.99",
						Currency:  "USD",
					}},
				}},
			}},
		},
	}

	review := buildSheinDraftBackedPricingReview(pkg, sheinpub.PricingRule{
		SourceCurrency:   "CNY",
		TargetCurrency:   "USD",
		ExchangeRate:     7.2,
		MarkupMultiplier: 2,
		MinimumPrice:     9.99,
		RoundTo:          0.01,
	}, nil)
	if review == nil || !review.Ready {
		t.Fatalf("review = %+v, want ready review", review)
	}
	if len(review.SKUPrices) != 1 {
		t.Fatalf("sku prices = %+v, want 1 item", review.SKUPrices)
	}
	if got := review.SKUPrices[0].CalculatedPrice; got != 21.99 {
		t.Fatalf("calculated price = %v, want 21.99", got)
	}
	if got := review.SKUPrices[0].FinalPrice; got != 21.99 {
		t.Fatalf("final price = %v, want 21.99", got)
	}
	if got := review.SKUPrices[0].Currency; got != "USD" {
		t.Fatalf("currency = %q, want USD", got)
	}
}

func TestBuildSheinDraftBackedPricingReviewNormalizesLegacyCNYDraftCurrency(t *testing.T) {
	t.Parallel()

	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "SKU-1",
					Currency:    "CNY",
					CostPrice:   "73.80",
					BasePrice:   "24.99",
					SitePriceList: []sheinpub.SitePrice{{
						SubSite:   "US",
						BasePrice: "24.99",
						Currency:  "CNY",
					}},
				}},
			}},
		},
	}

	review := buildSheinDraftBackedPricingReview(pkg, sheinpub.PricingRule{
		SourceCurrency:   "CNY",
		TargetCurrency:   "USD",
		ExchangeRate:     7.2,
		MarkupMultiplier: 2,
		MinimumPrice:     9.99,
		RoundTo:          0.01,
	}, nil)
	if review == nil || !review.Ready {
		t.Fatalf("review = %+v, want ready review", review)
	}
	if len(review.SKUPrices) != 1 {
		t.Fatalf("sku prices = %+v, want 1 item", review.SKUPrices)
	}
	if got := review.SKUPrices[0].FinalPrice; got != 24.99 {
		t.Fatalf("final price = %v, want 24.99", got)
	}
	if got := review.SKUPrices[0].Currency; got != "USD" {
		t.Fatalf("currency = %q, want USD", got)
	}
}

func TestSubmitTaskPreservesDraftPriceWhenPricingReviewMissing(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].CostPrice = "48.80"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "21.99"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "21.99",
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 21.99,
		Currency:  "USD",
	}}

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
		withDefaultTestSheinImageAPI(),
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
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 21.99 {
		t.Fatalf("submitted base price = %v, want 21.99", got)
	}
}

func TestSubmitTaskNormalizesLegacyCNYDraftCurrencyToUSD(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].CostPrice = "73.80"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].Currency = "CNY"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "24.99"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "24.99",
		Currency:  "CNY",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 24.99,
		Currency:  "CNY",
	}}

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
		withDefaultTestSheinImageAPI(),
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
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 24.99 {
		t.Fatalf("submitted base price = %v, want 24.99", got)
	}
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].Currency; got != "USD" {
		t.Fatalf("submitted price currency = %q, want USD", got)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo == nil || submitted.SKCList[0].SKUS[0].CostInfo.CostPrice != "24.99" {
		t.Fatalf("submitted cost info = %+v, want cost price 24.99", submitted.SKCList[0].SKUS[0].CostInfo)
	}
}

func TestUpdateSheinFinalDraftPreservesExistingDraftPrice(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = nil
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].CostPrice = "48.80"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "21.99"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "21.99",
		Currency:  "USD",
	}}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(repo))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	confirmed := true
	if _, err := svc.UpdateSheinFinalDraft(context.Background(), task.ID, &SheinFinalDraftUpdateRequest{Confirmed: &confirmed}); err != nil {
		t.Fatalf("update final draft: %v", err)
	}

	saved, err := repo.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got := saved.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice; got != "21.99" {
		t.Fatalf("saved draft base price = %q, want 21.99", got)
	}
	if saved.Result.Shein.Pricing == nil || len(saved.Result.Shein.Pricing.SKUPrices) != 1 {
		t.Fatalf("saved pricing = %+v, want one sku review", saved.Result.Shein.Pricing)
	}
	if got := saved.Result.Shein.Pricing.SKUPrices[0].FinalPrice; got != 21.99 {
		t.Fatalf("saved final price = %v, want 21.99", got)
	}
	if repo.mutateCalls == 0 {
		t.Fatal("expected UpdateSheinFinalDraft to persist through mutate task result")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0 when transaction mutation is available", repo.saveCalls)
	}
}

func TestPreviewSheinPriceApplyToTaskUsesMutation(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	svc, err := NewService(newTestServiceConfig(repo))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	review, err := svc.PreviewSheinPrice(context.Background(), task.ID, &SheinPricePreviewRequest{
		ApplyToTask: true,
		ManualOverrides: map[string]float64{
			"SKU-1": 29.99,
		},
	})
	if err != nil {
		t.Fatalf("preview shein price: %v", err)
	}
	if review == nil || len(review.SKUPrices) != 1 || review.SKUPrices[0].FinalPrice != 29.99 {
		t.Fatalf("review = %+v, want persisted manual override", review)
	}
	if repo.mutateCalls == 0 {
		t.Fatal("expected PreviewSheinPrice(apply_to_task=true) to persist through mutate task result")
	}
	if repo.saveCalls != 0 {
		t.Fatalf("save calls = %d, want 0 when transaction mutation is available", repo.saveCalls)
	}
}

func TestSubmitTaskUsesFinalDraftManualOverrideWhenReadyPricingExists(t *testing.T) {
	t.Parallel()

	repo := &stubSubmitRepo{}
	task := makeReadySheinTask()
	currentSKU := task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SupplierSKU
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency: "CNY",
			TargetCurrency: "USD",
		},
		Ready: true,
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: currentSKU,
			CostCNY:     91.8,
			FinalPrice:  91.8,
			Currency:    "USD",
		}},
		ManualOverrides: map[string]float64{currentSKU: 99.99},
	}
	task.Result.Shein.FinalDraft = &sheinpub.FinalDraft{
		Confirmed:            true,
		MainImageURL:         "https://oss.shuomiai.com/listingkit/pricing-main.png",
		FinalImageOrder:      []string{"https://oss.shuomiai.com/listingkit/pricing-main.png"},
		ManualPriceOverrides: map[string]float64{currentSKU: 99.99},
	}
	task.Result.Shein.RequestDraft.ImageInfo = &SheinImageDraft{
		MainImage: "https://oss.shuomiai.com/listingkit/pricing-main.png",
		Gallery:   []string{"https://oss.shuomiai.com/listingkit/pricing-main.png"},
	}
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].CostPrice = "91.80"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "91.80"
	task.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "91.80",
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.ImageInfo = sheinImageInfo([]string{"https://oss.shuomiai.com/listingkit/pricing-main.png"})
	task.Result.Shein.PreviewProduct.SKCList[0].ImageInfo = *sheinImageInfo([]string{"https://oss.shuomiai.com/listingkit/pricing-main.png"})
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 91.8,
		Currency:  "USD",
	}}
	task.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].CostInfo = &sheinproduct.CostInfo{
		CostPrice: "91.80",
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
		withDefaultTestSheinImageAPI(),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if _, err := svc.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{Platform: "shein", Action: "save_draft", ConfirmedFinal: true}); err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if submitted == nil {
		t.Fatal("expected save draft payload to be captured")
	}
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 99.99 {
		t.Fatalf("submitted base price = %v, want 99.99 from final draft manual override", got)
	}
}

func TestApplyDefaultSheinPricingUsesPublishedPriceCache(t *testing.T) {
	t.Parallel()

	store := &submitResolutionCacheStore{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "USD",
			ExchangeRate:     7.2,
			MarkupMultiplier: 2,
			MinimumPrice:     9.99,
			RoundTo:          0.01,
		},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU:     "SKU-1",
			SupplierCode:    "SKC-1",
			CostCNY:         10,
			CalculatedPrice: 19.99,
			FinalPrice:      27.99,
			Currency:        "USD",
			Manual:          true,
		}},
		Ready: true,
	}
	svc, err := NewService(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = store
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.(*service).rememberSheinSubmittedPricing(task, "publish")

	fresh := makeReadySheinTask()
	fresh.Result.Shein.Pricing = nil
	fresh.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "19.99"
	fresh.Result.Shein.RequestDraft.SKCList[0].SKUList[0].SitePriceList = []sheinpub.SitePrice{{
		SubSite:   "US",
		BasePrice: "19.99",
		Currency:  "USD",
	}}
	fresh.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList = []sheinproduct.PriceInfo{{
		SubSite:   "US",
		BasePrice: 19.99,
		Currency:  "USD",
	}}

	svc.(*service).applyDefaultSheinPricing(fresh.Request, fresh.Result.Shein)

	if fresh.Result.Shein.Pricing == nil || len(fresh.Result.Shein.Pricing.SKUPrices) != 1 {
		t.Fatalf("pricing review = %+v, want cached review", fresh.Result.Shein.Pricing)
	}
	if got := fresh.Result.Shein.Pricing.SKUPrices[0].FinalPrice; got != 27.99 {
		t.Fatalf("final price = %v, want cached 27.99", got)
	}
	if got := fresh.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice; got != "27.99" {
		t.Fatalf("draft base price = %q, want cached 27.99", got)
	}
	if got := fresh.Result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 27.99 {
		t.Fatalf("preview base price = %v, want cached 27.99", got)
	}
}

func TestApplyDefaultSheinPricingRemapsCachedManualOverrideToCurrentSKU(t *testing.T) {
	t.Parallel()

	store := &submitResolutionCacheStore{}
	svc, err := NewService(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = store
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	req := &GenerateRequest{
		Platforms:    []string{"shein"},
		SheinStoreID: 869,
	}
	fresh := &sheinpub.Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8006062"},
			{Name: "variant_sku", Value: "MG8006062004"},
			{Name: "sku", Value: "MG8006062"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8006062004-EF926739",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8006062004-EF926739",
					Attributes:  map[string]string{"source_sds_sku": "MG8006062004"},
					CostPrice:   "143",
					BasePrice:   "143",
					Currency:    "CNY",
					SitePriceList: []sheinpub.SitePrice{{
						SubSite:   "US",
						BasePrice: "143",
						Currency:  "CNY",
					}},
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{
			SKCList: []sheinproduct.SKC{{
				SKUS: []sheinproduct.SKU{{
					SupplierSKU: "MG8006062004-EF926739",
					PriceInfoList: []sheinproduct.PriceInfo{{
						SubSite:   "US",
						BasePrice: 143,
						Currency:  "CNY",
					}},
				}},
			}},
		},
	}

	key := sheinPricingCacheKey(buildSheinPublishRequest(req), fresh, svc.(*service).currentSheinPricingRule())
	if key == "" {
		t.Fatal("pricing cache key is empty")
	}
	cachedReview := &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "USD",
			ExchangeRate:     7.2,
			MarkupMultiplier: 2,
			MinimumPrice:     9.99,
			RoundTo:          0.01,
		},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU:     "MG8006062004-V96697-TB9B6431C-RA8CD5E-E5BADD24",
			SupplierCode:    "MG8006062004-9D6E1EC4",
			CostCNY:         143,
			CalculatedPrice: 143,
			FinalPrice:      143,
			Currency:        "USD",
		}},
		ManualOverrides: map[string]float64{
			"MG8006062004-V96697-TAB634DC8-R14A9AC-92B5A7B8": 82,
		},
		Ready: true,
	}
	if err := store.SaveResolutionCache(context.Background(), &sheinpub.SheinResolutionCacheEntry{
		StoreID:        "869",
		CacheKind:      sheinpub.ResolutionCacheKindPricing,
		CacheKey:       key,
		ShortKey:       sheinPricingShortKey(key),
		Source:         "manual_cache",
		Manual:         true,
		SourceIdentity: sheinPricingSourceIdentity(fresh),
		ResolutionJSON: mustMarshalSheinPricingReview(cachedReview),
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	svc.(*service).applyDefaultSheinPricing(req, fresh)

	if fresh.Pricing == nil || len(fresh.Pricing.SKUPrices) != 1 {
		t.Fatalf("pricing review = %+v, want one cached price", fresh.Pricing)
	}
	if got := fresh.Pricing.SKUPrices[0].SupplierSKU; got != "MG8006062004-EF926739" {
		t.Fatalf("pricing sku = %q, want current draft SKU", got)
	}
	if got := fresh.Pricing.SKUPrices[0].FinalPrice; got != 82 {
		t.Fatalf("final price = %v, want remapped manual override 82", got)
	}
	if got := fresh.Pricing.ManualOverrides["MG8006062004-EF926739"]; got != 82 {
		t.Fatalf("manual overrides = %#v, want current SKU override 82", fresh.Pricing.ManualOverrides)
	}
	if got := fresh.RequestDraft.SKCList[0].SKUList[0].BasePrice; got != "82.00" {
		t.Fatalf("draft base price = %q, want 82.00", got)
	}
	if got := fresh.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 82 {
		t.Fatalf("preview base price = %v, want 82", got)
	}
}

func TestApplyDefaultSheinPricingSkipsExistingReadyPricing(t *testing.T) {
	t.Parallel()

	pkg := makeReadySheinTask().Result.Shein
	pkg.Pricing = &sheinpub.PricingReview{
		Ready: true,
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU: "SKU-1",
			FinalPrice:  33.33,
			Currency:    "USD",
		}},
	}

	if reason := sheinPricingCacheSkipReason(pkg); reason != "existing_ready_pricing" {
		t.Fatalf("skip reason = %q, want existing_ready_pricing", reason)
	}
}

func TestLoadSheinPricingCacheRejectsNonApplicableCache(t *testing.T) {
	t.Parallel()

	store := &submitResolutionCacheStore{}
	task := makeReadySheinTask()
	task.Result.Shein.Pricing = &sheinpub.PricingReview{
		RuleSnapshot: &sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "USD",
			ExchangeRate:     7.2,
			MarkupMultiplier: 2,
			MinimumPrice:     9.99,
			RoundTo:          0.01,
		},
		SKUPrices: []sheinpub.SKUPriceReview{{
			SupplierSKU:     "SKU-1",
			SupplierCode:    "SKC-1",
			CostCNY:         10,
			CalculatedPrice: 19.99,
			FinalPrice:      27.99,
			Currency:        "USD",
			Manual:          true,
		}},
		Ready: true,
	}
	svc, err := NewService(newTestServiceConfig(
		&stubSubmitRepo{},
		withTestConfig(func(cfg *ServiceConfig) {
			cfg.Shein.SheinResolutionCacheStore = store
		}),
	))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	svc.(*service).rememberSheinSubmittedPricing(task, "publish")

	fresh := makeReadySheinTask()
	fresh.Result.Shein.Pricing = nil
	fresh.Result.Shein.RequestDraft.SKCList[0].SKUList[0].CostPrice = "88.80"
	fresh.Result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice = "19.99"

	cached := svc.(*service).loadSheinPricingCache(fresh.Request, fresh.Result.Shein)
	if cached != nil {
		t.Fatalf("cached review = %+v, want nil for non-applicable cache", cached)
	}
}

func TestSheinPricingCacheKeyUsesStableSDSIdentifiers(t *testing.T) {
	t.Parallel()

	req := &sheinpub.BuildRequest{SheinStoreID: 869}
	first := &sheinpub.Package{
		SpuName:        "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn:  "Vintage Metal Bottle Cap Wall Sign - Professional Lazy Expert Sloth Print",
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"美国本地直发", "生活用品", "铁板画"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-8A78E611",
					CostPrice:   "91.80",
					Currency:    "USD",
				}},
			}},
		},
	}
	second := &sheinpub.Package{
		SpuName:        "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn:  "Professional Lazy Expert Metal Bottle Cap Wall Sign, Funny Sloth Gaming Decor",
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"美国本地直发", "生活用品", "铁板画"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-8A78E611",
					CostPrice:   "91.80",
					Currency:    "USD",
				}},
			}},
		},
	}

	firstKey := sheinPricingCacheKey(req, first, sheinpub.PricingRule{})
	secondKey := sheinPricingCacheKey(req, second, sheinpub.PricingRule{})
	if firstKey == "" || secondKey == "" {
		t.Fatalf("pricing cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("pricing cache key drifted for stable SDS identifiers: first=%s second=%s", firstKey, secondKey)
	}
}

func TestSheinPricingCacheKeyIgnoresDecoratedSubmitSupplierSKUsForSDS(t *testing.T) {
	t.Parallel()

	req := &sheinpub.BuildRequest{SheinStoreID: 869}
	first := &sheinpub.Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"美国本地直发", "生活用品", "铁板画"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001-8A78E611",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-V124111-T838E0EBE-R84A7E-8A78E611",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
					CostPrice:   "91.80",
					Currency:    "USD",
				}},
			}},
		},
	}
	second := &sheinpub.Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"美国本地直发", "生活用品", "铁板画"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001-8A78E611",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-8A78E611",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
					CostPrice:   "91.80",
					Currency:    "USD",
				}},
			}},
		},
	}

	firstKey := sheinPricingCacheKey(req, first, sheinpub.PricingRule{})
	secondKey := sheinPricingCacheKey(req, second, sheinpub.PricingRule{})
	if firstKey == "" || secondKey == "" {
		t.Fatalf("pricing cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("pricing cache key drifted across decorated submit SKUs: first=%s second=%s", firstKey, secondKey)
	}
}

func TestSheinPricingCacheKeyNormalizesLegacyCNYDraftCurrency(t *testing.T) {
	t.Parallel()

	req := &sheinpub.BuildRequest{SheinStoreID: 869}
	rule := sheinpub.PricingRule{SourceCurrency: "CNY", TargetCurrency: "USD"}
	first := &sheinpub.Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001-E22DF523",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-E22DF523",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
					CostPrice:   "91.80",
					Currency:    "CNY",
					SitePriceList: []sheinpub.SitePrice{{
						SubSite:   "US",
						BasePrice: "91.80",
						Currency:  "CNY",
					}},
				}},
			}},
		},
	}
	second := &sheinpub.Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		CategoryPath:   []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "MG8014062001-E22DF523",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "MG8014062001-E22DF523",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
					CostPrice:   "91.80",
					Currency:    "USD",
					SitePriceList: []sheinpub.SitePrice{{
						SubSite:   "US",
						BasePrice: "91.80",
						Currency:  "USD",
					}},
				}},
			}},
		},
	}

	firstKey := sheinPricingCacheKey(req, first, rule)
	secondKey := sheinPricingCacheKey(req, second, rule)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("pricing cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("pricing cache key drifted across legacy CNY draft currency: first=%s second=%s", firstKey, secondKey)
	}
}
