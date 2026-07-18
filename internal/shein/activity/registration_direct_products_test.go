package activity

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/marketing"

	"github.com/sirupsen/logrus"
)

func TestRegisterPromotionProductsUsesProvidedPriceAndStockWhenAttributesAreMissing(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0.15,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg260603174864291057873",
			Stock:               999,
			SupplyPrice:         12,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   48,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("config list = %+v, want one config from provided product", result)
	}
	config := result.Request.ConfigList[0]
	if config.Skc != "sg260603174864291057873" {
		t.Fatalf("config skc = %q", config.Skc)
	}
	if config.ActStock != 499 || config.ReservedActStock != 999 {
		t.Fatalf("stock config = %+v, want act 499 reserved 999", config)
	}
	if config.DropRate <= 0 {
		t.Fatalf("drop rate = %d, want positive", config.DropRate)
	}
	if api.saved == nil || len(api.saved.ConfigList) != 1 {
		t.Fatalf("saved request = %+v, want one config", api.saved)
	}
}

func TestPromotionGoodsFromProductSnapshotsKeepsRetailOnlySKUForDiscountPricing(t *testing.T) {
	goods := promotionGoodsFromProductSnapshots([]marketing.SkcInfo{{
		Skc: "skc-retail-only", Stock: 1,
		SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 29.9, IsAvailable: true}},
		SkuPriceInfoList:  []marketing.SkuSitePriceInfo{{SkuCode: "sku-1"}},
	}}, "USD")
	if len(goods) != 1 || len(goods[0].SkuInfoList) != 1 {
		t.Fatalf("goods = %+v, want retail-only SKU retained for discount pricing", goods)
	}
}

func TestBuildCalculateRequestForPromotionProductsRequiresDirectSKUPrices(t *testing.T) {
	service := &activityRegistrationServiceImpl{}
	goods := []marketing.PromotionGoodsData{{
		Skc:           "skc-direct-prices",
		USSupplyPrice: 99,
		SkuInfoList: []marketing.PromotionSkuInfo{
			{Sku: "sku-complete"},
			{Sku: "sku-missing-cost"},
			{Sku: "sku-eur-retail"},
			{Sku: "sku-eur-cost"},
			{Sku: "sku-disabled-retail"},
		},
	}}
	products := []marketing.SkcInfo{{
		Skc: "skc-direct-prices", Stock: 1, SupplyPrice: 99,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-complete", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 100, IsAvailable: true}}},
			{SkuCode: "sku-missing-cost", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 80, IsAvailable: true}}},
			{SkuCode: "sku-eur-retail", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "EUR", SalePrice: 500, IsAvailable: true}}},
			{SkuCode: "sku-eur-cost", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 60, IsAvailable: true}}},
			{SkuCode: "sku-disabled-retail", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 70, IsAvailable: false}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-complete", CostPrice: 50, Currency: "USD"},
			{SkuCode: "sku-eur-retail", CostPrice: 5, Currency: "USD"},
			{SkuCode: "sku-eur-cost", CostPrice: 1, Currency: "EUR"},
			{SkuCode: "sku-disabled-retail", CostPrice: 10, Currency: "USD"},
		},
	}}

	for _, tc := range []struct {
		name          string
		mode          string
		wantSKUPrices map[string]float64
	}{
		{name: "discount", mode: "DISCOUNT", wantSKUPrices: map[string]float64{"sku-complete": 100, "sku-missing-cost": 80, "sku-eur-cost": 60}},
		{name: "profit", mode: "PROFIT", wantSKUPrices: map[string]float64{"sku-complete": 100}},
		{name: "breakeven", mode: "BREAKEVEN", wantSKUPrices: map[string]float64{"sku-complete": 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := service.buildCalculateRequestForPromotionProducts(TimeLimitedDiscountConfig{
				PriceMode: tc.mode, Currency: "USD", DiscountRate: 0.2,
			}, goods, products)
			if req == nil || len(req.SkcInfoList) != 1 || len(req.SkcInfoList[0].SkuInfoList) != len(tc.wantSKUPrices) {
				t.Fatalf("calculate request = %+v, want SKU prices %+v", req, tc.wantSKUPrices)
			}
			for _, sku := range req.SkcInfoList[0].SkuInfoList {
				wantPrice, ok := tc.wantSKUPrices[sku.SkuCode]
				if !ok || sku.ProductPrice != wantPrice || sku.DiscountValue <= 0 {
					t.Fatalf("calculated SKU = %+v, want direct USD retail price and positive activity price", sku)
				}
			}
		})
	}
}

func TestRegisterPromotionProductsAllowsZeroMinProfitRateWithProvidedProductSnapshot(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-zero-profit",
			Stock:               10,
			SupplyPrice:         20,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("config list = %+v, want one config when min profit is zero", result)
	}
	config := result.Request.ConfigList[0]
	if config.Skc != "sg-zero-profit" {
		t.Fatalf("config skc = %q", config.Skc)
	}
	if config.DropRate != 1 {
		t.Fatalf("drop rate = %d, want minimum SHEIN drop rate 1", config.DropRate)
	}
}

func TestRegisterPromotionProductsUsesBreakevenDropRateWithProvidedProductSnapshot(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:            870,
			ActivityPriceMode:  "BREAKEVEN",
			ActivityStockRatio: 0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-breakeven",
			Stock:               10,
			SupplyPrice:         18,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("config list = %+v, want one breakeven config", result)
	}
	config := result.Request.ConfigList[0]
	if config.DropRate != 40 {
		t.Fatalf("drop rate = %d, want breakeven drop rate 40", config.DropRate)
	}
	if config.ActStock != 5 {
		t.Fatalf("act stock = %d, want 5", config.ActStock)
	}
}

