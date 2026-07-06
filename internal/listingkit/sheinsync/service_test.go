package sheinsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"task-processor/internal/listingadmin"
	sheinproduct "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/productsync"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSyncSheinOnShelfProductsUsesOnShelfRequestAndPersistsRows(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	firstPageItems := make([]sheinproduct.ProductListItem, 0, 100)
	firstPageItems = append(firstPageItems, sheinproduct.ProductListItem{
		SpuName:          "spu-1",
		SpuCode:          "SPU001",
		CategoryID:       1001,
		BrandName:        "SHEIN",
		ProductNameMulti: "Product One",
		ShelfStatus:      "ON_SHELF",
		PublishTime:      "2026-06-01 08:30:00",
		FirstShelfTime:   "2026-06-01 09:00:00",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1", SkcCode: "SKC001", SaleName: "Red", SupplierCode: "SUP-1", MainImageThumbnailURL: "https://img/1.jpg", BusinessModel: 7},
			{SkcName: "skc-2", SkcCode: "SKC002", SaleName: "Blue", SupplierCode: "SUP-2", MainImageThumbnailURL: "https://img/2.jpg"},
		},
	})
	for i := 0; i < 99; i++ {
		firstPageItems = append(firstPageItems, sheinproduct.ProductListItem{
			SpuName:     fmt.Sprintf("placeholder-%d", i),
			ShelfStatus: "ON_SHELF",
		})
	}
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse(firstPageItems, 101),
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-2",
					SpuCode:          "SPU002",
					CategoryID:       1002,
					BrandName:        "SHEIN",
					ProductNameMulti: "Product Two",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-3", SkcCode: "SKC003", SaleName: "Green", SupplierCode: "SUP-3", MainImageThumbnailURL: "https://img/3.jpg"},
					},
				},
			}, 101),
		},
		queryPriceResp: makePriceQueryResponse([]sheinproduct.SkcPriceData{
			{
				SkcName: "skc-1",
				SkuInfoList: []sheinproduct.SkuPriceInfo{
					{
						SkuCode: "SKU001",
						PriceInfoList: []sheinproduct.SkuPriceDetail{
							{Currency: "USD", ShopPrice: 34.17},
						},
					},
					{
						SkuCode: "SKU002",
						PriceInfoList: []sheinproduct.SkuPriceDetail{
							{Currency: "USD", ShopPrice: 39.99},
						},
					},
				},
			},
		}),
		queryInventoryResp: makeInventoryQueryResponse([]sheinproduct.SkcInventory{
			{
				SkcName: "skc-1",
				SkuInfo: []sheinproduct.SkuInventory{
					{
						SkuCode: "SKU001",
						SaleNameInfo: []sheinproduct.SkuSaleNameInfo{
							{SaleAttrName: "尺寸", SaleName: "12*18cm"},
						},
						InventoryInfo: []sheinproduct.WarehouseInventory{
							{InventoryQuantity: 999, UsableInventory: 321},
						},
					},
					{
						SkuCode: "SKU002",
						SaleNameInfo: []sheinproduct.SkuSaleNameInfo{
							{SaleAttrName: "尺寸", SaleName: "20*25cm"},
						},
						InventoryInfo: []sheinproduct.WarehouseInventory{
							{InventoryQuantity: 111, UsableInventory: 22},
						},
					},
				},
			},
		}),
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{
			"spu-1|skc-1": {CostPrice: float64Ptr(11.2), Currency: "USD"},
			"spu-1|skc-2": {CostPrice: float64Ptr(12.3), Currency: "USD"},
			"spu-2|skc-3": {CostPrice: float64Ptr(13.4), Currency: "USD"},
		},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 11, 22, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	require.Equal(t, 3, job.FetchedCount)
	require.Equal(t, 3, job.InsertedCount)
	require.Equal(t, 0, job.UpdatedCount)
	require.Equal(t, SheinSyncJobStatusSucceeded, job.Status)
	require.Len(t, productAPI.listCalls, 2)
	require.Equal(t, 1, productAPI.listCalls[0].pageNum)
	require.Equal(t, 2, productAPI.listCalls[1].pageNum)
	require.Equal(t, 100, productAPI.listCalls[0].pageSize)
	require.Equal(t, "ON_SHELF", productAPI.listCalls[0].request.ShelfType)
	require.Equal(t, 1, productAPI.listCalls[0].request.SortType)
	require.Equal(t, "ON_SHELF", productAPI.listCalls[1].request.ShelfType)
	require.Equal(t, 1, productAPI.listCalls[1].request.SortType)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 11, StoreID: 22, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 3)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.True(t, rows[0].IsActive)
	require.Equal(t, 7, rows[0].BusinessModel)
	require.Equal(t, "USD", rows[0].Currency)
	require.NotNil(t, rows[0].PublishTime)
	require.NotNil(t, rows[0].FirstShelfTime)
	require.JSONEq(t, `{
		"sale_price":34.17,
		"currency":"USD",
		"sub_site":"",
		"sku_prices":[
			{"sku_code":"SKU001","sale_price":34.17,"currency":"USD","sub_site":""},
			{"sku_code":"SKU002","sale_price":39.99,"currency":"USD","sub_site":""}
		]
	}`, rows[0].PriceSnapshot)
	require.JSONEq(t, `{"total":1110,"available":343}`, rows[0].InventorySnapshot)
	require.JSONEq(t, `{
		"spu_name":"spu-1",
		"spu_code":"SPU001",
		"shelf_status":"ON_SHELF",
		"publish_time":"2026-06-01 08:30:00",
		"first_shelf_time":"2026-06-01 09:00:00",
		"product_name_multi":"Product One",
		"skc_name":"skc-1",
		"skc_code":"SKC001",
		"sale_name":"Red",
		"supplier_code":"SUP-1",
		"sku_codes":["SKU001","SKU002"],
		"sku_info":[
			{"sku_code":"SKU001","variant_label":"12*18cm","sale_name_info":[{"sale_attr_name":"尺寸","sale_name":"12*18cm"}]},
			{"sku_code":"SKU002","variant_label":"20*25cm","sale_name_info":[{"sale_attr_name":"尺寸","sale_name":"20*25cm"}]}
		]
	}`, rows[0].SiteSnapshot)
}

