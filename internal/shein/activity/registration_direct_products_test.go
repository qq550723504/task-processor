package activity

import (
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
	saved          *marketing.SaveConfigRequest
	created        *marketing.CreateActivityRequest
	calculated     *marketing.CalculateSupplyPriceRequest
	promotionGoods []marketing.PromotionGoodsData
	calcResponse   *marketing.CalculateSupplyPriceResponse
	createResponse *marketing.CreateActivityResponse
}

func (s *promotionProductsMarketingAPIStub) GetAvailableSkcList(req *marketing.GetAvailableSkcListRequest) (*marketing.GetAvailableSkcListResponse, error) {
	return nil, nil
}

func (s *promotionProductsMarketingAPIStub) SaveConfig(req *marketing.SaveConfigRequest) (*marketing.SaveConfigResponse, error) {
	s.saved = req
	return &marketing.SaveConfigResponse{Code: "0", Msg: "ok"}, nil
}

func (s *promotionProductsMarketingAPIStub) GetConfigList(req *marketing.GetConfigListRequest) (*marketing.GetConfigListResponse, error) {
	return nil, nil
}

func (s *promotionProductsMarketingAPIStub) QueryPromotionGoods(req *marketing.QueryPromotionGoodsRequest) (*marketing.QueryPromotionGoodsResponse, error) {
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