func TestRegisterPromotionProductsAcceptsPromotionMultiSkuDifferentPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}
	strategy := &listingruntime.OperationStrategy{
		StoreID:            870,
		ActivityPriceMode:  "BREAKEVEN",
		ActivityStockRatio: 0.5,
	}
	product := marketing.SkcInfo{
		Skc:         "sg-multi-price",
		Stock:       100,
		SupplyPrice: 30,
		SitePriceInfoList: []marketing.SitePriceInfo{{
			SalePrice:   30,
			Currency:    "USD",
			IsAvailable: true,
		}},
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{
				SkuCode: "sku-small",
				SitePriceInfoList: []marketing.SitePriceInfo{{
					SalePrice:   29.9,
					Currency:    "USD",
					IsAvailable: true,
				}},
			},
			{
				SkuCode: "sku-large",
				SitePriceInfoList: []marketing.SitePriceInfo{{
					SalePrice:   34.9,
					Currency:    "USD",
					IsAvailable: true,
				}},
			},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-small", CostPrice: 12.5, Currency: "USD"},
			{SkuCode: "sku-large", CostPrice: 20.5, Currency: "USD"},
		},
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		strategy,
		"",
		[]marketing.SkcInfo{product},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("result request = %+v, want one SaveConfig request", result)
	}
	if api.saved == nil {
		t.Fatal("SaveConfig was not called")
	}

	api.promotionGoods = []marketing.PromotionGoodsData{{
		Skc:              product.Skc,
		IsSaleAttribute:  1,
		InventoryNum:     product.Stock,
		USSupplyPrice:    88,
		MaxUSSupplyPrice: 88,
		SkuInfoList: []marketing.PromotionSkuInfo{
			{Sku: "sku-small", USSupplyPrice: promotionTestFloat64Ptr(88)},
			{Sku: "sku-large", USSupplyPrice: promotionTestFloat64Ptr(99)},
		},
	}}
	api.calcResponse = &marketing.CalculateSupplyPriceResponse{
		Code: "0",
		Msg:  "ok",
		Info: []marketing.SkcCalculationResult{{
			SkcName: product.Skc,
			SkuInfoList: []marketing.SkuCalculationInfo{
				{SkuCode: "sku-small", PriceInfo: marketing.PriceInfo{ProductAmount: 29.9, PromotionAmount: 12.5}},
				{SkuCode: "sku-large", PriceInfo: marketing.PriceInfo{ProductAmount: 34.9, PromotionAmount: 20.5}},
			},
		}},
	}
	activityResult, err := service.RegisterPromotionProducts(
		t.Context(),
		strategy,
		"TIME_LIMITED:227:870:multi-price",
		[]marketing.SkcInfo{product},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts activity error = %v", err)
	}
	if activityResult == nil || activityResult.ActivityRequest == nil {
		t.Fatalf("activity result = %+v, want create activity request", activityResult)
	}
	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 {
		t.Fatalf("calculated request = %+v, want one SKU-priced product", api.calculated)
	}
	skuPrices := api.calculated.SkcInfoList[0].SkuInfoList
	if len(skuPrices) != 2 {
		t.Fatalf("calculated SKU prices = %+v, want two entries", skuPrices)
	}
	if skuPrices[0].SkuCode != "sku-small" || skuPrices[0].ProductPrice != 29.9 || skuPrices[0].DiscountValue != 12.5 {
		t.Fatalf("first calculated SKU price = %+v, want sku-small price 29.9 and cost 12.5", skuPrices[0])
	}
	if skuPrices[1].SkuCode != "sku-large" || skuPrices[1].ProductPrice != 34.9 || skuPrices[1].DiscountValue != 20.5 {
		t.Fatalf("second calculated SKU price = %+v, want sku-large price 34.9 and cost 20.5", skuPrices[1])
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created request = %+v, want one product", api.created)
	}
	createdProduct := api.created.AddCostAndStockInfoList[0]
	if createdProduct.IsSaleAttribute != 1 {
		t.Fatalf("is_sale_attribute = %d, want 1 for a multi-SKU activity", createdProduct.IsSaleAttribute)
	}
	if createdProduct.CostPrice != 0 || createdProduct.MaxProductActPrice != 0 || createdProduct.ProductActPrice != 0 {
		t.Fatalf("multi-SKU product-level prices = %+v, want zero so each add_sku_list price takes effect", createdProduct)
	}
	createdSKUs := createdProduct.AddSkuList
	if len(createdSKUs) != 2 {
		t.Fatalf("created SKU list = %+v, want two entries", createdSKUs)
	}
	if createdSKUs[0].Sku != "sku-small" || createdSKUs[0].CostPrice != 29.9 {
		t.Fatalf("first created SKU = %+v, want sku-small original 29.9 activity 12.5", createdSKUs[0])
	}
	assertClose(t, createdSKUs[0].ProductActPrice, 12.5)
	if createdSKUs[1].Sku != "sku-large" || createdSKUs[1].CostPrice != 34.9 {
		t.Fatalf("second created SKU = %+v, want sku-large original 34.9 activity 20.5", createdSKUs[1])
	}
	assertClose(t, createdSKUs[1].ProductActPrice, 20.5)
}

func TestRegisterPromotionProductsAllowsPromotionMultiSkuSamePrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0.1,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:         "sg-multi-same-price",
			Stock:       100,
			SupplyPrice: 30,
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30,
				Currency:    "USD",
				IsAvailable: true,
			}},
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{
				{
					SkuCode: "sku-small",
					SitePriceInfoList: []marketing.SitePriceInfo{{
						SalePrice:   29.9,
						Currency:    "USD",
						IsAvailable: true,
					}},
				},
				{
					SkuCode: "sku-large",
					SitePriceInfoList: []marketing.SitePriceInfo{{
						SalePrice:   29.9,
						Currency:    "USD",
						IsAvailable: true,
					}},
				},
			},
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 18, Currency: "USD"},
				{SkuCode: "sku-large", CostPrice: 18, Currency: "USD"},
			},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("config list = %+v, want one config", result)
	}
	if api.saved == nil {
		t.Fatalf("saved request is nil, want SaveConfig called")
	}
}

func TestRegisterPromotionProductsKeepsLimitedBreakevenDropRateGreaterThanRegular(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-breakeven-both",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:             870,
			ActivityPriceMode:   "BREAKEVEN",
			ActivityPartakeType: "BOTH",
			ActivityStockRatio:  0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-breakeven-both",
			Stock:               100,
			SupplyPrice:         18,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
	}
	if got := api.savedRequests[0].ConfigList[0].DropRate; got != 39 {
		t.Fatalf("regular breakeven drop rate = %d, want 39", got)
	}
	if got := api.savedRequests[1].ConfigList[0].DropRate; got != 40 {
		t.Fatalf("limited breakeven drop rate = %d, want 40", got)
	}
}

func TestRegisterPromotionProductsEnablesSavedRegularConfig(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-enable-regular",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-enable-regular",
			Stock:               10,
			SupplyPrice:         20,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil {
		t.Fatalf("result = %+v, want request", result)
	}
	if len(api.updateStateRequests) != 1 {
		t.Fatalf("update state calls = %d, want 1", len(api.updateStateRequests))
	}
	update := api.updateStateRequests[0]
	if update.State != marketing.AutoPartakeConfigStateOpen {
		t.Fatalf("update state = %d, want open", update.State)
	}
	if len(update.IDs) != 1 || update.IDs[0] != 13042103 {
		t.Fatalf("update ids = %#v, want [13042103]", update.IDs)
	}
}

func TestRegisterPromotionProductsAllowsRegularPartakeWithoutStockRatio(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityPartakeType:   "REGULAR",
			ActivityMinProfitRate: 0,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-regular-no-stock-ratio",
			Stock:               10,
			SupplyPrice:         20,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil || len(result.Request.ConfigList) != 1 {
		t.Fatalf("result = %+v, want one regular config", result)
	}
	if result.Request.Type != marketing.AutoPartakeActivityTypeRegular {
		t.Fatalf("request type = %d, want regular", result.Request.Type)
	}
}

func TestRegisterPromotionProductsUsesLimitedPartakeStrategy(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-enable-limited",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityPartakeType:   "LIMITED",
			ActivityMinProfitRate: 0,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-enable-limited",
			Stock:               10,
			SupplyPrice:         20,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.saved == nil || api.saved.Type != marketing.AutoPartakeActivityTypeLimited {
		t.Fatalf("saved request type = %+v, want limited", api.saved)
	}
	if len(api.updateStateRequests) != 1 {
		t.Fatalf("update state calls = %d, want 1", len(api.updateStateRequests))
	}
	update := api.updateStateRequests[0]
	if len(update.IDs) != 1 || update.IDs[0] != 13042104 {
		t.Fatalf("update ids = %#v, want limited config id 13042104", update.IDs)
	}
}

func TestRegisterPromotionProductsUsesBothPartakeStrategy(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-enable-both",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:                      870,
			ActivityPriceMode:            "PROFIT",
			ActivityPartakeType:          "BOTH",
			ActivityMinProfitRate:        0.1,
			ActivityLimitedMinProfitRate: 0,
			ActivityStockRatio:           0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-enable-both",
			Stock:               10,
			SupplyPrice:         10,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
	}
	if result == nil || len(result.Requests) != 2 {
		t.Fatalf("result requests = %+v, want two SaveConfig requests", result)
	}
	if api.savedRequests[0].Type != marketing.AutoPartakeActivityTypeRegular || api.savedRequests[1].Type != marketing.AutoPartakeActivityTypeLimited {
		t.Fatalf("saved request types = %d/%d, want regular/limited", api.savedRequests[0].Type, api.savedRequests[1].Type)
	}
	if len(api.updateStateRequests) != 1 {
		t.Fatalf("update state calls = %d, want 1", len(api.updateStateRequests))
	}
	update := api.updateStateRequests[0]
	if len(update.IDs) != 2 || update.IDs[0] != 13042103 || update.IDs[1] != 13042104 {
		t.Fatalf("update ids = %#v, want regular and limited ids", update.IDs)
	}
}