func TestSyncSheinOnShelfProductsPersistsSheinSupplyPriceSeparatelyFromCost(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	oldAutoCost := 37.2
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           11,
		StoreID:            22,
		SPUName:            "spu-supply",
		SKCName:            "sh260626230038058040685",
		SupplierCode:       "sh260626230038058040685",
		AutoCostPrice:      &oldAutoCost,
		EffectiveCostPrice: &oldAutoCost,
		CostPriceSource:    SheinCostPriceSourceAuto,
		IsActive:           true,
	})
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-supply",
					SpuCode:          "SPU-SUPPLY",
					ProductNameMulti: "SHEIN synced product",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{
							SkcName:               "sh260626230038058040685",
							SkcCode:               "SKC-SUPPLY",
							SaleName:              "Default",
							SupplierCode:          "sh260626230038058040685",
							MainImageThumbnailURL: "https://img/supply.jpg",
						},
					},
				},
			}, 1),
		},
		queryPriceResp: makePriceQueryResponse([]sheinproduct.SkcPriceData{
			{
				SkcName: "sh260626230038058040685",
				SkuInfoList: []sheinproduct.SkuPriceInfo{
					{
						SkuCode: "SKU-SUPPLY",
						PriceInfoList: []sheinproduct.SkuPriceDetail{
							{Currency: "USD", ShopPrice: 42.8},
						},
					},
				},
			},
		}),
		queryCostPriceResp: makeCostPriceQueryResponse([]sheinproduct.SkcCostData{
			{
				SkcName: "sh260626230038058040685",
				SkuCostInfoList: []sheinproduct.SkuCostInfo{
					{
						SkuCode: "SKU-SUPPLY",
						CostPriceInfo: sheinproduct.CostPrice{
							CostPrice: "37.20",
							Currency:  "USD",
						},
					},
				},
			},
		}),
	}

	service := NewSheinSyncService(repo, productAPI, nil)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 11, 22, SheinSyncTriggerModeManual)
	require.NoError(t, err)
	require.Equal(t, SheinSyncJobStatusSucceeded, job.Status)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 11, StoreID: 22, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].AutoCostPrice)
	require.Nil(t, rows[0].EffectiveCostPrice)
	require.Equal(t, SheinCostPriceSourceNone, rows[0].CostPriceSource)
	require.NotNil(t, rows[0].SupplyPrice)
	require.Equal(t, 37.2, *rows[0].SupplyPrice)
	require.Equal(t, "USD", rows[0].SupplyPriceCurrency)
	require.JSONEq(t, `{
		"sale_price":42.8,
		"currency":"USD",
		"sub_site":"",
		"sku_prices":[{"sku_code":"SKU-SUPPLY","sale_price":42.8,"currency":"USD","sub_site":""}]
	}`, rows[0].PriceSnapshot)
}

func TestSyncSheinOnShelfProductsPersistsInventorySyncAttributes(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{{
				SpuName:          "spu-1",
				ProductNameMulti: "Product One",
				ShelfStatus:      "ON_SHELF",
				SkcInfoList: []sheinproduct.SkcInfoItem{{
					SkcName: "skc-1",
					SkcCode: "O1",
					SkuInfo: []sheinproduct.SkuInfo{{SkuCode: "I1", SupplierSKU: "seller-sku-1"}},
				}},
			}}, 1),
		},
		queryInventoryResp: makeInventoryQueryResponse([]sheinproduct.SkcInventory{{
			SkcName: "skc-1",
			SkuInfo: []sheinproduct.SkuInventory{{
				SkuCode: "I1",
				InventoryInfo: []sheinproduct.WarehouseInventory{{
					InventoryQuantity: 8,
					UsableInventory:   5,
				}},
			}},
		}}),
	}
	service := newSheinSyncService(repo, productAPI, nil, &sheinSyncServiceCostResolverStub{})
	service.inventoryMappingSource = sheinInventoryMappingSourceStub{
		expectedPlatform: "shein",
		byPlatformProductID: map[string]listingadmin.ProductImportMapping{
			"I1": {
				ID:                100,
				TenantID:          11,
				StoreID:           22,
				Platform:          "SHEIN",
				Region:            "US",
				ProductID:         "B0TEST",
				SKU:               "seller-sku-1",
				PlatformProductID: "I1",
			},
		},
	}

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 11, 22, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 11, StoreID: 22, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	var attributes []productsync.EnrichedSkcInfo
	require.NoError(t, json.Unmarshal([]byte(rows[0].InventorySyncAttributes), &attributes))
	require.Len(t, attributes, 1)
	require.Equal(t, "skc-1", attributes[0].SkcName)
	require.Equal(t, "O1", attributes[0].SkcCode)
	require.Len(t, attributes[0].SkuInfo, 1)
	require.Equal(t, "I1", attributes[0].SkuInfo[0].SkuCode)
	require.Equal(t, "seller-sku-1", attributes[0].SkuInfo[0].SupplierSKU)
	require.NotNil(t, attributes[0].SkuInfo[0].MappingInfo)
	require.Equal(t, int64(100), attributes[0].SkuInfo[0].MappingInfo.ID)
	require.Equal(t, "SHEIN", attributes[0].SkuInfo[0].MappingInfo.Platform)
	require.Equal(t, "US", attributes[0].SkuInfo[0].MappingInfo.Region)
	require.Equal(t, "B0TEST", attributes[0].SkuInfo[0].MappingInfo.ProductID)
	require.NotNil(t, attributes[0].SkuInfo[0].MappingInfo.SKU)
	require.Equal(t, "seller-sku-1", *attributes[0].SkuInfo[0].MappingInfo.SKU)
	require.NotNil(t, attributes[0].SkuInfo[0].UsableInventory)
	require.Equal(t, 5, *attributes[0].SkuInfo[0].UsableInventory)
	require.NotNil(t, attributes[0].SkuInfo[0].InventoryQuantity)
	require.Equal(t, 8, *attributes[0].SkuInfo[0].InventoryQuantity)
}

