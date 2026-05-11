package listingkit

import (
	"context"
	"testing"

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
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice; got != 24.99 {
		t.Fatalf("submitted base price = %v, want 24.99", got)
	}
	if got := submitted.SKCList[0].SKUS[0].PriceInfoList[0].Currency; got != "USD" {
		t.Fatalf("submitted price currency = %q, want USD", got)
	}
	if submitted.SKCList[0].SKUS[0].CostInfo == nil || submitted.SKCList[0].SKUS[0].CostInfo.CostPrice != "10.25" {
		t.Fatalf("submitted cost info = %+v, want cost price 10.25", submitted.SKCList[0].SKUS[0].CostInfo)
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

	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: stubSubmitProductService{},
	})
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
}