func TestRegisterPromotionProductsUsesLimitedDiscountForBothPartake(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-both-discount",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:                     870,
			ActivityPriceMode:           "DISCOUNT",
			ActivityPartakeType:         "BOTH",
			ActivityDiscountRate:        0.2,
			ActivityLimitedDiscountRate: 0.25,
			ActivityStockRatio:          0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-both-discount",
			Stock:               10,
			SupplyPrice:         10,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
	}
	if got := api.savedRequests[0].ConfigList[0].DropRate; got != 20 {
		t.Fatalf("regular drop rate = %d, want 20", got)
	}
	if got := api.savedRequests[1].ConfigList[0].DropRate; got != 25 {
		t.Fatalf("limited drop rate = %d, want 25", got)
	}
}

func TestRegisterPromotionProductsUsesLimitedMinProfitForBothPartake(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-both-profit",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:                      870,
			ActivityPriceMode:            "PROFIT",
			ActivityPartakeType:          "BOTH",
			ActivityMinProfitRate:        0.2,
			ActivityLimitedMinProfitRate: 0.1,
			ActivityStockRatio:           0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-both-profit",
			Stock:               10,
			SupplyPrice:         10,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   20,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
	}
	if got := api.savedRequests[0].ConfigList[0].DropRate; got != 37 {
		t.Fatalf("regular profit drop rate = %d, want 37", got)
	}
	if got := api.savedRequests[1].ConfigList[0].DropRate; got != 44 {
		t.Fatalf("limited profit drop rate = %d, want 44", got)
	}
}

func TestRegisterPromotionProductsKeepsLimitedProfitDropRateGreaterAfterRounding(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{
					{
						ID:  13042103,
						Skc: "sg-both-profit-rounded",
						ActivityConfigList: []marketing.ActivityConfigDetail{
							{ID: 13042103, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
							{ID: 13042104, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:                      870,
			ActivityPriceMode:            "PROFIT",
			ActivityPartakeType:          "BOTH",
			ActivityMinProfitRate:        0.01,
			ActivityLimitedMinProfitRate: 0,
			ActivityStockRatio:           0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-both-profit-rounded",
			Stock:               999,
			SupplyPrice:         16.88,
			SupplyPriceCurrency: "USD",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30.08,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
	}
	if got := api.savedRequests[0].ConfigList[0].DropRate; got != 43 {
		t.Fatalf("regular profit drop rate = %d, want 43", got)
	}
	if got := api.savedRequests[1].ConfigList[0].DropRate; got != 44 {
		t.Fatalf("limited profit drop rate = %d, want 44", got)
	}
}

func TestRegisterPromotionProductsCreatesActivityWhenActivityKeyIsProvided(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-create",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    30,
				MaxUSSupplyPrice: 30,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-create-1"}},
			},
			{
				Skc:              "sg-other",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    30,
				MaxUSSupplyPrice: 30,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-other-1"}},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-create",
					SkuInfoList: []marketing.SkuCalculationInfo{
						{
							SkuCode: "sku-create-1",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   30,
								PromotionAmount: 6,
							},
						},
					},
				},
			},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12345},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"PROMOTION:227:870",
		[]marketing.SkcInfo{{Skc: "sg-create", Stock: 100, SupplyPrice: 30, SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-create-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, Currency: "USD", IsAvailable: true}}}}}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.saved != nil {
		t.Fatalf("saved config = %+v, want real activity creation path", api.saved)
	}
	if api.created == nil {
		t.Fatalf("created request is nil, want create activity request")
	}
	if len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created goods = %+v, want one selected SKC", api.created.AddCostAndStockInfoList)
	}
	created := api.created.AddCostAndStockInfoList[0]
	if created.Skc != "sg-create" {
		t.Fatalf("created skc = %q", created.Skc)
	}
	if created.StockNum != 50 || created.AttendNum != 50 {
		t.Fatalf("created stock = attend:%d stock:%d, want 50/50", created.AttendNum, created.StockNum)
	}
	if result == nil || result.ActivityResponse == nil || result.ActivityResponse.Info == nil || result.ActivityResponse.Info.ActivityID != 12345 {
		t.Fatalf("activity response = %+v, want activity id 12345", result)
	}
}

func TestRegisterPromotionProductsUsesSavedSupplyPriceWithoutQueryingGoods(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{{
				SkcName: "sq-saved-prices",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{SkuCode: "sku-small", PriceInfo: marketing.PriceInfo{ProductAmount: 51.54, PromotionAmount: 41.23}},
					{SkuCode: "sku-large", PriceInfo: marketing.PriceInfo{ProductAmount: 65.02, PromotionAmount: 52.02}},
				},
			}},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0", Msg: "ok", Info: &marketing.ActivityCreateInfo{ActivityID: 12345},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{store: &listingruntime.StoreInfo{ID: 687, Username: "seller"}},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 687, ActivityPriceMode: "DISCOUNT", ActivityDiscountRate: 0.2, ActivityStockRatio: 0.5,
	}, "PROMOTION:227:687", []marketing.SkcInfo{{
		Skc: "sq-saved-prices", Stock: 100, SupplyPrice: 82.16, SupplyPriceCurrency: "USD",
		SitePriceInfoList: []marketing.SitePriceInfo{{SiteCode: "US", SalePrice: 82.16, Currency: "USD", IsAvailable: true}},
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{SiteCode: "US", SalePrice: 51.54, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{SiteCode: "US", SalePrice: 65.02, Currency: "USD", IsAvailable: true}}},
		},
	}})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("QueryPromotionGoods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 {
		t.Fatalf("calculate request = %+v, want one SKC", api.calculated)
	}
	calculated := api.calculated.SkcInfoList[0].SkuInfoList
	if len(calculated) != 2 || calculated[0].ProductPrice != 51.54 || calculated[1].ProductPrice != 65.02 {
		t.Fatalf("calculated SKU prices = %+v, want 51.54 and 65.02", calculated)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created request = %+v, want one SKC", api.created)
	}
	created := api.created.AddCostAndStockInfoList[0]
	if created.IsSaleAttribute != 1 {
		t.Fatalf("is_sale_attribute = %d, want 1 for multiple SKU prices", created.IsSaleAttribute)
	}
	if created.CostPrice != 0 || created.MaxProductActPrice != 0 || created.ProductActPrice != 0 {
		t.Fatalf("multi-SKU product-level prices = %+v, want zero", created)
	}
}

func TestRegisterPromotionProductsUsesActivityKeyInTimeLimitedActivityName(t *testing.T) {
	newService := func() (*activityRegistrationServiceImpl, *promotionProductsMarketingAPIStub) {
		api := &promotionProductsMarketingAPIStub{
			promotionGoods: []marketing.PromotionGoodsData{
				{
					Skc:              "sg-create",
					IsSaleAttribute:  1,
					InventoryNum:     100,
					USSupplyPrice:    30,
					MaxUSSupplyPrice: 30,
					SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-create-1"}},
				},
			},
			calcResponse: &marketing.CalculateSupplyPriceResponse{
				Code: "0",
				Msg:  "ok",
				Info: []marketing.SkcCalculationResult{
					{
						SkcName: "sg-create",
						SkuInfoList: []marketing.SkuCalculationInfo{
							{
								SkuCode: "sku-create-1",
								PriceInfo: marketing.PriceInfo{
									ProductAmount:   30,
									PromotionAmount: 6,
								},
							},
						},
					},
				},
			},
			createResponse: &marketing.CreateActivityResponse{
				Code: "0",
				Msg:  "ok",
				Info: &marketing.ActivityCreateInfo{ActivityID: 12345},
			},
		}
		return &activityRegistrationServiceImpl{
			storeService: promotionProductsStoreServiceStub{
				store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
			},
			marketingAPI: api,
			logger:       logrus.NewEntry(logrus.New()),
		}, api
	}

	product := marketing.SkcInfo{Skc: "sg-create", Stock: 100, SupplyPrice: 30, SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-create-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, Currency: "USD", IsAvailable: true}}}}}
	serviceA, apiA := newService()
	_, err := serviceA.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"TIME_LIMITED:227:870:run-133:1",
		[]marketing.SkcInfo{product},
	)
	if err != nil {
		t.Fatalf("first RegisterPromotionProducts error = %v", err)
	}
	serviceB, apiB := newService()
	_, err = serviceB.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"TIME_LIMITED:227:870:run-133:2",
		[]marketing.SkcInfo{product},
	)
	if err != nil {
		t.Fatalf("second RegisterPromotionProducts error = %v", err)
	}

	nameA := apiA.created.ActivityBaseInfoRequest.ActName
	nameB := apiB.created.ActivityBaseInfoRequest.ActName
	if nameA == "" || nameB == "" {
		t.Fatalf("activity names are empty: %q %q", nameA, nameB)
	}
	if nameA == nameB {
		t.Fatalf("activity names are both %q, want unique names from activity key", nameA)
	}
}