func TestSyncSheinSourceSDSProductRefreshesOnlyMatchingSourceProducts(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           11,
		StoreID:            22,
		SPUName:            "spu-1",
		SPUCode:            "SPU001",
		SKCName:            "skc-1",
		SKCCode:            "SKC001",
		SupplierCode:       "XB0603003001-181EB5DF",
		ProductNameMulti:   "Old Product",
		ManualCostPrice:    float64Ptr(31.2),
		EffectiveCostPrice: float64Ptr(31.2),
		CostPriceSource:    SheinCostPriceSourceManual,
		IsActive:           true,
		SiteSnapshot:       `{"sku_codes":["OLD-SKU"]}`,
	})
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:     11,
		StoreID:      22,
		SPUName:      "spu-2",
		SKCName:      "skc-2",
		SupplierCode: "OTHER0603001-181EB5DF",
		IsActive:     true,
		SiteSnapshot: `{"sku_codes":["OTHER-SKU"]}`,
	})
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					SpuCode:          "SPU001",
					ProductNameMulti: "Fresh Product",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{
							SkcName:      "skc-1",
							SkcCode:      "SKC001",
							SaleName:     "多色",
							SupplierCode: "XB0603003001-181EB5DF",
							SkuInfo: []sheinproduct.SkuInfo{
								{SkuCode: "SKU001", SupplierSKU: "XB0603003001-V381-TF7E6627E-RB6679CE2-7192C992"},
								{SkuCode: "SKU002", SupplierSKU: "XB0603003002-V382-TF7E6627E-RB6679CE2-7192C992"},
							},
						},
						{
							SkcName:      "skc-other",
							SupplierCode: "XB0603999999-181EB5DF",
						},
					},
				},
				{
					SpuName: "spu-2",
					SkcInfoList: []sheinproduct.SkcInfoItem{{
						SkcName:      "skc-2",
						SupplierCode: "OTHER0603001-181EB5DF",
					}},
				},
			}, 2),
		},
		queryInventoryResp: makeInventoryQueryResponse([]sheinproduct.SkcInventory{
			{
				SkcName: "skc-1",
				SkuInfo: []sheinproduct.SkuInventory{
					{
						SkuCode: "SKU001",
						SaleNameInfo: []sheinproduct.SkuSaleNameInfo{
							{SaleAttrName: "颜色", SaleName: "white"},
							{SaleAttrName: "尺寸", SaleName: "12*18cm"},
						},
					},
					{
						SkuCode: "SKU002",
						SaleNameInfo: []sheinproduct.SkuSaleNameInfo{
							{SaleAttrName: "颜色", SaleName: "white"},
							{SaleAttrName: "尺寸", SaleName: "20*25cm"},
						},
					},
				},
			},
		}),
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{
			"spu-1|skc-1": {CostPrice: float64Ptr(12.5), Currency: "USD"},
		},
	}
	service := NewSheinSyncService(repo, productAPI, costResolver)

	syncedCount, err := service.SyncSheinSourceSDSProduct(context.Background(), 11, 22, "XB0603003001")
	require.NoError(t, err)
	require.Equal(t, 1, syncedCount)
	require.Len(t, productAPI.listCalls, 1)
	require.Equal(t, "ON_SHELF", productAPI.listCalls[0].request.ShelfType)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 11, StoreID: 22, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, rows, 2)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.NotNil(t, rows[0].ManualCostPrice)
	require.Equal(t, 31.2, *rows[0].ManualCostPrice)
	require.Nil(t, rows[0].AutoCostPrice)
	require.NotNil(t, rows[0].SupplyPrice)
	require.Equal(t, 12.5, *rows[0].SupplyPrice)
	require.Equal(t, "USD", rows[0].SupplyPriceCurrency)
	require.Equal(t, SheinCostPriceSourceManual, rows[0].CostPriceSource)
	require.JSONEq(t, `{
		"spu_name":"spu-1",
		"spu_code":"SPU001",
		"shelf_status":"ON_SHELF",
		"publish_time":"",
		"first_shelf_time":"",
		"product_name_multi":"Fresh Product",
		"skc_name":"skc-1",
		"skc_code":"SKC001",
		"sale_name":"多色",
		"supplier_code":"XB0603003001-181EB5DF",
		"sku_codes":["SKU001","SKU002"],
		"sku_info":[
			{"sku_code":"SKU001","supplier_sku":"XB0603003001-V381-TF7E6627E-RB6679CE2-7192C992","variant_label":"12*18cm","sale_name_info":[{"sale_attr_name":"颜色","sale_name":"white"},{"sale_attr_name":"尺寸","sale_name":"12*18cm"}]},
			{"sku_code":"SKU002","supplier_sku":"XB0603003002-V382-TF7E6627E-RB6679CE2-7192C992","variant_label":"20*25cm","sale_name_info":[{"sale_attr_name":"颜色","sale_name":"white"},{"sale_attr_name":"尺寸","sale_name":"20*25cm"}]}
		]
	}`, rows[0].SiteSnapshot)
	require.Equal(t, "OTHER-SKU", SheinSyncedProductSKUCodes(rows[1])[0])
}

func TestSheinSyncServiceResolveProductAPIReturnsConfiguredRuntimeAPI(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{}
	service := NewSheinSyncService(repo, productAPI, nil)

	resolved, err := service.ResolveProductAPI(context.Background(), 77)
	require.NoError(t, err)
	require.Same(t, productAPI, resolved)
}

func TestSheinSyncServiceResolveProductAPIUsesBuilderWhenConfigured(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{}
	service := NewSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderStub{productAPI: productAPI}, nil)

	resolved, err := service.ResolveProductAPI(context.Background(), 88)
	require.NoError(t, err)
	require.Same(t, productAPI, resolved)
}

func TestSyncSheinOnShelfProductsRewordsRuntimeOfflineParsingFallback(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	service := NewSheinSyncServiceWithBuilder(repo, sheinSyncProductAPIBuilderStub{
		fallback: "SHEIN 店铺 cookie 不可用，已降级为离线解析",
	}, nil)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "SHEIN 店铺 cookie 不可用，无法同步 SHEIN 商品")
	require.NotContains(t, err.Error(), "离线解析")

	jobs, total, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, jobs, 1)
	require.Equal(t, SheinSyncJobStatusFailed, jobs[0].Status)
	require.Contains(t, jobs[0].ErrorSummary, "无法同步 SHEIN 商品")
	require.NotContains(t, jobs[0].ErrorSummary, "离线解析")
}

