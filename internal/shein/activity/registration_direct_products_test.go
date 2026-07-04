package activity

import (
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

	_, err := service.RegisterPromotionProducts(
		t.Context(),
		&listingruntime.OperationStrategy{
			StoreID:               870,
			ActivityPriceMode:     "PROFIT",
			ActivityPartakeType:   "BOTH",
			ActivityMinProfitRate: 0,
			ActivityStockRatio:    0.5,
		},
		"",
		[]marketing.SkcInfo{{
			Skc:                 "sg-enable-both",
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
	if len(api.savedRequests) != 2 {
		t.Fatalf("saved request count = %d, want 2", len(api.savedRequests))
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
		[]marketing.SkcInfo{{Skc: "sg-create"}},
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
		[]marketing.SkcInfo{{Skc: "sg-create"}},
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
		[]marketing.SkcInfo{{Skc: "sg-create"}},
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

func TestPromotionRegistrationSessionReusesPromotionGoodsAcrossChunks(t *testing.T) {
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

	if _, err := session.RegisterPromotionProducts(t.Context(), "TIME_LIMITED:227:870:1", []marketing.SkcInfo{{Skc: "sg-one"}}); err != nil {
		t.Fatalf("first RegisterPromotionProducts error = %v", err)
	}
	if _, err := session.RegisterPromotionProducts(t.Context(), "TIME_LIMITED:227:870:2", []marketing.SkcInfo{{Skc: "sg-two"}}); err != nil {
		t.Fatalf("second RegisterPromotionProducts error = %v", err)
	}

	if api.queryPromotionGoodsCalls != 1 {
		t.Fatalf("query promotion goods calls = %d, want one shared query", api.queryPromotionGoodsCalls)
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
		[]marketing.SkcInfo{{Skc: "sg-sale-attribute"}},
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

func TestRegisterPromotionProductsUsesProvidedSkuPricesWhenPromotionGoodsSkuPricesAreMissing(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-snapshot-prices",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    0,
				MaxUSSupplyPrice: 0,
				SkuInfoList: []marketing.PromotionSkuInfo{
					{Sku: "sku-small"},
					{Sku: "sku-large"},
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
			Skc: "sg-snapshot-prices",
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

func TestRegisterPromotionProductsUsesCandidateSalePriceWhenPromotionGoodsPriceIsMissing(t *testing.T) {
	api := &promotionProductsMarketingAPIStub{
		promotionGoods: []marketing.PromotionGoodsData{
			{
				Skc:              "sg-missing-price",
				IsSaleAttribute:  1,
				InventoryNum:     100,
				USSupplyPrice:    0,
				MaxUSSupplyPrice: 0,
				SkuInfoList:      []marketing.PromotionSkuInfo{{Sku: "sku-missing-price-1"}},
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
			Skc: "sg-missing-price",
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
	if api.calculated == nil || len(api.calculated.SkcInfoList) != 1 {
		t.Fatalf("calculated request = %+v, want one SKC from candidate sale price fallback", api.calculated)
	}
	sku := api.calculated.SkcInfoList[0].SkuInfoList[0]
	if sku.ProductPrice != 30 || sku.DiscountValue != 24 {
		t.Fatalf("sku price = %+v, want product 30 discount value 24", sku)
	}
	if api.created == nil || len(api.created.AddCostAndStockInfoList) != 1 {
		t.Fatalf("created request = %+v, want one created SKC", api.created)
	}
	created := api.created.AddCostAndStockInfoList[0]
	if created.ProductActPrice != 24 || created.CostPrice != 30 || created.MaxProductActPrice != 30 {
		t.Fatalf("created price = %+v, want fallback prices from candidate sale snapshot", created)
	}
}

func TestBuildCalculateRequestForPromotionProductsScalesProfitModeSkuPrices(t *testing.T) {
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
			Skc:         "sg-multi-sku-profit",
			SupplyPrice: 20.88,
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
			Skc:         "sg-profit-sku-prices",
			SupplyPrice: 10.88,
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
	assertClose(t, wantSmall, 9.45)
	assertClose(t, wantLarge, 10.08)
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
			Skc:         "sg-profit-sku-costs",
			SupplyPrice: 10.88,
			SkuCostPriceInfoList: []marketing.SkuCostPriceInfo{
				{SkuCode: "sku-small", CostPrice: 9.88},
				{SkuCode: "sku-large", CostPrice: 10.88},
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

func TestRegisterPromotionProductsReturnsFilteredProductReasonWhenNoGoodsCanBeCreated(t *testing.T) {
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
			Skc: "sg-already-active",
			SitePriceInfoList: []marketing.SitePriceInfo{{
				SalePrice:   30,
				Currency:    "USD",
				IsAvailable: true,
			}},
		}},
	)

	if err == nil {
		t.Fatal("RegisterPromotionProducts error = nil, want filtered product reason")
	}
	if !strings.Contains(err.Error(), "sg-already-active") ||
		!strings.Contains(err.Error(), "mrs-simple_platform_limit_discounts-0006") {
		t.Fatalf("error = %q, want SKC and SHEIN activity reason", err.Error())
	}
	if result == nil || result.FilterReasons["sg-already-active"] == "" {
		t.Fatalf("filter reasons = %+v, want SKC-specific reason", result)
	}
	if !strings.Contains(result.FilterReasons["sg-already-active"], "mrs-simple_platform_limit_discounts-0006") {
		t.Fatalf("filter reason = %q, want SHEIN activity reason", result.FilterReasons["sg-already-active"])
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