func TestPromotionRegistrationSessionUsesSnapshotsAcrossChunks(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-one",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    30,
				MaxUSSupplyPrice: 30,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-one-1"}},
			},
			{
				Skc:              "sg-two",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    40,
				MaxUSSupplyPrice: 40,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-two-1"}},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-one",
					SkuInfoList: []marketing.SkuCalculationInfo{{
						SkuCode: "sku-one-1",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   30,
							PromotionAmount: 6,
						},
					}},
				},
				{
					SkcName: "sg-two",
					SkuInfoList: []marketing.SkuCalculationInfo{{
						SkuCode: "sku-two-1",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   40,
							PromotionAmount: 8,
						},
					}},
				},
			},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12345},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	session, err := service.NewPromotionRegistrationSession(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"TIME_LIMITED:227:870",
	)
	if err != nil {
		t.Fatalf("NewPromotionRegistrationSession error = %v", err)
	}

	productOne := marketing.SkcInfo{Skc: "sg-one", Stock: 100, SupplyPrice: 30, SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-one-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, Currency: "USD", IsAvailable: true}}}}}
	productTwo := marketing.SkcInfo{Skc: "sg-two", Stock: 100, SupplyPrice: 40, SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-two-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 40, Currency: "USD", IsAvailable: true}}}}}
	if _, err := session.RegisterPromotionProducts(t.Context(), "TIME_LIMITED:227:870:1", []marketing.SkcInfo{productOne}); err != nil {
		t.Fatalf("first RegisterPromotionProducts error = %v", err)
	}
	if _, err := session.RegisterPromotionProducts(t.Context(), "TIME_LIMITED:227:870:2", []marketing.SkcInfo{productTwo}); err != nil {
		t.Fatalf("second RegisterPromotionProducts error = %v", err)
	}

	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("query promotion goods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
	if api.createActivityCalls != 2 {
		t.Fatalf("create activity calls = %d, want two chunk creates", api.createActivityCalls)
	}
}

func TestRegisterPromotionProductsUsesSkuPricesForSaleAttributeGoods(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-sale-attribute",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    35.04,
				MaxUSSupplyPrice: 35.04,
				SkuInfoList: []marketing.PromotionSkuInfo{
					{
						Sku:              "sku-small",
						USSupplyPrice:    promotionTestFloat64Ptr(31.68),
						MaxUSSupplyPrice: promotionTestFloat64Ptr(31.68),
					},
					{
						Sku:              "sku-large",
						USSupplyPrice:    promotionTestFloat64Ptr(35.04),
						MaxUSSupplyPrice: promotionTestFloat64Ptr(35.04),
					},
				},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-sale-attribute",
					SkuInfoList: []marketing.SkuCalculationInfo{
						{
							SkuCode: "sku-small",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   31.68,
								PromotionAmount: 15.69,
							},
						},
						{
							SkuCode: "sku-large",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   35.04,
								PromotionAmount: 16.05,
							},
						},
					},
				},
			},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12345},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"TIME_LIMITED:227:870:sale-attribute",
		[]marketing.SkcInfo{{Skc: "sg-sale-attribute", Stock: 100, SupplyPrice: 35.04, SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 31.68, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 35.04, Currency: "USD", IsAvailable: true}}},
		}}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}

	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 || len(api.calculated.SkcInfoList[0].SkuInfoList) != 2 {
		t.Fatalf("calculated request = %+v, want two sale-attribute SKU prices", api.calculated)
	}
	if got := api.calculated.SkcInfoList[0].SkuInfoList[0].ProductPrice; got != 31.68 {
		t.Fatalf("first SKU calculate product price = %.2f, want SKU price 31.68", got)
	}
	if got := api.calculated.SkcInfoList[0].SkuInfoList[1].ProductPrice; got != 35.04 {
		t.Fatalf("second SKU calculate product price = %.2f, want SKU price 35.04", got)
	}

	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created request = %+v, want one sale-attribute SKC", api.created)
	}
	created := api.created.AddCostAndStockInfoList[0]
	if created.IsSaleAttribute != 1 || len(created.AddSkuList) != 2 {
		t.Fatalf("created sale attribute item = %+v, want two SKU rows", created)
	}
	if created.CostPrice != 0 || created.MaxProductActPrice != 0 || created.ProductActPrice != 0 {
		t.Fatalf("multi-SKU product-level prices = %+v, want zero", created)
	}
	if got := created.AddSkuList[0].CostPrice; got != 31.68 {
		t.Fatalf("first SKU create cost price = %.2f, want 31.68", got)
	}
	if got := created.AddSkuList[0].MaxProductActPrice; got != 31.68 {
		t.Fatalf("first SKU create max activity price = %.2f, want 31.68", got)
	}
	if got := created.AddSkuList[0].ProductActPrice; got != 15.99 {
		t.Fatalf("first SKU create activity price = %.2f, want 15.99", got)
	}
	if got := created.AddSkuList[1].CostPrice; got != 35.04 {
		t.Fatalf("second SKU create cost price = %.2f, want 35.04", got)
	}
	if got := created.AddSkuList[1].MaxProductActPrice; got != 35.04 {
		t.Fatalf("second SKU create max activity price = %.2f, want 35.04", got)
	}
	if got := created.AddSkuList[1].ProductActPrice; got != 18.99 {
		t.Fatalf("second SKU create activity price = %.2f, want 18.99", got)
	}
}