func TestSyncSheinOnShelfProductsManualOverrideWinsOverSheinSupplyPrice(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           1,
		StoreID:            101,
		SPUName:            "spu-1",
		SKCName:            "skc-1",
		ManualCostPrice:    float64Ptr(19.8),
		EffectiveCostPrice: float64Ptr(19.8),
		CostPriceSource:    SheinCostPriceSourceManual,
		IsActive:           true,
	})

	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
			}, 1),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{
			"spu-1|skc-1": {CostPrice: float64Ptr(12.5), Currency: "USD"},
		},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 1, 101, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 1, StoreID: 101, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ManualCostPrice)
	require.Nil(t, rows[0].AutoCostPrice)
	require.NotNil(t, rows[0].SupplyPrice)
	require.NotNil(t, rows[0].EffectiveCostPrice)
	require.Equal(t, 19.8, *rows[0].ManualCostPrice)
	require.Equal(t, 12.5, *rows[0].SupplyPrice)
	require.Equal(t, "USD", rows[0].SupplyPriceCurrency)
	require.Equal(t, 19.8, *rows[0].EffectiveCostPrice)
	require.Equal(t, SheinCostPriceSourceManual, rows[0].CostPriceSource)
}

func TestSyncSheinOnShelfProductsClearsLegacyAutoCostWhenResolverOmitsSKC(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		TenantID:           5,
		StoreID:            55,
		SPUName:            "spu-1",
		SKCName:            "skc-1",
		AutoCostPrice:      float64Ptr(16.6),
		EffectiveCostPrice: float64Ptr(16.6),
		CostPriceSource:    SheinCostPriceSourceAuto,
		Currency:           "USD",
		IsActive:           true,
	})

	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
			}, 1),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{
		autoCosts: map[string]resolvedSheinCost{},
	}

	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 5, 55, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 5, StoreID: 55, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].AutoCostPrice)
	require.Nil(t, rows[0].EffectiveCostPrice)
	require.Equal(t, "USD", rows[0].Currency)
	require.Equal(t, SheinCostPriceSourceNone, rows[0].CostPriceSource)
}

func TestSyncSheinOnShelfProductsMarksMissingSKCsInactive(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1"},
						{SkcName: "skc-2"},
					},
				},
			}, 2),
		},
	}
	costResolver := &sheinSyncServiceCostResolverStub{}
	service := NewSheinSyncService(repo, productAPI, costResolver)

	_, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.NoError(t, err)

	productAPI.listResponses = []*sheinproduct.ProductListResponse{
		makeProductListResponse([]sheinproduct.ProductListItem{
			{
				SpuName:          "spu-1",
				ProductNameMulti: "Product One",
				ShelfStatus:      "ON_SHELF",
				SkcInfoList: []sheinproduct.SkcInfoItem{
					{SkcName: "skc-1"},
				},
			},
		}, 1),
	}
	productAPI.listCalls = nil

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeSchedule)
	require.NoError(t, err)
	require.Equal(t, SheinSyncTriggerModeSchedule, job.TriggerMode)
	require.Equal(t, 1, job.DeactivatedCount)

	active := true
	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 7, StoreID: 88, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, rows, 2)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.Equal(t, "skc-2", rows[1].SKCName)
	require.True(t, rows[0].IsActive)
	require.False(t, rows[1].IsActive)
	require.Equal(t, "OFF_SHELF", rows[1].ShelfStatus)

	activeRows, activeTotal, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{TenantID: 7, StoreID: 88, IsActive: &active, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), activeTotal)
	require.Len(t, activeRows, 1)
	require.Equal(t, "skc-1", activeRows[0].SKCName)
}

func TestSyncSheinOnShelfProductsContinuesWhenCostPriceForbidden(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "g2606161328066702",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
			}, 1),
		},
		queryCostPriceErr: errors.New("API错误 [403]: Forbidden"),
	}
	costResolver := &sheinProductCostResolver{
		productAPI:  productAPI,
		retryDelays: []time.Duration{0, 0},
		sleep:       func(context.Context, time.Duration) error { return nil },
	}
	service := NewSheinSyncService(repo, productAPI, costResolver)

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 7, 88, SheinSyncTriggerModeManual)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, SheinSyncJobStatusSucceeded, job.Status)
	require.Equal(t, 1, job.FetchedCount)

	rows, total, err := repo.ListSyncedProducts(context.Background(), &SheinSyncedProductQuery{
		TenantID: 7,
		StoreID:  88,
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	require.Equal(t, "skc-1", rows[0].SKCName)
	require.Nil(t, rows[0].AutoCostPrice)
	require.Nil(t, rows[0].EffectiveCostPrice)
}

func TestSyncSheinOnShelfProductsMarksJobFailedWhenListProductsFails(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listErr: errors.New("shein unavailable"),
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 3, 33, SheinSyncTriggerModeManual)
	require.Error(t, err)
	require.Nil(t, job)

	jobs, total, err := repo.ListSyncJobs(context.Background(), &SheinSyncJobQuery{TenantID: 3, StoreID: 33, Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, jobs, 1)
	require.Equal(t, SheinSyncJobStatusFailed, jobs[0].Status)
	require.Contains(t, jobs[0].ErrorSummary, "shein unavailable")
	require.NotNil(t, jobs[0].FinishedAt)
}

func TestSyncSheinOnShelfProductsReturnsClearErrorWhenProductAPIMissing(t *testing.T) {
	t.Parallel()

	service := NewSheinSyncService(newSheinSyncServiceRepoStub(), nil, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 6, 66, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "product API is required")
}

func TestSyncSheinOnShelfProductsReturnsClearErrorWhenListProductsResponseIsNil(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{nil},
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 8, 88, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "returned nil response")
}

func TestSyncSheinOnShelfProductsReturnsPersistenceErrorWhenFailedJobStateCannotBeSaved(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.saveFailedJobErr = errors.New("save failed job state")
	productAPI := &sheinSyncServiceProductAPIStub{
		listErr: errors.New("shein unavailable"),
	}
	service := NewSheinSyncService(repo, productAPI, &sheinSyncServiceCostResolverStub{})

	job, err := service.SyncSheinOnShelfProducts(context.Background(), 4, 44, SheinSyncTriggerModeManual)
	require.Nil(t, job)
	require.Error(t, err)
	require.ErrorContains(t, err, "save failed job state")
}