func TestBuildCreateActivityRequestValidatesEverySKUDiscount(t *testing.T) {
	tests := []struct {
		name                   string
		secondSKUOriginalPrice float64
		secondSKUPrice         float64
		wantIncluded           bool
		wantReasonPrice        string
	}{
		{name: "equal to 95 percent", secondSKUOriginalPrice: 200, secondSKUPrice: 190, wantIncluded: false, wantReasonPrice: "190.00"},
		{name: "equal to 95 percent with decimal rounding", secondSKUOriginalPrice: 16.60, secondSKUPrice: 15.77, wantIncluded: false, wantReasonPrice: "15.77"},
		{name: "above 95 percent", secondSKUOriginalPrice: 200, secondSKUPrice: 191, wantIncluded: false, wantReasonPrice: "191.00"},
		{name: "strictly below 95 percent", secondSKUOriginalPrice: 200, secondSKUPrice: 189.99, wantIncluded: true},
		{name: "strictly below 95 percent with sub-cent precision", secondSKUOriginalPrice: 100, secondSKUPrice: 94.999, wantIncluded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &activityRegistrationServiceImpl{logger: logrus.NewEntry(logrus.New())}
			goods := []marketing.PromotionGoodsData{{
				Skc:              "sg-multi-sku-discount",
				InventoryNum:     100,
				USSupplyPrice:    100,
				MaxUSSupplyPrice: 100,
				SkuInfoList: []marketing.PromotionSkuInfo{
					{Sku: "sku-small", USSupplyPrice: promotionTestFloat64Ptr(100)},
					{Sku: "sku-large", USSupplyPrice: promotionTestFloat64Ptr(tt.secondSKUOriginalPrice)},
				},
			}}
			calcResp := &marketing.CalculateSupplyPriceResponse{Info: []marketing.SkcCalculationResult{{
				SkcName: "sg-multi-sku-discount",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{SkuCode: "sku-small", PriceInfo: marketing.PriceInfo{ProductAmount: 100, PromotionAmount: 20}},
					{SkuCode: "sku-large", PriceInfo: marketing.PriceInfo{ProductAmount: tt.secondSKUOriginalPrice, PromotionAmount: tt.secondSKUOriginalPrice - tt.secondSKUPrice}},
				},
			}}}

			req, _, reasons := service.buildCreateActivityRequest(
				TimeLimitedDiscountConfig{EffectiveCenterList: []int{2}},
				goods,
				nil,
				calcResp,
			)

			if got := len(req.AddCostAndStockInfoList); (got == 1) != tt.wantIncluded {
				t.Fatalf("created goods count = %d, want included %t", got, tt.wantIncluded)
			}
			if tt.wantIncluded {
				if reason := reasons["sg-multi-sku-discount"]; reason != "" {
					t.Fatalf("filter reason = %q, want empty", reason)
				}
				return
			}
			reason := reasons["sg-multi-sku-discount"]
			for _, want := range []string{"sku-large", tt.wantReasonPrice, fmt.Sprintf("%.2f", tt.secondSKUOriginalPrice), "95%"} {
				if !strings.Contains(reason, want) {
					t.Fatalf("filter reason = %q, want %q", reason, want)
				}
			}
		})
	}
}

func TestBuildCreateActivityRequestRejectsMissingSKUActivityPrice(t *testing.T) {
	service := &activityRegistrationServiceImpl{logger: logrus.NewEntry(logrus.New())}
	goods := []marketing.PromotionGoodsData{{
		Skc:              "sg-missing-sku-price",
		InventoryNum:     100,
		USSupplyPrice:    100,
		MaxUSSupplyPrice: 100,
		SkuInfoList: []marketing.PromotionSkuInfo{
			{Sku: "sku-priced", USSupplyPrice: promotionTestFloat64Ptr(100)},
			{Sku: "sku-missing", USSupplyPrice: promotionTestFloat64Ptr(200)},
		},
	}}
	calcResp := &marketing.CalculateSupplyPriceResponse{Info: []marketing.SkcCalculationResult{{
		SkcName: "sg-missing-sku-price",
		SkuInfoList: []marketing.SkuCalculationInfo{
			{SkuCode: "sku-priced", PriceInfo: marketing.PriceInfo{ProductAmount: 100, PromotionAmount: 20}},
		},
	}}}

	req, _, reasons := service.buildCreateActivityRequest(
		TimeLimitedDiscountConfig{EffectiveCenterList: []int{2}},
		goods,
		nil,
		calcResp,
	)

	if got := len(req.AddCostAndStockInfoList); got != 0 {
		t.Fatalf("created goods count = %d, want missing-price SKC excluded", got)
	}
	reason := reasons["sg-missing-sku-price"]
	for _, want := range []string{"sku-missing", "活动价 0.00", "原价 0.00"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("filter reason = %q, want %q", reason, want)
		}
	}
}

func TestRegisterPromotionProductsUsesProvidedSkuPricesOverPromotionGoodsSkuPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-snapshot-prices",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    88,
				MaxUSSupplyPrice: 88,
				SkuInfoList: []marketing.PromotionSkuInfo{
					{Sku: "sku-small", USSupplyPrice: promotionTestFloat64Ptr(88), MaxUSSupplyPrice: promotionTestFloat64Ptr(88)},
					{Sku: "sku-large", USSupplyPrice: promotionTestFloat64Ptr(99), MaxUSSupplyPrice: promotionTestFloat64Ptr(99)},
				},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-snapshot-prices",
					SkuInfoList: []marketing.SkuCalculationInfo{
						{
							SkuCode: "sku-small",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   31.68,
								PromotionAmount: 6.34,
							},
						},
						{
							SkuCode: "sku-large",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   35.04,
								PromotionAmount: 7.01,
							},
						},
					},
				},
			},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12347},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"TIME_LIMITED:227:870:snapshot-prices",
		[]marketing.SkcInfo{{
			Skc: "sg-snapshot-prices", Stock: 100, SupplyPrice: 35.04,
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{
				{
					SkuCode: "sku-small",
					SitePriceInfoList: []marketing.SitePriceInfo{{
						SalePrice:   31.68,
						Currency:    "USD",
						IsAvailable: true,
					}},
				},
				{
					SkuCode: "sku-large",
					SitePriceInfoList: []marketing.SitePriceInfo{{
						SalePrice:   35.04,
						Currency:    "USD",
						IsAvailable: true,
					}},
				},
			},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}

	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 || len(api.calculated.SkcInfoList[0].SkuInfoList) != 2 {
		t.Fatalf("calculated request = %+v, want two SKU prices from provided snapshot", api.calculated)
	}
	if got := api.calculated.SkcInfoList[0].SkuInfoList[0].ProductPrice; got != 31.68 {
		t.Fatalf("first SKU calculate product price = %.2f, want snapshot price 31.68", got)
	}
	if got := api.calculated.SkcInfoList[0].SkuInfoList[1].ProductPrice; got != 35.04 {
		t.Fatalf("second SKU calculate product price = %.2f, want snapshot price 35.04", got)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 || len(api.created.AddCostAndStockInfoList[0].AddSkuList) != 2 {
		t.Fatalf("created request = %+v, want two SKU rows from snapshot prices", api.created)
	}
	if got := api.created.AddCostAndStockInfoList[0].AddSkuList[0].CostPrice; got != 31.68 {
		t.Fatalf("first SKU create cost price = %.2f, want snapshot price 31.68", got)
	}
	if got := api.created.AddCostAndStockInfoList[0].AddSkuList[1].CostPrice; got != 35.04 {
		t.Fatalf("second SKU create cost price = %.2f, want snapshot price 35.04", got)
	}
}

func TestRegisterPromotionProductsUsesCandidateSalePriceOverPromotionGoodsPrice(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-missing-price",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    62,
				MaxUSSupplyPrice: 62,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-missing-price-1", USSupplyPrice: promotionTestFloat64Ptr(62), MaxUSSupplyPrice: promotionTestFloat64Ptr(62)}},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-missing-price",
					SkuInfoList: []marketing.SkuCalculationInfo{
						{
							SkuCode: "sku-missing-price-1",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   53.95,
								PromotionAmount: 10.79,
							},
						},
					},
				},
			},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12346},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"PROMOTION:227:870",
		[]marketing.SkcInfo{{
			Skc: "sg-missing-price", Stock: 100, SupplyPrice: 53.95,
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   53.95,
				Currency:    "USD",
				IsAvailable: true,
			}},
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-missing-price-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 53.95, Currency: "USD", IsAvailable: true}}}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 {
		t.Fatalf("calculated request = %+v, want one SKC from candidate sale price", api.calculated)
	}
	sku := api.calculated.SkcInfoList[0].SkuInfoList[0]
	assertClose(t, sku.ProductPrice, 53.95)
	assertClose(t, sku.DiscountValue, 43.16)
	if sku.ProductPrice == 62 {
		t.Fatalf("sku price = %+v, still using promotion goods price", sku)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created request = %+v, want one created SKC", api.created)
	}
	created := api.created.AddCostAndStockInfoList[0]
	assertClose(t, created.ProductActPrice, 43.16)
	assertClose(t, created.CostPrice, 53.95)
	assertClose(t, created.MaxProductActPrice, 53.95)
}

func TestBuildCalculateRequestForPromotionProductsUsesDirectSKUCosts(t *testing.T) {
	service := &activityRegistrationServiceImpl{}
	config := TimeLimitedDiscountConfig{
		PriceMode:            "PROFIT",
		MinProfitRate:        0,
		FixedPriceAdjustment: 0,
		Currency:             "USD",
	}

	req := service.buildCalculateRequestForPromotionProducts(
		config,
		[]marketing.PromotionGoodsData{{
			Skc:           "sg-multi-sku-profit",
			USSupplyPrice: 25.16,
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-small", USSupplyPrice: promotionFloat64Ptr(25.16)},
				{Sku: "sku-medium", USSupplyPrice: promotionFloat64Ptr(35.34)},
				{Sku: "sku-large", USSupplyPrice: promotionFloat64Ptr(39.06)},
			},
		}},
		[]marketing.SkcInfo{{
			Skc: "sg-multi-sku-profit",
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{
				{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 25.16, IsAvailable: true}}},
				{SkuCode: "sku-medium", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 35.34, IsAvailable: true}}},
				{SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{Currency: "USD", SalePrice: 39.06, IsAvailable: true}}},
			},
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 20.88, Currency: "USD"},
				{SkuCode: "sku-medium", CostPrice: 29.33, Currency: "USD"},
				{SkuCode: "sku-large", CostPrice: 32.42, Currency: "USD"},
			},
		}},
	)

	if req == nil || len(req.SkcInfoList) != 1 || len(req.SkcInfoList[0].SkuInfoList) != 3 {
		t.Fatalf("calculate request = %+v, want three SKU prices", req)
	}
	assertClose(t, req.SkcInfoList[0].SkuInfoList[0].DiscountValue, 20.88)
	assertClose(t, req.SkcInfoList[0].SkuInfoList[1].DiscountValue, 29.33)
	assertClose(t, req.SkcInfoList[0].SkuInfoList[2].DiscountValue, 32.42)
}

func TestRegisterPromotionProductsUsesRequestedProfitModeSkuActivityPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc:              "sg-profit-sku-prices",
			IsSaleAttribute:  1,
			InventoryNum:     999,
			USSupplyPrice:    34.2,
			MaxUSSupplyPrice: 34.2,
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-small", USSupplyPrice: promotionFloat64Ptr(29.7), MaxUSSupplyPrice: promotionFloat64Ptr(28.21)},
				{Sku: "sku-large", USSupplyPrice: promotionFloat64Ptr(31.68), MaxUSSupplyPrice: promotionFloat64Ptr(30.09)},
			},
		}},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{{
				SkcName: "sg-profit-sku-prices",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{
						SkuCode: "sku-small",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   29.7,
							PromotionAmount: 18.82,
						},
					},
					{
						SkuCode: "sku-large",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   31.68,
							PromotionAmount: 20.8,
						},
					},
				},
			}},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12348},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0,
		},
		"TIME_LIMITED:227:870:profit-sku-prices",
		[]marketing.SkcInfo{{
			Skc: "sg-profit-sku-prices", Stock: 999, SupplyPrice: 10.88,
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 29.7, Currency: "USD", IsAvailable: true}}}, {SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 31.68, Currency: "USD", IsAvailable: true}}}},
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 10.88, Currency: "USD"},
				{SkuCode: "sku-large", CostPrice: 11.6053, Currency: "USD"},
			},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 || len(api.created.AddCostAndStockInfoList[0].AddSkuList) != 2 {
		t.Fatalf("created request = %+v, want two SKU rows", api.created)
	}
	wantSmall := api.calculated.SkcInfoList[0].SkuInfoList[0].DiscountValue
	wantLarge := api.calculated.SkcInfoList[0].SkuInfoList[1].DiscountValue
	assertClose(t, wantSmall, 10.88)
	assertClose(t, wantLarge, 11.6053)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[0].ProductActPrice, wantSmall)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[1].ProductActPrice, wantLarge)
}

func TestRegisterPromotionProductsUsesSKUCostsForProfitModeSkuActivityPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc:              "sg-profit-sku-costs",
			IsSaleAttribute:  1,
			InventoryNum:     999,
			USSupplyPrice:    34.2,
			MaxUSSupplyPrice: 34.2,
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-small", USSupplyPrice: promotionFloat64Ptr(29.7), MaxUSSupplyPrice: promotionFloat64Ptr(28.21)},
				{Sku: "sku-large", USSupplyPrice: promotionFloat64Ptr(31.68), MaxUSSupplyPrice: promotionFloat64Ptr(30.09)},
			},
		}},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{{
				SkcName: "sg-profit-sku-costs",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{
						SkuCode: "sku-small",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   29.7,
							PromotionAmount: 19.82,
						},
					},
					{
						SkuCode: "sku-large",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   31.68,
							PromotionAmount: 20.8,
						},
					},
				},
			}},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12349},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityMinProfitRate: 0,
		},
		"TIME_LIMITED:227:870:profit-sku-costs",
		[]marketing.SkcInfo{{
			Skc: "sg-profit-sku-costs", Stock: 999, SupplyPrice: 10.88,
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 29.7, Currency: "USD", IsAvailable: true}}}, {SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 31.68, Currency: "USD", IsAvailable: true}}}},
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 9.88, Currency: "USD"},
				{SkuCode: "sku-large", CostPrice: 10.88, Currency: "USD"},
			},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 || len(api.created.AddCostAndStockInfoList[0].AddSkuList) != 2 {
		t.Fatalf("created request = %+v, want two SKU rows", api.created)
	}
	assertClose(t, api.calculated.SkcInfoList[0].SkuInfoList[0].DiscountValue, 9.88)
	assertClose(t, api.calculated.SkcInfoList[0].SkuInfoList[1].DiscountValue, 10.88)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[0].ProductActPrice, 9.88)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[1].ProductActPrice, 10.88)
}

func TestRegisterPromotionProductsUsesSKUCostsForBreakevenModeSkuActivityPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc:              "sg-breakeven-sku-costs",
			IsSaleAttribute:  1,
			InventoryNum:     999,
			USSupplyPrice:    34.2,
			MaxUSSupplyPrice: 34.2,
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-small", USSupplyPrice: promotionFloat64Ptr(29.7), MaxUSSupplyPrice: promotionFloat64Ptr(28.21)},
				{Sku: "sku-large", USSupplyPrice: promotionFloat64Ptr(31.68), MaxUSSupplyPrice: promotionFloat64Ptr(30.09)},
			},
		}},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{{
				SkcName: "sg-breakeven-sku-costs",
				SkuInfoList: []marketing.SkuCalculationInfo{
					{
						SkuCode: "sku-small",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   29.7,
							PromotionAmount: 1.82,
						},
					},
					{
						SkuCode: "sku-large",
						PriceInfo: marketing.PriceInfo{
							ProductAmount:   31.68,
							PromotionAmount: 2.8,
						},
					},
				},
			}},
		},
		createResponse: &marketing.CreateActivityResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ActivityCreateInfo{ActivityID: 12350},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:           870,
			ActivityPriceMode: "BREAKEVEN",
		},
		"TIME_LIMITED:227:870:breakeven-sku-costs",
		[]marketing.SkcInfo{{
			Skc: "sg-breakeven-sku-costs", Stock: 999, SupplyPrice: 10.88,
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-small", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 29.7, Currency: "USD", IsAvailable: true}}}, {SkuCode: "sku-large", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 31.68, Currency: "USD", IsAvailable: true}}}},
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 9.88, Currency: "USD"},
				{SkuCode: "sku-large", CostPrice: 10.88, Currency: "USD"},
			},
		}},
	)
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 || len(api.created.AddCostAndStockInfoList[0].AddSkuList) != 2 {
		t.Fatalf("created request = %+v, want two SKU rows", api.created)
	}
	assertClose(t, api.calculated.SkcInfoList[0].SkuInfoList[0].DiscountValue, 9.88)
	assertClose(t, api.calculated.SkcInfoList[0].SkuInfoList[1].DiscountValue, 10.88)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[0].ProductActPrice, 9.88)
	assertClose(t, api.created.AddCostAndStockInfoList[0].AddSkuList[1].ProductActPrice, 10.88)
}