func TestSheinCostResolverReturnsClearErrorWhenQueryCostPriceResponseIsNil(t *testing.T) {
	t.Parallel()

	resolver := NewSheinCostResolver(&sheinSyncServiceProductAPIStub{})

	_, err := resolver.ResolveAutoCosts(context.Background(), sheinproduct.ProductListItem{
		SpuName: "spu-1",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1"},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "returned nil response")
}

func TestSyncSheinOnShelfProductsResolvesCostsSeriallyWithinPage(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	productAPI := &sheinSyncServiceProductAPIStub{
		listResponses: []*sheinproduct.ProductListResponse{
			makeProductListResponse([]sheinproduct.ProductListItem{
				{
					SpuName:          "spu-1",
					ProductNameMulti: "Product One",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-1", SupplierCode: "SUP-1"},
					},
				},
				{
					SpuName:          "spu-2",
					ProductNameMulti: "Product Two",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-2", SupplierCode: "SUP-2"},
					},
				},
				{
					SpuName:          "spu-3",
					ProductNameMulti: "Product Three",
					ShelfStatus:      "ON_SHELF",
					SkcInfoList: []sheinproduct.SkcInfoItem{
						{SkcName: "skc-3", SupplierCode: "SUP-3"},
					},
				},
			}, 3),
		},
	}
	costResolver := newBlockingConcurrentCostResolver(3)
	service := NewSheinSyncService(repo, productAPI, costResolver)

	done := make(chan error, 1)
	go func() {
		_, err := service.SyncSheinOnShelfProducts(context.Background(), 12, 34, SheinSyncTriggerModeManual)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return costResolver.startedCount() == 1
	}, time.Second, 10*time.Millisecond)
	require.Never(t, func() bool {
		return costResolver.startedCount() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	costResolver.releaseAll()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("sync did not finish after releasing cost resolver")
	}
}

func TestUpdateSDSCostGroupManualCostRefreshesRelatedCandidateCosts(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	repo.seedProduct(SheinSyncedProductRecord{
		ID:                 301,
		TenantID:           227,
		StoreID:            870,
		SKCName:            "sg260524164927164214023",
		SupplierCode:       "XB0608035002-AB885FBF",
		EffectiveCostPrice: float64Ptr(25.99),
		IsActive:           true,
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                   804,
		TenantID:             227,
		StoreID:              870,
		SyncedProductID:      301,
		ActivityType:         "PROMOTION",
		ActivityKey:          "PROMOTION:227:870",
		SKCName:              "sg260524164927164214023",
		CandidateVersion:     "v1",
		EffectiveCostPrice:   float64Ptr(47.52),
		PriceSnapshot:        `{"currency":"USD","sale_price":29.9}`,
		CalculatedProfitRate: float64Ptr(-0.5892976588628764),
		EligibilityStatus:    SheinCandidateEligibilityStatusEligible,
		ReviewStatus:         SheinCandidateReviewStatusPendingReview,
	})
	service := NewSheinSyncService(repo, &sheinSyncServiceProductAPIStub{}, &sheinSyncServiceCostResolverStub{})

	group, err := service.(interface {
		UpdateSDSCostGroupManualCost(context.Context, int64, int64, string, string, *float64) (*SheinSDSCostGroupRecord, error)
	}).UpdateSDSCostGroupManualCost(context.Background(), 227, 870, "source:XB0608035002", "XB0608035002", float64Ptr(21.99))

	require.NoError(t, err)
	require.NotNil(t, group)
	require.NotNil(t, group.ManualCostPrice)
	require.Equal(t, 21.99, *group.ManualCostPrice)

	candidates, _, err := repo.ListCandidates(context.Background(), &SheinActivityCandidateQuery{
		TenantID: 227,
		StoreID:  870,
		SKCName:  "sg260524164927164214023",
	})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].EffectiveCostPrice)
	require.Equal(t, 21.99, *candidates[0].EffectiveCostPrice)
	require.NotNil(t, candidates[0].CalculatedProfitRate)
	require.InEpsilon(t, 0.264548494983278, *candidates[0].CalculatedProfitRate, 0.0000000001)
	require.Equal(t, SheinCandidateReviewStatusPendingReview, candidates[0].ReviewStatus)
}