func assertClose(t *testing.T, got float64, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("value = %.4f, want %.4f", got, want)
	}
}

func TestRegisterPromotionProductsCreatesFromSavedSnapshotWithoutQueryingGoods(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-already-active",
				ErrorCode:        "mrs-simple_platform_limit_discounts-0006",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    30,
				MaxUSSupplyPrice: 30,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-already-active-1"}},
			},
		},
		calcResponse: &marketing.CalculateSupplyPriceResponse{
			Code: "0",
			Msg:  "ok",
			Info: []marketing.SkcCalculationResult{
				{
					SkcName: "sg-already-active",
					SkuInfoList: []marketing.SkuCalculationInfo{
						{
							SkuCode: "sku-already-active-1",
							PriceInfo: marketing.PriceInfo{
								ProductAmount:   30,
								PromotionAmount: 6,
							},
						},
					},
				},
			},
		},
	}
	service := &activityRegistrationServiceImpl{
		storeService: promotionProductsStoreServiceStub{
			store: &listingruntime.StoreInfo{ID: 870, Username: "seller"},
		},
		marketingAPI: api,
		logger:       logrus.NewEntry(logrus.New()),
	}

	result, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:              870,
			ActivityPriceMode:    "DISCOUNT",
			ActivityDiscountRate: 0.2,
			ActivityStockRatio:   0.5,
		},
		"PROMOTION:227:870",
		[]marketing.SkcInfo{{
			Skc: "sg-already-active", Stock: 100, SupplyPrice: 30,
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30,
				Currency:    "USD",
				IsAvailable: true,
			}},
			SkuPriceInfoList: []marketing.SkuSitePriceInfo{{SkuCode: "sku-already-active-1", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, Currency: "USD", IsAvailable: true}}}},
		}},
	)

	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.ActivityRequest == nil {
		t.Fatalf("result = %+v, want activity request", result)
	}
	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("QueryPromotionGoods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
}

type promotionProductsMarketingAPIStub struct {
	saved                    *marketing.SaveConfigRequest
	savedRequests            []*marketing.SaveConfigRequest
	created                  *marketing.CreateActivityRequest
	calculated               *marketing.CalculateSupplyPriceRequest
	configListResponse       *marketing.GetConfigListResponse
	updateStateRequests      []*marketing.UpdateConfigStateRequest
	promotionGoods           []marketing.PromotionGoodsData
	calcResponse             *marketing.CalculateSupplyPriceResponse
	createResponse           *marketing.CreateActivityResponse
	queryPromotionGoodsCalls int
	createActivityCalls      int
}

func (s *promotionProductsMarketingAPIStub) GetAvailableSkcList(req *marketing.GetAvailableSkcListRequest) (*marketing.GetAvailableSkcListResponse, error) {
	return nil, nil
}

func (s *promotionProductsMarketingAPIStub) SaveConfig(req *marketing.SaveConfigRequest) (*marketing.SaveConfigResponse, error) {
	s.saved = req
	s.savedRequests = append(s.savedRequests, req)
	return &marketing.SaveConfigResponse{Code: "0", Msg: "ok"}, nil
}

func (s *promotionProductsMarketingAPIStub) GetConfigList(req *marketing.GetConfigListRequest) (*marketing.GetConfigListResponse, error) {
	if s.configListResponse != nil {
		return s.configListResponse, nil
	}
	if s.saved != nil {
		configs := make([]marketing.ActivityConfigInfo, 0, len(s.saved.ConfigList))
		for i, savedConfig := range s.saved.ConfigList {
			configs = append(configs, marketing.ActivityConfigInfo{
				ID:  int64(13000000 + i),
				Skc: savedConfig.Skc,
				ActivityConfigList: []marketing.ActivityConfigDetail{
					{
						ID:           int64(13000000 + i),
						ActivityType: s.saved.ActivityType(),
						State:        marketing.AutoPartakeConfigStateClosed,
					},
				},
			})
		}
		return &marketing.GetConfigListResponse{
			Code: "0",
			Msg:  "ok",
			Info: &marketing.ConfigListInfo{
				Total:      len(configs),
				ConfigList: configs,
			},
		}, nil
	}
	return &marketing.GetConfigListResponse{Code: "0", Msg: "ok", Info: &marketing.ConfigListInfo{}}, nil
}

func (s *promotionProductsMarketingAPIStub) UpdateConfigState(req *marketing.UpdateConfigStateRequest) (*marketing.UpdateConfigStateResponse, error) {
	s.updateStateRequests = append(s.updateStateRequests, req)
	return &marketing.UpdateConfigStateResponse{Code: "0", Msg: "ok"}, nil
}

func (s *promotionProductsMarketingAPIStub) QueryPromotionGoods(req *marketing.QueryPromotionGoodsRequest) (*marketing.QueryPromotionGoodsResponse, error) {
	s.queryPromotionGoodsCalls++
	return &marketing.QueryPromotionGoodsResponse{
		Code: "0",
		Msg:  "ok",
		Info: &marketing.PromotionGoodsInfo{
			Data: s.promotionGoods,
			Meta: marketing.MetaInfo{Count: len(s.promotionGoods)},
		},
	}, nil
}

func (s *promotionProductsMarketingAPIStub) CalculateSupplyPrice(req *marketing.CalculateSupplyPriceRequest) (*marketing.CalculateSupplyPriceResponse, error) {
	s.calculated = req
	if s.calcResponse != nil {
		return s.calcResponse, nil
	}
	return &marketing.CalculateSupplyPriceResponse{Code: "0", Msg: "ok"}, nil
}

func (s *promotionProductsMarketingAPIStub) CreateActivity(req *marketing.CreateActivityRequest) (*marketing.CreateActivityResponse, error) {
	s.createActivityCalls++
	s.created = req
	if s.createResponse != nil {
		return s.createResponse, nil
	}
	return &marketing.CreateActivityResponse{Code: "0", Msg: "ok", Info: &marketing.ActivityCreateInfo{ActivityID: 1}}, nil
}

type promotionProductsStoreServiceStub struct {
	store *listingruntime.StoreInfo
}

func (s promotionProductsStoreServiceStub) GetStore(storeID int64) (*listingruntime.StoreInfo, error) {
	return s.store, nil
}

func (s promotionProductsStoreServiceStub) GetStorePauseStatus(storeID int64) (bool, error) {
	return false, nil
}

func (s promotionProductsStoreServiceStub) GetStorePauseStatusDetail(storeID int64) (*listingruntime.StorePauseStatusDetail, error) {
	return nil, nil
}

func (s promotionProductsStoreServiceStub) SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error) {
	return false, nil
}

func promotionTestFloat64Ptr(value float64) *float64 {
	return &value
}

func TestRegisterPromotionProductsUsesLowestSKUPriceAndHighestCostForBoth(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0", Msg: "ok", Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{{
					ID: 1, Skc: "sh260625180761728097751",
					ActivityConfigList: []marketing.ActivityConfigDetail{
						{ID: 1, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
						{ID: 2, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
					},
				}},
			},
		},
	}
	service := &activityRegistrationServiceImpl{marketingAPI: api, logger: logrus.NewEntry(logrus.New())}
	product := marketing.SkcInfo{
		Skc:   "sh260625180761728097751",
		Stock: 10989,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-min", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 40, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-mid", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 80, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-high", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 100, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "SKU-MIN", CostPrice: 10, Currency: "USD"},
			{SkuCode: "SKU-MID", CostPrice: 15, Currency: "USD"},
			{SkuCode: "SKU-HIGH", CostPrice: 30, Currency: "USD"},
		},
	}

	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 177, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "BOTH", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{product})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || len(result.Requests) != 2 {
		t.Fatalf("requests = %+v, want regular and limited", result)
	}
	if got := result.Requests[0].ConfigList[0].DropRate; got != 24 {
		t.Fatalf("regular drop rate = %d, want 24", got)
	}
	if got := result.Requests[1].ConfigList[0].DropRate; got != 25 {
		t.Fatalf("limited drop rate = %d, want 25", got)
	}
}