func TestUpdateSDSCostGroupManualCostRefreshesOnlyCurrentExecutableCandidateCosts(t *testing.T) {
	t.Parallel()

	repo := newSheinSyncServiceRepoStub()
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	repo.seedProduct(SheinSyncedProductRecord{
		ID:                 301,
		TenantID:           227,
		StoreID:            870,
		SKCName:            "sg260524164927164214023",
		SupplierCode:       "XB0608035002-AB885FBF",
		EffectiveCostPrice: float64Ptr(25.99),
		IsActive:           true,
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 803,
		TenantID:           227,
		StoreID:            870,
		SyncedProductID:    301,
		ActivityType:       "PROMOTION",
		ActivityKey:        "PROMOTION:227:870",
		SKCName:            "sg260524164927164214023",
		CandidateVersion:   "v-old",
		EffectiveCostPrice: float64Ptr(47.52),
		PriceSnapshot:      `{"currency":"USD","sale_price":29.9}`,
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusApproved,
		AutoModeEligible:   true,
		SelectedForRun:     true,
		CreatedAt:          now.Add(-2 * time.Hour),
		UpdatedAt:          now.Add(-2 * time.Hour),
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 804,
		TenantID:           227,
		StoreID:            870,
		SyncedProductID:    301,
		ActivityType:       "PROMOTION",
		ActivityKey:        "PROMOTION:227:870",
		SKCName:            "sg260524164927164214023",
		CandidateVersion:   "v-current",
		EffectiveCostPrice: float64Ptr(47.52),
		PriceSnapshot:      `{"currency":"USD","sale_price":29.9}`,
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusPendingReview,
		CreatedAt:          now.Add(-time.Hour),
		UpdatedAt:          now.Add(-time.Hour),
	})
	repo.seedCandidate(SheinActivityCandidateRecord{
		ID:                 805,
		TenantID:           227,
		StoreID:            870,
		SyncedProductID:    301,
		ActivityType:       "PROMOTION",
		ActivityKey:        "PROMOTION:227:870",
		SKCName:            "sg260524164927164214023",
		CandidateVersion:   "v-rejected",
		EffectiveCostPrice: float64Ptr(47.52),
		PriceSnapshot:      `{"currency":"USD","sale_price":29.9}`,
		EligibilityStatus:  SheinCandidateEligibilityStatusEligible,
		ReviewStatus:       SheinCandidateReviewStatusRejected,
		CreatedAt:          now,
		UpdatedAt:          now,
	})
	service := NewSheinSyncService(repo, &sheinSyncServiceProductAPIStub{}, &sheinSyncServiceCostResolverStub{})

	group, err := service.(interface {
		UpdateSDSCostGroupManualCost(context.Context, int64, int64, string, string, *float64) (*SheinSDSCostGroupRecord, error)
	}).UpdateSDSCostGroupManualCost(context.Background(), 227, 870, "source:XB0608035002", "XB0608035002", float64Ptr(21.99))

	require.NoError(t, err)
	require.NotNil(t, group)

	candidates, _, err := repo.ListCandidates(context.Background(), &SheinActivityCandidateQuery{
		TenantID: 227,
		StoreID:  870,
		SKCName:  "sg260524164927164214023",
	})
	require.NoError(t, err)
	require.Len(t, candidates, 3)

	byVersion := make(map[string]SheinActivityCandidateRecord, len(candidates))
	for _, candidate := range candidates {
		byVersion[candidate.CandidateVersion] = candidate
	}
	require.NotNil(t, byVersion["v-current"].EffectiveCostPrice)
	require.Equal(t, 21.99, *byVersion["v-current"].EffectiveCostPrice)
	require.NotNil(t, byVersion["v-current"].CalculatedProfitRate)
	require.InEpsilon(t, 0.264548494983278, *byVersion["v-current"].CalculatedProfitRate, 0.0000000001)

	require.NotNil(t, byVersion["v-old"].EffectiveCostPrice)
	require.Equal(t, 47.52, *byVersion["v-old"].EffectiveCostPrice)
	require.Equal(t, SheinCandidateReviewStatusApproved, byVersion["v-old"].ReviewStatus)
	require.True(t, byVersion["v-old"].AutoModeEligible)
	require.True(t, byVersion["v-old"].SelectedForRun)

	require.NotNil(t, byVersion["v-rejected"].EffectiveCostPrice)
	require.Equal(t, 47.52, *byVersion["v-rejected"].EffectiveCostPrice)
	require.Equal(t, SheinCandidateReviewStatusRejected, byVersion["v-rejected"].ReviewStatus)
}

type sheinSyncServiceRepoStub struct {
	mu            sync.RWMutex
	nextID        int64
	nextCandidate int64
	nextJob       int64
	products      map[string]SheinSyncedProductRecord
	candidates    map[int64]SheinActivityCandidateRecord
	sdsCostGroups map[string]SheinSDSCostGroupRecord
	jobs          map[int64]SheinSyncJobRecord

	saveFailedJobErr error
}

type blockingConcurrentCostResolver struct {
	mu        sync.Mutex
	started   int
	release   chan struct{}
	autoCosts map[string]resolvedSheinCost
}

func newBlockingConcurrentCostResolver(expected int) *blockingConcurrentCostResolver {
	autoCosts := make(map[string]resolvedSheinCost, expected)
	for i := 1; i <= expected; i++ {
		autoCosts[fmt.Sprintf("spu-%d|skc-%d", i, i)] = resolvedSheinCost{
			CostPrice: float64Ptr(float64(10 + i)),
			Currency:  "USD",
		}
	}
	return &blockingConcurrentCostResolver{
		release:   make(chan struct{}),
		autoCosts: autoCosts,
	}
}

func (r *blockingConcurrentCostResolver) ResolveAutoCosts(_ context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
	r.mu.Lock()
	r.started++
	r.mu.Unlock()

	<-r.release

	resolved := map[string]resolvedSheinCost{}
	for _, skc := range product.SkcInfoList {
		if cost, ok := r.autoCosts[product.SpuName+"|"+skc.SkcName]; ok {
			resolved[skc.SkcName] = cost
		}
	}
	return resolved, nil
}

func (r *blockingConcurrentCostResolver) startedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started
}

func (r *blockingConcurrentCostResolver) releaseAll() {
	close(r.release)
}

func newSheinSyncServiceRepoStub() *sheinSyncServiceRepoStub {
	return &sheinSyncServiceRepoStub{
		nextID:        1,
		nextCandidate: 1,
		nextJob:       1,
		products:      make(map[string]SheinSyncedProductRecord),
		candidates:    make(map[int64]SheinActivityCandidateRecord),
		sdsCostGroups: make(map[string]SheinSDSCostGroupRecord),
		jobs:          make(map[int64]SheinSyncJobRecord),
	}
}

func (r *sheinSyncServiceRepoStub) seedProduct(record SheinSyncedProductRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record.ID == 0 {
		record.ID = r.nextID
		r.nextID++
	}
	r.products[r.productKey(record.TenantID, record.StoreID, record.SKCName)] = cloneServiceTestProduct(record)
}

func (r *sheinSyncServiceRepoStub) seedCandidate(record SheinActivityCandidateRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record.ID == 0 {
		record.ID = r.nextCandidate
		r.nextCandidate++
	}
	r.candidates[record.ID] = cloneServiceTestCandidate(record)
}

func (r *sheinSyncServiceRepoStub) UpsertSyncedProducts(_ context.Context, records []*SheinSyncedProductRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, record := range records {
		if record == nil {
			continue
		}
		key := r.productKey(record.TenantID, record.StoreID, record.SKCName)
		row := cloneServiceTestProduct(*record)
		if existing, ok := r.products[key]; ok {
			row.ID = existing.ID
		} else {
			row.ID = r.nextID
			r.nextID++
		}
		ApplyEffectiveCostPrice(&row)
		r.products[key] = row
	}
	return nil
}

func (r *sheinSyncServiceRepoStub) ListSyncedProducts(_ context.Context, query *SheinSyncedProductQuery) ([]SheinSyncedProductRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinSyncedProductRecord, 0, len(r.products))
	for _, row := range r.products {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.SKCName != "" && row.SKCName != query.SKCName {
				continue
			}
			if query.IsActive != nil && row.IsActive != *query.IsActive {
				continue
			}
		}
		items = append(items, cloneServiceTestProduct(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].SKCName < items[j].SKCName
	})

	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []SheinSyncedProductRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *sheinSyncServiceRepoStub) UpdateManualCostPrice(_ context.Context, productID int64, manualCostPrice *float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, row := range r.products {
		if row.ID != productID {
			continue
		}
		row.ManualCostPrice = cloneServiceTestFloat64(manualCostPrice)
		ApplyEffectiveCostPrice(&row)
		r.products[key] = row
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (r *sheinSyncServiceRepoStub) UpdateSyncedProductInventoryAttributes(_ context.Context, tenantID, storeID int64, skcName string, attributes string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.productKey(tenantID, storeID, skcName)
	row, ok := r.products[key]
	if !ok {
		return 0, nil
	}
	row.InventorySyncAttributes = attributes
	r.products[key] = row
	return 1, nil
}

func (r *sheinSyncServiceRepoStub) UpsertSDSCostGroup(_ context.Context, record *SheinSDSCostGroupRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if record == nil {
		return nil
	}
	row := cloneServiceTestSDSCostGroup(*record)
	key := r.sdsCostGroupKey(row.TenantID, row.StoreID, row.GroupKey)
	r.sdsCostGroups[key] = row
	return nil
}

func (r *sheinSyncServiceRepoStub) ListSDSCostGroups(_ context.Context, query *SheinSDSCostGroupQuery) ([]SheinSDSCostGroupRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinSDSCostGroupRecord, 0, len(r.sdsCostGroups))
	for _, row := range r.sdsCostGroups {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if len(query.GroupKeys) > 0 && !containsServiceTestString(query.GroupKeys, row.GroupKey) {
				continue
			}
		}
		items = append(items, cloneServiceTestSDSCostGroup(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].GroupKey < items[j].GroupKey
	})
	return items, int64(len(items)), nil
}

func (r *sheinSyncServiceRepoStub) MarkMissingSyncedProductsInactive(_ context.Context, tenantID, storeID int64, activeSKCNames []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	activeSet := make(map[string]struct{}, len(activeSKCNames))
	for _, skcName := range activeSKCNames {
		activeSet[skcName] = struct{}{}
	}
	for key, row := range r.products {
		if row.TenantID != tenantID || row.StoreID != storeID {
			continue
		}
		if _, ok := activeSet[row.SKCName]; ok {
			continue
		}
		row.IsActive = false
		row.ShelfStatus = "OFF_SHELF"
		row.LastSyncAt = &now
		row.UpdatedAt = now
		r.products[key] = row
	}
	return nil
}

func (r *sheinSyncServiceRepoStub) SaveSyncJob(_ context.Context, job *SheinSyncJobRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job == nil {
		return nil
	}
	if job.Status == SheinSyncJobStatusFailed && r.saveFailedJobErr != nil {
		return r.saveFailedJobErr
	}
	row := *job
	if row.ID == 0 {
		row.ID = r.nextJob
		r.nextJob++
	}
	r.jobs[row.ID] = row
	*job = row
	return nil
}

func (r *sheinSyncServiceRepoStub) ListSyncJobs(_ context.Context, query *SheinSyncJobQuery) ([]SheinSyncJobRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinSyncJobRecord, 0, len(r.jobs))
	for _, row := range r.jobs {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.TriggerMode != nil && row.TriggerMode != *query.TriggerMode {
				continue
			}
			if query.Status != nil && row.Status != *query.Status {
				continue
			}
		}
		items = append(items, row)
	}
	return items, int64(len(items)), nil
}

func (r *sheinSyncServiceRepoStub) ListCandidates(_ context.Context, query *SheinActivityCandidateQuery) ([]SheinActivityCandidateRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if query.ActivityType != "" && row.ActivityType != query.ActivityType {
				continue
			}
			if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
				continue
			}
			if query.SKCName != "" && row.SKCName != query.SKCName {
				continue
			}
			if query.CandidateVersion != "" && row.CandidateVersion != query.CandidateVersion {
				continue
			}
			if len(query.CandidateIDs) > 0 && !containsServiceTestID(query.CandidateIDs, row.ID) {
				continue
			}
			if query.ExecutableOnly && !isServiceTestExecutableCandidate(row) {
				continue
			}
		}
		items = append(items, cloneServiceTestCandidate(row))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, int64(len(items)), nil
}

func isServiceTestExecutableCandidate(row SheinActivityCandidateRecord) bool {
	if row.EligibilityStatus != SheinCandidateEligibilityStatusEligible {
		return false
	}
	switch row.ReviewStatus {
	case SheinCandidateReviewStatusPendingReview, SheinCandidateReviewStatusApproved, SheinCandidateReviewStatusAutoQueued:
		return true
	default:
		return false
	}
}

func (r *sheinSyncServiceRepoStub) SaveCandidates(_ context.Context, records []*SheinActivityCandidateRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, record := range records {
		if record == nil {
			continue
		}
		row := cloneServiceTestCandidate(*record)
		if row.ID == 0 {
			row.ID = r.nextCandidate
			r.nextCandidate++
		}
		r.candidates[row.ID] = row
	}
	return nil
}

func (r *sheinSyncServiceRepoStub) CreateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) UpdateEnrollmentRun(_ context.Context, _ *SheinActivityEnrollmentRunRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) ListEnrollmentRuns(_ context.Context, _ *SheinEnrollmentRunQuery) ([]SheinActivityEnrollmentRunRecord, int64, error) {
	return nil, 0, nil
}

func (r *sheinSyncServiceRepoStub) SaveEnrollmentItems(_ context.Context, _ []*SheinActivityEnrollmentItemRecord) error {
	return nil
}

func (r *sheinSyncServiceRepoStub) ListEnrollmentItems(_ context.Context, _ *SheinEnrollmentItemQuery) ([]SheinActivityEnrollmentItemRecord, int64, error) {
	return nil, 0, nil
}

func (r *sheinSyncServiceRepoStub) productKey(tenantID, storeID int64, skcName string) string {
	return fmt.Sprintf("%d|%d|%s", tenantID, storeID, skcName)
}