func TestRegisterPromotionProductsUsesSupplyPriceForBreakevenDropRate(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{Code: "0", Msg: "ok", Info: &marketing.ConfigListInfo{
			Total: 1,
			ConfigList: []marketing.ActivityConfigInfo{{
				ID: 1, Skc: "sz260708164727639531767",
				ActivityConfigList: []marketing.ActivityConfigDetail{
					{ID: 1, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
					{ID: 2, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
				},
			}},
		}},
	}
	service := &activityRegistrationServiceImpl{marketingAPI: api, logger: logrus.NewEntry(logrus.New())}
	product := marketing.SkcInfo{
		Skc: "sz260708164727639531767", Stock: 7992, SupplyPrice: 82.16,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 96.98, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 96.98, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-one", CostPrice: 26.12, Currency: "USD"},
			{SkuCode: "sku-two", CostPrice: 26.12, Currency: "USD"},
		},
	}

	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 1043, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "BOTH", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{product})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || len(result.Requests) != 2 {
		t.Fatalf("requests = %+v, want regular and limited", result)
	}
	if got := result.Requests[0].ConfigList[0].DropRate; got != 67 {
		t.Fatalf("regular drop rate = %d, want 67 from supply price 82.16 and cost 26.12", got)
	}
	if got := result.Requests[1].ConfigList[0].DropRate; got != 68 {
		t.Fatalf("limited drop rate = %d, want 68 from supply price 82.16 and cost 26.12", got)
	}
}

func TestRegisterPromotionProductsUsesSavedSKUPricesForCandidate(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		configListResponse: &marketing.GetConfigListResponse{
			Code: "0", Msg: "ok", Info: &marketing.ConfigListInfo{
				Total: 1,
				ConfigList: []marketing.ActivityConfigInfo{{
					ID: 1, Skc: "sq-live-prices",
					ActivityConfigList: []marketing.ActivityConfigDetail{
						{ID: 1, ActivityType: marketing.AutoPartakeActivityTypeRegular, State: marketing.AutoPartakeConfigStateClosed},
						{ID: 2, ActivityType: marketing.AutoPartakeActivityTypeLimited, State: marketing.AutoPartakeConfigStateClosed},
					},
				}},
			},
		},
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc: "sq-live-prices",
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-one", USSupplyPrice: promotionTestFloat64Ptr(30)},
				{Sku: "sku-two", SupplyPriceInfo: &marketing.SupplyPriceInfo{SupplyPrice: 20, Currency: "USD"}},
			},
		}},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		storeService: promotionProductsStoreServiceStub{store: &listingruntime.StoreInfo{ID: 687}},
		logger:       logrus.NewEntry(logrus.New()),
	}
	product := marketing.SkcInfo{
		Skc: "sq-live-prices", Stock: 100,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 74.8, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 59.2, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "SKU-ONE", CostPrice: 10, Currency: "USD"},
			{SkuCode: "SKU-TWO", CostPrice: 5, Currency: "USD"},
		},
	}

	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 687, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "BOTH", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{product})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("QueryPromotionGoods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
	if result == nil || len(result.Requests) != 2 {
		t.Fatalf("requests = %+v, want regular and limited", result)
	}
	regularDropRate := result.Requests[0].ConfigList[0].DropRate
	limitedDropRate := result.Requests[1].ConfigList[0].DropRate
	if regularDropRate > 80 || limitedDropRate > 80 {
		t.Fatalf("drop rates = regular %d, limited %d; neither may exceed SHEIN's 80%% discount limit", regularDropRate, limitedDropRate)
	}
	if limitedDropRate <= regularDropRate {
		t.Fatalf("limited drop rate = %d, want greater than regular drop rate %d", limitedDropRate, regularDropRate)
	}
}

func TestRegisterPromotionProductsUsesSavedSKUPricesWhenRealtimePricesAreMissing(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc: "sq-common-live-price",
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-one", USSupplyPrice: promotionTestFloat64Ptr(30)},
				{Sku: "sku-two", SupplyPrice: promotionTestFloat64Ptr(30)},
				{Sku: "sku-three"},
			},
		}},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		storeService: promotionProductsStoreServiceStub{store: &listingruntime.StoreInfo{ID: 1043}},
		logger:       logrus.NewEntry(logrus.New()),
	}
	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 1043, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "REGULAR",
	}, "", []marketing.SkcInfo{{
		Skc: "sq-common-live-price", Stock: 100,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 96.98, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 96.98, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-three", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 96.98, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-one", CostPrice: 5, Currency: "USD"},
			{SkuCode: "sku-two", CostPrice: 10, Currency: "USD"},
			{SkuCode: "sku-three", CostPrice: 15, Currency: "USD"},
		},
	}})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil {
		t.Fatalf("result = %+v, want promotion request", result)
	}
	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("QueryPromotionGoods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
	if got := result.Request.ConfigList[0].DropRate; got > 80 {
		t.Fatalf("drop rate = %d, must not exceed SHEIN's 80%% discount limit", got)
	}
}

func TestRegisterPromotionProductsDoesNotFilterCandidateForMissingRealtimePrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{{
			Skc: "sq-live-prices-incomplete",
			SkuInfoList: []marketing.PromotionSkuInfo{
				{Sku: "sku-one", USSupplyPrice: promotionTestFloat64Ptr(30)},
			},
		}},
	}
	service := &activityRegistrationServiceImpl{
		marketingAPI: api,
		storeService: promotionProductsStoreServiceStub{store: &listingruntime.StoreInfo{ID: 687}},
		logger:       logrus.NewEntry(logrus.New()),
	}
	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 687, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "REGULAR",
	}, "", []marketing.SkcInfo{{
		Skc: "sq-live-prices-incomplete", Stock: 100,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 74.8, Currency: "USD", IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 59.2, Currency: "USD", IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
			{SkuCode: "sku-one", CostPrice: 10, Currency: "USD"},
			{SkuCode: "sku-two", CostPrice: 5, Currency: "USD"},
		},
	}})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request == nil {
		t.Fatalf("result = %+v, want promotion request", result)
	}
	if api.queryPromotionGoodsCalls != 0 {
		t.Fatalf("QueryPromotionGoods calls = %d, want 0", api.queryPromotionGoodsCalls)
	}
	if api.saved == nil {
		t.Fatal("saved request is nil")
	}
}

func TestRegisterPromotionProductsRejectsPartialMultiSKUPrices(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{}
	service := &activityRegistrationServiceImpl{marketingAPI: api, logger: logrus.NewEntry(logrus.New())}
	result, err := service.RegisterPromotionProducts(t.Context(), &listingruntime.OperationStrategy{
		StoreID: 177, ActivityPriceMode: "BREAKEVEN", ActivityPartakeType: "REGULAR", ActivityStockRatio: 0.5,
	}, "", []marketing.SkcInfo{{
		Skc: "skc-partial", Stock: 10,
		SkuPriceInfoList: []marketing.SkuSitePriceInfo{
			{SkuCode: "sku-one", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 30, IsAvailable: true}}},
			{SkuCode: "sku-two", SitePriceInfoList: []marketing.SitePriceInfo{{SalePrice: 40, IsAvailable: true}}},
		},
		SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{{SkuCode: "sku-one", CostPrice: 18}},
	}})
	if err != nil {
		t.Fatalf("RegisterPromotionProducts error = %v", err)
	}
	if result == nil || result.Request != nil || len(result.Requests) != 0 {
		t.Fatalf("result = %+v, want no promotion request", result)
	}
	if api.saved != nil {
		t.Fatalf("saved request = %+v, want SaveConfig not called", api.saved)
	}
}