func (r *sheinSyncServiceRepoStub) sdsCostGroupKey(tenantID, storeID int64, groupKey string) string {
	return fmt.Sprintf("%d|%d|%s", tenantID, storeID, groupKey)
}

type sheinSyncServiceProductAPIStub struct {
	listResponses      []*sheinproduct.ProductListResponse
	listErr            error
	listCalls          []sheinProductListCall
	queryPriceResp     *sheinproduct.PriceQueryResponse
	queryPriceErr      error
	queryInventoryResp *sheinproduct.InventoryQueryResponse
	queryInventoryErr  error
	queryCostPriceResp *sheinproduct.CostPriceQueryResponse
	queryCostPriceErr  error
}

type sheinProductListCall struct {
	pageNum  int
	pageSize int
	request  *sheinproduct.ProductListRequest
}

func (s *sheinSyncServiceProductAPIStub) GetProduct(string) (*sheinproduct.Product, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) UpdateProduct(*sheinproduct.Product) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) DeleteProduct(string) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) GetPartInfo(int) (*sheinproduct.PartInfoResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) SaveDraftProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) PublishProduct(*sheinproduct.Product) (*sheinproduct.SheinResponse, string, error) {
	return nil, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) ConfirmPublish(*sheinproduct.Product) (bool, string, error) {
	return false, "", errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) Record(*sheinproduct.ProductRecordRequest) (*sheinproduct.RecordResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) ListProducts(pageNum, pageSize int, request *sheinproduct.ProductListRequest) (*sheinproduct.ProductListResponse, error) {
	s.listCalls = append(s.listCalls, sheinProductListCall{
		pageNum:  pageNum,
		pageSize: pageSize,
		request:  cloneProductListRequest(request),
	})
	if s.listErr != nil {
		return nil, s.listErr
	}
	if len(s.listResponses) == 0 {
		return makeProductListResponse(nil, 0), nil
	}
	index := pageNum - 1
	if index < 0 || index >= len(s.listResponses) {
		return makeProductListResponse(nil, 0), nil
	}
	return s.listResponses[index], nil
}

func (s *sheinSyncServiceProductAPIStub) QueryBrandList() (*sheinproduct.BrandListResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryStock(*sheinproduct.StockQueryRequest) (*sheinproduct.StockQueryResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryInventory(string) (*sheinproduct.InventoryQueryResponse, error) {
	return s.queryInventoryResp, s.queryInventoryErr
}

func (s *sheinSyncServiceProductAPIStub) UpdateInventory(*sheinproduct.InventoryUpdateRequest) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) QueryPrice(string) (*sheinproduct.PriceQueryResponse, error) {
	return s.queryPriceResp, s.queryPriceErr
}

func (s *sheinSyncServiceProductAPIStub) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	return s.queryCostPriceResp, s.queryCostPriceErr
}

func (s *sheinSyncServiceProductAPIStub) OffShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

func (s *sheinSyncServiceProductAPIStub) OnShelf(*sheinproduct.ShelfOperateRequest) error {
	return errors.New("not implemented")
}

type sheinSyncServiceCostResolverStub struct {
	autoCosts map[string]resolvedSheinCost
	err       error
}

func (s *sheinSyncServiceCostResolverStub) ResolveAutoCosts(_ context.Context, product sheinproduct.ProductListItem) (map[string]resolvedSheinCost, error) {
	if s.err != nil {
		return nil, s.err
	}
	resolved := make(map[string]resolvedSheinCost)
	for _, skc := range product.SkcInfoList {
		key := product.SpuName + "|" + skc.SkcName
		if cost, ok := s.autoCosts[key]; ok {
			resolved[skc.SkcName] = cost
		}
	}
	return resolved, nil
}

type sheinInventoryMappingSourceStub struct {
	expectedPlatform    string
	byPlatformProductID map[string]listingadmin.ProductImportMapping
}

func (s sheinInventoryMappingSourceStub) FindLatest(_ context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error) {
	if s.expectedPlatform != "" && query.Platform != s.expectedPlatform {
		return nil, nil
	}
	if s.byPlatformProductID == nil {
		return nil, nil
	}
	row, ok := s.byPlatformProductID[query.PlatformProductID]
	if !ok {
		return nil, nil
	}
	return &row, nil
}

func makeProductListResponse(items []sheinproduct.ProductListItem, total int) *sheinproduct.ProductListResponse {
	resp := &sheinproduct.ProductListResponse{Code: "0", Msg: "ok"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = total
	return resp
}

func makePriceQueryResponse(items []sheinproduct.SkcPriceData) *sheinproduct.PriceQueryResponse {
	resp := &sheinproduct.PriceQueryResponse{Code: "0", Msg: "ok"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = len(items)
	return resp
}

func makeInventoryQueryResponse(items []sheinproduct.SkcInventory) *sheinproduct.InventoryQueryResponse {
	resp := &sheinproduct.InventoryQueryResponse{Code: "0", Msg: "ok"}
	resp.Info.SkcInfo = append(resp.Info.SkcInfo, items...)
	return resp
}

func cloneProductListRequest(request *sheinproduct.ProductListRequest) *sheinproduct.ProductListRequest {
	if request == nil {
		return nil
	}
	row := *request
	return &row
}

func cloneServiceTestProduct(row SheinSyncedProductRecord) SheinSyncedProductRecord {
	row.PublishTime = cloneServiceTestTime(row.PublishTime)
	row.FirstShelfTime = cloneServiceTestTime(row.FirstShelfTime)
	row.LastSyncAt = cloneServiceTestTime(row.LastSyncAt)
	row.SupplyPrice = cloneServiceTestFloat64(row.SupplyPrice)
	row.AutoCostPrice = cloneServiceTestFloat64(row.AutoCostPrice)
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	return row
}

func cloneServiceTestCandidate(row SheinActivityCandidateRecord) SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneServiceTestFloat64(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneServiceTestFloat64(row.CalculatedProfitRate)
	return row
}

func cloneServiceTestSDSCostGroup(row SheinSDSCostGroupRecord) SheinSDSCostGroupRecord {
	row.ManualCostPrice = cloneServiceTestFloat64(row.ManualCostPrice)
	return row
}

func cloneServiceTestFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func containsServiceTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsServiceTestID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneServiceTestTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}
